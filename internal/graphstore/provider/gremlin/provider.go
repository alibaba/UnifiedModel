// Package gremlin persists GraphStore records through TinkerPop Gremlin Server.
// Reads use the shared Go evaluator to preserve workspace, lifecycle, and
// controlled read-only Cypher semantics across providers.
package gremlin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/pkg/model"
	"github.com/gorilla/websocket"
)

type settings struct {
	uri, username, password, traversalSource string
	caFile                                   string
	timeout                                  time.Duration
}

func configuration(options map[string]string) (settings, error) {
	value := func(key, fallback string) string {
		if v, ok := options[key]; ok {
			return v
		}
		if v, ok := os.LookupEnv("GREMLIN_" + strings.ToUpper(key)); ok {
			return v
		}
		return fallback
	}
	c := settings{
		uri: value("uri", "ws://localhost:8182/gremlin"), username: value("username", ""),
		password: value("password", ""), traversalSource: value("traversal_source", "g"),
		caFile: value("ca_file", ""),
	}
	u, err := url.Parse(c.uri)
	if err != nil || u.Hostname() == "" || (u.Scheme != "ws" && u.Scheme != "wss") {
		return c, fmt.Errorf("gremlin: uri must be a valid ws:// or wss:// endpoint")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return c, fmt.Errorf("gremlin: URI must not contain credentials, query parameters, or a fragment; use username/password options")
	}
	if (c.username == "") != (c.password == "") {
		return c, fmt.Errorf("gremlin: username and password must be configured together")
	}
	if c.caFile != "" && u.Scheme != "wss" {
		return c, fmt.Errorf("gremlin: ca_file requires a wss:// endpoint")
	}
	if strings.TrimSpace(c.traversalSource) == "" {
		return c, fmt.Errorf("gremlin: traversal_source is required")
	}
	c.timeout, err = time.ParseDuration(value("timeout", "10s"))
	if err != nil || c.timeout < time.Millisecond || c.timeout > 24*time.Hour {
		return c, fmt.Errorf("gremlin: timeout must be between 1ms and 24h")
	}
	return c, nil
}

// Provider serializes operations within one instance. Run one writing instance
// per graph: Gremlin does not offer portable uniqueness constraints or locks.
type Provider struct {
	client   *websocket.Conn
	settings settings
	timeout  time.Duration
	gate     chan struct{}
	closed   bool // protected by gate
}

var _ contract.GraphStore = (*Provider)(nil)

func init() {
	graphstore.RegisterProvider(graphstore.ProviderTypeGremlin, func(config graphstore.ProviderConfig) (contract.GraphStore, error) {
		return NewProvider(config)
	})
}

func NewProvider(config graphstore.ProviderConfig) (*Provider, error) {
	c, err := configuration(config.Options)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if c.caFile != "" {
		pem, err := os.ReadFile(c.caFile)
		if err != nil {
			return nil, fmt.Errorf("gremlin: cannot read CA certificate file")
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("gremlin: CA file contains no valid PEM certificates")
		}
		tlsConfig.RootCAs = roots
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	// Gremlin Server expects one complete request per WebSocket frame. Keep
	// every permitted packet in one frame rather than sending continuations.
	dialer := websocket.Dialer{TLSClientConfig: tlsConfig, HandshakeTimeout: c.timeout, ReadBufferSize: 32 << 10, WriteBufferSize: 1 << 20}
	client, response, err := dialer.DialContext(ctx, c.uri, nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		// Driver errors can contain the endpoint or authentication response.
		return nil, fmt.Errorf("gremlin: connection failed; check GREMLIN_URI and authentication")
	}
	client.SetReadLimit(8 << 20)
	p := &Provider{client: client, settings: c, timeout: c.timeout, gate: make(chan struct{}, 1)}
	if _, err := p.Health(context.Background()); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("gremlin: initialization failed; check traversal source, GraphSON 3, and gremlin-groovy support: %w", err)
	}
	return p, nil
}

// lock bounds the entire operation, including waiting for other operations.
func (p *Provider) lock(ctx context.Context) (context.Context, func(), error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	select {
	case p.gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-p.gate
			cancel()
			return nil, nil, err
		}
		if p.closed {
			<-p.gate
			cancel()
			return nil, nil, fmt.Errorf("gremlin: provider is closed; reopen it before retrying")
		}
		return ctx, func() { <-p.gate; cancel() }, nil
	case <-ctx.Done():
		cancel()
		return nil, nil, ctx.Err()
	}
}

func (p *Provider) closeLocked() {
	if !p.closed {
		p.closed = true
		_ = p.client.Close()
	}
}

func (p *Provider) Close() error {
	p.gate <- struct{}{}
	defer func() { <-p.gate }()
	p.closeLocked()
	return nil
}

const healthScript = `g.V().limit(0).count().next()`

const workspaceScript = `
def markers = g.V().hasLabel('UModelWorkspace').has('workspace', workspace).limit(2).toList()
if (markers.size() > 1) throw new IllegalStateException('duplicate UModel workspace markers')
if (markers.isEmpty()) g.addV('UModelWorkspace').property('workspace', workspace).iterate()
1`

func (p *Provider) OpenWorkspace(ctx context.Context, workspace model.WorkspaceMetadata) error {
	if workspace.ID == "" {
		return fmt.Errorf("workspace is required")
	}
	ctx, unlock, err := p.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	_, err = p.submit(ctx, workspaceScript, map[string]any{"workspace": workspace.ID})
	return err
}

func (p *Provider) EnsureSchema(ctx context.Context, workspace string) error {
	// Indexes and constraints are backend-specific and operator-managed.
	return p.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: workspace})
}

type collection int

const (
	models collection = 1 << iota
	entities
	relations
)

// JSON row envelopes avoid graph element serialization differences. Payload
// JSON is kept as a string and decoded with UseNumber for integer precision.
const snapshotScript = `
def records = []
def collect = { traversal, kind ->
    traversal.has('workspace', workspace).has('data').each { n ->
        def key = n.value('key')
        if (um_selected_keys == null || um_selected_keys.contains(key))
            records.add(groovy.json.JsonOutput.toJson([kind: kind, key: key, data: n.value('data')]))
    }
}
if (models) collect(g.V().hasLabel('UModelElement'), 'model')
if (entities) collect(g.V().hasLabel('UModelEntity'), 'entity')
if (relations) collect(g.E().hasLabel('UMODEL_RELATION'), 'relation')
records`

func (p *Provider) loadSnapshot(ctx context.Context, workspace string, c collection, keys []string) (graphstore.WorkspaceSnapshot, error) {
	// An untyped nil encodes as GraphBinary null, not a typed nil list.
	var selected any
	if keys != nil {
		selected = keys
	}
	rows, err := p.submit(ctx, snapshotScript, map[string]any{
		"workspace": workspace, "um_selected_keys": selected,
		"models": c&models != 0, "entities": c&entities != 0, "relations": c&relations != 0,
	})
	if err != nil {
		return graphstore.WorkspaceSnapshot{}, err
	}
	return decodeSnapshot(rows)
}

func decodeSnapshot(rows []*result) (graphstore.WorkspaceSnapshot, error) {
	s := graphstore.WorkspaceSnapshot{
		UModels: make(map[string]model.UModelElement), Entities: make(map[string]model.EntityPayload),
		Relations: make(map[string]model.RelationPayload),
	}
	seen := map[string]bool{}
	for _, row := range rows {
		var record struct{ Kind, Key, Data string }
		data, ok := row.Data.(string)
		if !ok || json.Unmarshal([]byte(data), &record) != nil || record.Key == "" {
			return s, fmt.Errorf("gremlin: invalid stored record envelope")
		}
		id := record.Kind + ":" + record.Key
		if seen[id] {
			return s, fmt.Errorf("gremlin: duplicate graph record %q; repair data and use one writing instance", record.Key)
		}
		seen[id] = true
		decode := func(target any) error {
			d := json.NewDecoder(strings.NewReader(record.Data))
			d.UseNumber()
			if err := d.Decode(target); err != nil {
				return fmt.Errorf("gremlin: decode record %q: %w", record.Key, err)
			}
			return nil
		}
		var err error
		switch record.Kind {
		case "model":
			var v model.UModelElement
			err = decode(&v)
			s.UModels[record.Key] = v
		case "entity":
			var v model.EntityPayload
			err = decode(&v)
			s.Entities[record.Key] = v
		case "relation":
			var v model.RelationPayload
			err = decode(&v)
			s.Relations[record.Key] = v
		default:
			return s, fmt.Errorf("gremlin: invalid stored record kind")
		}
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

// A complete batch is encoded before submitting this script. Values are always
// bindings, never interpolated into Groovy. Existing vertices use single
// cardinality to avoid accumulating stale payloads on TinkerGraph/JanusGraph.
const saveScript = `
def findVertex = { label, key ->
    def found = g.V().hasLabel(label).has('workspace', workspace).has('key', key).limit(2).toList()
    if (found.size() > 1) throw new IllegalStateException('duplicate UModel vertex')
    found.isEmpty() ? null : found[0]
}
def vertex = { label, key ->
    def v = findVertex(label, key)
    if (v == null) v = g.addV(label).property('workspace', workspace).property('key', key).next()
    v
}
// Preflight duplicate keys and endpoints before any mutations.
rows.each { row ->
    if (kind == 'relation') {
        findVertex('UModelEntity', row.src)
        findVertex('UModelEntity', row.dest)
        if (g.E().hasLabel('UMODEL_RELATION').has('workspace', workspace).has('key', row.key).limit(2).count().next() > 1)
            throw new IllegalStateException('duplicate UModel relation')
    } else findVertex(kind == 'model' ? 'UModelElement' : 'UModelEntity', row.key)
}
deleted.each { key -> g.V().hasLabel('UModelElement').has('workspace', workspace).has('key', key).drop().iterate() }
rows.each { row ->
    if (kind == 'relation') {
        def src = vertex('UModelEntity', row.src)
        def dest = vertex('UModelEntity', row.dest)
        def found = g.E().hasLabel('UMODEL_RELATION').has('workspace', workspace).has('key', row.key).toList()
        def edge = found.isEmpty() ? g.V(src).addE('UMODEL_RELATION').to(dest).property('workspace', workspace).property('key', row.key).next() : found[0]
        edge.property('data', row.data)
    } else {
        def v = vertex(kind == 'model' ? 'UModelElement' : 'UModelEntity', row.key)
        g.V(v).property(org.apache.tinkerpop.gremlin.structure.VertexProperty.Cardinality.single, 'data', row.data).iterate()
    }
}
1`

func changes(c collection, s graphstore.WorkspaceSnapshot, result model.WriteResult) ([]map[string]any, []string, error) {
	rows := make([]map[string]any, 0, result.Accepted)
	deleted := make([]string, 0)
	seen := map[string]bool{}
	for _, item := range result.Items {
		if !item.OK || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		row := map[string]any{"key": item.ID}
		var payload any
		switch c {
		case models:
			v, exists := s.UModels[item.ID]
			if !exists {
				deleted = append(deleted, item.ID)
				continue
			}
			payload = v
		case entities:
			payload = s.Entities[item.ID]
		case relations:
			v := s.Relations[item.ID]
			payload = v
			row["src"], row["dest"] = endpointKey(v, "src"), endpointKey(v, "dest")
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("gremlin: encode record %q: %w", item.ID, err)
		}
		row["data"] = string(data)
		rows = append(rows, row)
	}
	return rows, deleted, nil
}

func endpointKey(r model.RelationPayload, side string) string {
	return graphstore.EntityKey(model.EntityPayload{
		"__domain__": r["__"+side+"_domain__"], "__entity_type__": r["__"+side+"_entity_type__"],
		"__entity_id__": r["__"+side+"_entity_id__"],
	})
}

func (p *Provider) mutate(ctx context.Context, workspace string, c collection, keys []string, apply func(context.Context, *graphstore.MemoryStore) (model.WriteResult, error)) (model.WriteResult, error) {
	if workspace == "" {
		return model.WriteResult{}, fmt.Errorf("workspace is required")
	}
	ctx, unlock, err := p.lock(ctx)
	if err != nil {
		return model.WriteResult{}, err
	}
	defer unlock()
	snapshot, err := p.loadSnapshot(ctx, workspace, c, keys)
	if err != nil {
		return model.WriteResult{}, err
	}
	memory := graphstore.NewMemoryStoreFromSnapshot(workspace, snapshot)
	result, err := apply(ctx, memory)
	if err != nil {
		return model.WriteResult{}, err
	}
	if result.Accepted == 0 {
		return result, nil
	}
	rows, deleted, err := changes(c, memory.SnapshotWorkspace(workspace), result)
	if err != nil {
		return model.WriteResult{}, err
	}
	kind := map[collection]string{models: "model", entities: "entity", relations: "relation"}[c]
	_, err = p.submit(ctx, saveScript, map[string]any{"workspace": workspace, "kind": kind, "rows": rows, "deleted": deleted})
	if err != nil {
		return model.WriteResult{}, err
	}
	return result, nil
}

func (p *Provider) PutUModelElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error) {
	keys := make([]string, 0, len(batch.Elements))
	for _, e := range batch.Elements {
		keys = append(keys, model.UModelElementKey(e))
	}
	return p.mutate(ctx, batch.Workspace, models, keys, func(ctx context.Context, s *graphstore.MemoryStore) (model.WriteResult, error) {
		return s.PutUModelElements(ctx, batch)
	})
}

func (p *Provider) DeleteUModelElements(ctx context.Context, workspace string, ids []string) (model.WriteResult, error) {
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, strings.TrimSpace(id))
	}
	return p.mutate(ctx, workspace, models, keys, func(ctx context.Context, s *graphstore.MemoryStore) (model.WriteResult, error) {
		return s.DeleteUModelElements(ctx, workspace, ids)
	})
}

func (p *Provider) WriteEntities(ctx context.Context, batch model.EntityWriteBatch) (model.WriteResult, error) {
	keys := make([]string, 0, len(batch.Entities))
	for _, e := range batch.Entities {
		keys = append(keys, graphstore.EntityKey(e))
	}
	return p.mutate(ctx, batch.Workspace, entities, keys, func(ctx context.Context, s *graphstore.MemoryStore) (model.WriteResult, error) {
		return s.WriteEntities(ctx, batch)
	})
}

func (p *Provider) WriteRelations(ctx context.Context, batch model.RelationWriteBatch) (model.WriteResult, error) {
	keys := make([]string, 0, len(batch.Relations))
	for _, r := range batch.Relations {
		keys = append(keys, graphstore.RelationKey(r))
	}
	return p.mutate(ctx, batch.Workspace, relations, keys, func(ctx context.Context, s *graphstore.MemoryStore) (model.WriteResult, error) {
		return s.WriteRelations(ctx, batch)
	})
}

func (p *Provider) read(ctx context.Context, workspace string, c collection) (*graphstore.MemoryStore, error) {
	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	ctx, unlock, err := p.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	snapshot, err := p.loadSnapshot(ctx, workspace, c, nil)
	if err != nil {
		return nil, err
	}
	return graphstore.NewMemoryStoreFromSnapshot(workspace, snapshot), nil
}

func (p *Provider) GetUModelSnapshot(ctx context.Context, req model.UModelSnapshotRequest) (model.UModelSnapshot, error) {
	s, err := p.read(ctx, req.Workspace, models)
	if err != nil {
		return model.UModelSnapshot{}, err
	}
	if req.Version == "" {
		req.Version = graphstore.ProviderTypeGremlin
	}
	return s.GetUModelSnapshot(ctx, req)
}

func (p *Provider) QueryEntities(ctx context.Context, plan model.EntityQueryPlan) (model.QueryResult, error) {
	s, err := p.read(ctx, plan.Workspace, entities)
	if err != nil {
		return model.QueryResult{}, err
	}
	return s.QueryEntities(ctx, plan)
}

func (p *Provider) QueryTopo(ctx context.Context, plan model.TopoQueryPlan) (model.QueryResult, error) {
	c := relations
	if plan.GraphCall != nil && plan.GraphCall.Name == "cypher" {
		c |= entities
	}
	s, err := p.read(ctx, plan.Workspace, c)
	if err != nil {
		return model.QueryResult{}, err
	}
	return s.QueryTopo(ctx, plan)
}

func (p *Provider) Capabilities(ctx context.Context) (model.GraphStoreCapabilities, error) {
	c, err := graphstore.NewMemoryStore().Capabilities(ctx)
	c.Timeout = p.timeout.String()
	return c, err
}

func (p *Provider) Health(ctx context.Context) (model.GraphStoreHealth, error) {
	health := model.GraphStoreHealth{
		Provider: graphstore.ProviderTypeGremlin, Status: "ok",
		Message: "use one writing provider instance per graph; schema is operator-managed",
	}
	ctx, unlock, err := p.lock(ctx)
	if err == nil {
		defer unlock()
		_, err = p.submit(ctx, healthScript, nil)
	}
	if err != nil {
		health.Status, health.Message = "unavailable", "Gremlin Server is unavailable"
	}
	return health, err
}
