//go:build ladybug

package ladybug

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	lbug "github.com/LadybugDB/go-ladybug"
	"github.com/alibaba/UnifiedModel/internal/cypher"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type Provider struct {
	root       string
	mu         sync.Mutex
	workspaces map[string]*workspaceHandle
}

type workspaceHandle struct {
	db   *lbug.Database
	conn *lbug.Connection
}

const (
	upsertUModelNodeStatement = `MERGE (n:umodel_node {key: $key}) SET n.kind = $kind, n.domain = $domain, n.name = $name, n.version = $version, n.spec = $spec;`
	deleteUModelNodeStatement = `MATCH (n:umodel_node {key: $key}) DELETE n;`
	upsertEntityStatement     = `MERGE (e:entity {entity_key: $entity_key}) SET e.domain = $domain, e.entity_type = $entity_type, e.entity_id = $entity_id, e.method = $method, e.first_observed_time = $first_observed_time, e.last_observed_time = $last_observed_time, e.keep_alive_seconds = $keep_alive_seconds, e.deleted = $deleted, e.properties = $properties;`
	upsertTopoStatement       = `MATCH (s:entity {entity_key: $src_key}), (d:entity {entity_key: $dest_key}) MERGE (s)-[r:topo {relation_key: $relation_key}]->(d) SET r.relation_type = $relation_type, r.method = $method, r.first_observed_time = $first_observed_time, r.last_observed_time = $last_observed_time, r.keep_alive_seconds = $keep_alive_seconds, r.deleted = $deleted, r.properties = $properties;`
)

func NewProvider(config graphstore.ProviderConfig) (*Provider, error) {
	root := config.DataRoot
	if root == "" {
		root = "data"
	}
	return &Provider{root: root, workspaces: make(map[string]*workspaceHandle)}, nil
}

func init() {
	graphstore.RegisterProvider(graphstore.ProviderTypeLadybug, func(config graphstore.ProviderConfig) (contract.GraphStore, error) {
		return NewProvider(config)
	})
}

func (p *Provider) OpenWorkspace(ctx context.Context, workspace model.WorkspaceMetadata) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.workspaces[workspace.ID]; ok {
		return nil
	}
	return p.openWorkspaceLocked(workspace)
}

func (p *Provider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for workspace, handle := range p.workspaces {
		if handle.conn != nil {
			handle.conn.Close()
		}
		if handle.db != nil {
			handle.db.Close()
		}
		delete(p.workspaces, workspace)
	}
}

func (p *Provider) EnsureSchema(ctx context.Context, workspace string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ensureSchemaLocked(workspace)
}

func (p *Provider) PutUModelElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error) {
	conn, err := p.conn(batch.Workspace)
	if err != nil {
		return model.WriteResult{}, err
	}
	stmt, err := conn.Prepare(upsertUModelNodeStatement)
	if err != nil {
		return model.WriteResult{}, err
	}
	defer stmt.Close()

	items := make([]model.BatchItemResult, 0, len(batch.Elements))
	for _, element := range batch.Elements {
		key := model.UModelElementKey(element)
		spec, _ := json.Marshal(element.Spec)
		res, err := conn.Execute(stmt, map[string]any{
			"key":     key,
			"kind":    element.Kind,
			"domain":  element.Domain,
			"name":    element.Name,
			"version": element.Version,
			"spec":    string(spec),
		})
		if err != nil {
			return model.WriteResult{}, err
		}
		res.Close()
		items = append(items, model.BatchItemResult{ID: key, OK: true})
	}
	return model.WriteResult{Accepted: len(batch.Elements), Items: items}, nil
}

func (p *Provider) DeleteUModelElements(ctx context.Context, workspace string, ids []string) (model.WriteResult, error) {
	conn, err := p.conn(workspace)
	if err != nil {
		return model.WriteResult{}, err
	}
	stmt, err := conn.Prepare(deleteUModelNodeStatement)
	if err != nil {
		return model.WriteResult{}, err
	}
	defer stmt.Close()

	items := make([]model.BatchItemResult, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			items = append(items, model.BatchItemResult{ID: id, OK: false, Code: string(apperrors.CodeValidationFailed), Message: "umodel element id is required"})
			continue
		}
		rows, err := runRows(conn, `MATCH (n:umodel_node {key: $key}) RETURN n.key AS key LIMIT 1;`, map[string]any{"key": id})
		if err != nil {
			return model.WriteResult{}, err
		}
		if len(rows) == 0 {
			items = append(items, model.BatchItemResult{ID: id, OK: false, Code: string(apperrors.CodeNotFound), Message: "umodel element not found"})
			continue
		}
		res, err := conn.Execute(stmt, map[string]any{"key": id})
		if err != nil {
			return model.WriteResult{}, err
		}
		res.Close()
		items = append(items, model.BatchItemResult{ID: id, OK: true})
	}
	return summarizeItems(items), nil
}

func (p *Provider) GetUModelSnapshot(ctx context.Context, req model.UModelSnapshotRequest) (model.UModelSnapshot, error) {
	conn, err := p.conn(req.Workspace)
	if err != nil {
		return model.UModelSnapshot{}, err
	}
	rows, err := runRows(conn, `MATCH (n:umodel_node) RETURN n.kind AS kind, n.domain AS domain, n.name AS name, n.version AS version, n.spec AS spec ORDER BY n.key;`, nil)
	if err != nil {
		return model.UModelSnapshot{}, err
	}
	elements := make([]model.UModelElement, 0, len(rows))
	for _, row := range rows {
		spec := map[string]any{}
		_ = json.Unmarshal([]byte(asString(row["spec"])), &spec)
		elements = append(elements, model.UModelElement{
			Kind:    asString(row["kind"]),
			Domain:  asString(row["domain"]),
			Name:    asString(row["name"]),
			Version: asString(row["version"]),
			Spec:    spec,
		})
	}
	version := req.Version
	if version == "" {
		version = graphstore.ProviderTypeLadybug
	}
	return model.UModelSnapshot{Workspace: req.Workspace, Version: version, Elements: elements}, nil
}

func (p *Provider) WriteEntities(ctx context.Context, batch model.EntityWriteBatch) (model.WriteResult, error) {
	conn, err := p.conn(batch.Workspace)
	if err != nil {
		return model.WriteResult{}, err
	}
	stmt, err := conn.Prepare(upsertEntityStatement)
	if err != nil {
		return model.WriteResult{}, err
	}
	defer stmt.Close()

	items := make([]model.BatchItemResult, 0, len(batch.Entities))
	for _, payload := range batch.Entities {
		key := graphstore.EntityKey(payload)
		if err := executeEntityUpsert(conn, stmt, payload); err != nil {
			return model.WriteResult{}, err
		}
		items = append(items, model.BatchItemResult{ID: key, OK: true})
	}
	return model.WriteResult{Accepted: len(batch.Entities), Items: items}, nil
}

func (p *Provider) WriteRelations(ctx context.Context, batch model.RelationWriteBatch) (model.WriteResult, error) {
	conn, err := p.conn(batch.Workspace)
	if err != nil {
		return model.WriteResult{}, err
	}
	entityStmt, err := conn.Prepare(upsertEntityStatement)
	if err != nil {
		return model.WriteResult{}, err
	}
	defer entityStmt.Close()
	relationStmt, err := conn.Prepare(upsertTopoStatement)
	if err != nil {
		return model.WriteResult{}, err
	}
	defer relationStmt.Close()

	items := make([]model.BatchItemResult, 0, len(batch.Relations))
	for _, payload := range batch.Relations {
		src := relationEndpoint(payload, "src")
		dest := relationEndpoint(payload, "dest")
		if err := ensureEntityNode(conn, entityStmt, src); err != nil {
			return model.WriteResult{}, err
		}
		if err := ensureEntityNode(conn, entityStmt, dest); err != nil {
			return model.WriteResult{}, err
		}
		properties, _ := json.Marshal(payload)
		key := graphstore.RelationKey(payload)
		res, err := conn.Execute(relationStmt, map[string]any{
			"src_key":             graphstore.EntityKey(src),
			"dest_key":            graphstore.EntityKey(dest),
			"relation_key":        key,
			"relation_type":       asString(payload["__relation_type__"]),
			"method":              methodOf(payload),
			"first_observed_time": asInt64(payload["__first_observed_time__"]),
			"last_observed_time":  asInt64(payload["__last_observed_time__"]),
			"keep_alive_seconds":  asInt64(payload["__keep_alive_seconds__"]),
			"deleted":             isDeletedMethod(methodOf(payload)),
			"properties":          string(properties),
		})
		if err != nil {
			return model.WriteResult{}, err
		}
		res.Close()
		items = append(items, model.BatchItemResult{ID: key, OK: true})
	}
	return model.WriteResult{Accepted: len(batch.Relations), Items: items}, nil
}

func (p *Provider) QueryEntities(ctx context.Context, plan model.EntityQueryPlan) (model.QueryResult, error) {
	conn, err := p.conn(plan.Workspace)
	if err != nil {
		return model.QueryResult{}, err
	}
	rows, err := runRows(conn, fmt.Sprintf(`MATCH (e:entity) WHERE e.deleted = false RETURN e.entity_key AS entity_key, e.domain AS domain, e.entity_type AS entity_type, e.entity_id AS entity_id, e.method AS method, e.first_observed_time AS first_observed_time, e.last_observed_time AS last_observed_time, e.keep_alive_seconds AS keep_alive_seconds, e.deleted AS deleted, e.properties AS properties ORDER BY e.entity_key LIMIT %d;`, ladybugCapabilities().MaxLimit), nil)
	if err != nil {
		return model.QueryResult{}, err
	}
	limit := boundedLimit(plan.Limit)
	out := make([]map[string]any, 0, limit)
	for _, row := range rows {
		payload := entityPayloadFromRow(row)
		if !entityMatches(payload, plan) {
			continue
		}
		out = append(out, entityRow(payload))
		if len(out) == limit {
			break
		}
	}
	return model.QueryResult{Columns: []string{"__domain__", "__entity_type__", "__entity_id__", "__method__", "__deleted__"}, Rows: out, Page: model.PageRequest{Limit: limit}}, nil
}

func (p *Provider) QueryTopo(ctx context.Context, plan model.TopoQueryPlan) (model.QueryResult, error) {
	conn, err := p.conn(plan.Workspace)
	if err != nil {
		return model.QueryResult{}, err
	}
	if plan.GraphCall != nil && plan.GraphCall.Name == "cypher" {
		return p.queryControlledCypher(conn, plan)
	}
	rows, err := runRows(conn, fmt.Sprintf(`MATCH (s:entity)-[r:topo]->(d:entity) WHERE r.deleted = false RETURN s.entity_key AS src, r.relation_type AS relation, d.entity_key AS dest, r.method AS method, r.first_observed_time AS first_observed_time, r.last_observed_time AS last_observed_time, r.keep_alive_seconds AS keep_alive_seconds, r.deleted AS deleted, r.properties AS properties ORDER BY r.relation_key LIMIT %d;`, ladybugCapabilities().MaxLimit), nil)
	if err != nil {
		return model.QueryResult{}, err
	}
	limit := boundedLimit(plan.Limit)
	out := make([]map[string]any, 0, limit)
	for _, row := range rows {
		payload := relationPayloadFromRow(row)
		if !relationMatches(payload, plan) {
			continue
		}
		out = append(out, relationRow(payload))
		if len(out) == limit {
			break
		}
	}
	return model.QueryResult{Columns: []string{"src", "relation", "dest", "__relation_type__", "__deleted__"}, Rows: out, Page: model.PageRequest{Limit: limit}}, nil
}

func (p *Provider) queryControlledCypher(conn *lbug.Connection, plan model.TopoQueryPlan) (model.QueryResult, error) {
	if err := validateReadOnlyCypher(plan.GraphCall.Cypher); err != nil {
		return model.QueryResult{}, err
	}
	graph, err := p.cypherGraph(conn, plan)
	if err != nil {
		return model.QueryResult{}, err
	}
	limit := boundedLimit(plan.Limit)
	result, err := cypher.Execute(plan.GraphCall.Cypher, graph, plan.Params, cypher.Options{Limit: limit})
	if err != nil {
		return model.QueryResult{}, err
	}
	return model.QueryResult{
		Columns: result.Columns,
		Rows:    result.Rows,
		Page:    model.PageRequest{Limit: result.Limit},
	}, nil
}

func (p *Provider) cypherGraph(conn *lbug.Connection, plan model.TopoQueryPlan) (cypher.Graph, error) {
	entityRows, err := runRows(conn, fmt.Sprintf(`MATCH (e:entity) WHERE e.deleted = false RETURN e.entity_key AS entity_key, e.domain AS domain, e.entity_type AS entity_type, e.entity_id AS entity_id, e.method AS method, e.first_observed_time AS first_observed_time, e.last_observed_time AS last_observed_time, e.keep_alive_seconds AS keep_alive_seconds, e.deleted AS deleted, e.properties AS properties ORDER BY e.entity_key LIMIT %d;`, ladybugCapabilities().MaxLimit), nil)
	if err != nil {
		return cypher.Graph{}, err
	}
	nodes := map[string]cypher.Node{}
	for _, row := range entityRows {
		payload := entityPayloadFromRow(row)
		if !entityMatches(payload, plan) {
			continue
		}
		key := graphstore.EntityKey(payload)
		nodes[key] = cypher.Node{
			ID:         key,
			Labels:     entityLabels(payload),
			Properties: cloneMap(map[string]any(payload)),
		}
	}

	relationRows, err := runRows(conn, fmt.Sprintf(`MATCH (s:entity)-[r:topo]->(d:entity) WHERE r.deleted = false RETURN s.entity_key AS src, r.relation_type AS relation, d.entity_key AS dest, r.method AS method, r.first_observed_time AS first_observed_time, r.last_observed_time AS last_observed_time, r.keep_alive_seconds AS keep_alive_seconds, r.deleted AS deleted, r.properties AS properties ORDER BY r.relation_key LIMIT %d;`, ladybugCapabilities().MaxLimit), nil)
	if err != nil {
		return cypher.Graph{}, err
	}
	edges := []cypher.Edge{}
	for _, row := range relationRows {
		payload := relationPayloadFromRow(row)
		if !relationMatches(payload, plan) {
			continue
		}
		src := relationEndpoint(payload, "src")
		dest := relationEndpoint(payload, "dest")
		srcKey := graphstore.EntityKey(src)
		destKey := graphstore.EntityKey(dest)
		if _, ok := nodes[srcKey]; !ok {
			nodes[srcKey] = cypher.Node{ID: srcKey, Labels: entityLabels(src), Properties: cloneMap(map[string]any(src))}
		}
		if _, ok := nodes[destKey]; !ok {
			nodes[destKey] = cypher.Node{ID: destKey, Labels: entityLabels(dest), Properties: cloneMap(map[string]any(dest))}
		}
		edges = append(edges, cypher.Edge{
			ID:         graphstore.RelationKey(payload),
			From:       srcKey,
			To:         destKey,
			Type:       asString(payload["__relation_type__"]),
			Properties: cloneMap(map[string]any(payload)),
		})
	}
	return cypher.Graph{Nodes: nodes, Edges: edges}, nil
}

func (p *Provider) Capabilities(ctx context.Context) (model.GraphStoreCapabilities, error) {
	return ladybugCapabilities(), nil
}

func (p *Provider) Health(ctx context.Context) (model.GraphStoreHealth, error) {
	return model.GraphStoreHealth{Provider: graphstore.ProviderTypeLadybug, Status: "ok"}, nil
}

func (p *Provider) conn(workspace string) (*lbug.Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	handle := p.workspaces[workspace]
	if handle == nil {
		if err := p.openWorkspaceLocked(model.WorkspaceMetadata{ID: workspace}); err != nil {
			return nil, err
		}
		handle = p.workspaces[workspace]
	}
	return handle.conn, nil
}

func (p *Provider) openWorkspaceLocked(workspace model.WorkspaceMetadata) error {
	if workspace.ID == "" {
		return fmt.Errorf("workspace id is required")
	}
	dbPath := ladybugPath(p.root, workspace)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return err
	}
	db, err := lbug.OpenDatabase(dbPath, defaultSystemConfig())
	if err != nil {
		return err
	}
	conn, err := lbug.OpenConnection(db)
	if err != nil {
		db.Close()
		return err
	}
	p.workspaces[workspace.ID] = &workspaceHandle{db: db, conn: conn}
	return p.ensureSchemaLocked(workspace.ID)
}

func (p *Provider) ensureSchemaLocked(workspace string) error {
	handle := p.workspaces[workspace]
	if handle == nil {
		return fmt.Errorf("workspace %q is not open", workspace)
	}
	for _, statement := range []string{
		`CREATE NODE TABLE IF NOT EXISTS umodel_node (key STRING, kind STRING, domain STRING, name STRING, version STRING, spec STRING, PRIMARY KEY(key));`,
		`CREATE NODE TABLE IF NOT EXISTS entity (entity_key STRING, domain STRING, entity_type STRING, entity_id STRING, method STRING, first_observed_time INT64, last_observed_time INT64, keep_alive_seconds INT64, deleted BOOL, properties STRING, PRIMARY KEY(entity_key));`,
		`CREATE REL TABLE IF NOT EXISTS topo (FROM entity TO entity, relation_key STRING, relation_type STRING, method STRING, first_observed_time INT64, last_observed_time INT64, keep_alive_seconds INT64, deleted BOOL, properties STRING);`,
	} {
		res, err := handle.conn.Query(statement)
		if err != nil {
			return err
		}
		res.Close()
	}
	return nil
}

func defaultSystemConfig() lbug.SystemConfig {
	config := lbug.DefaultSystemConfig()
	config.BufferPoolSize = 256 * 1024 * 1024
	config.MaxNumThreads = 4
	return config
}

func executeEntityUpsert(conn *lbug.Connection, stmt *lbug.PreparedStatement, payload model.EntityPayload) error {
	properties, _ := json.Marshal(payload)
	res, err := conn.Execute(stmt, map[string]any{
		"entity_key":          graphstore.EntityKey(payload),
		"domain":              asString(payload["__domain__"]),
		"entity_type":         asString(payload["__entity_type__"]),
		"entity_id":           asString(payload["__entity_id__"]),
		"method":              methodOf(payload),
		"first_observed_time": asInt64(payload["__first_observed_time__"]),
		"last_observed_time":  asInt64(payload["__last_observed_time__"]),
		"keep_alive_seconds":  asInt64(payload["__keep_alive_seconds__"]),
		"deleted":             isDeletedMethod(methodOf(payload)),
		"properties":          string(properties),
	})
	if err != nil {
		return err
	}
	res.Close()
	return nil
}

func ensureEntityNode(conn *lbug.Connection, stmt *lbug.PreparedStatement, payload model.EntityPayload) error {
	rows, err := runRows(conn, `MATCH (e:entity {entity_key: $entity_key}) RETURN e.entity_key AS entity_key LIMIT 1;`, map[string]any{
		"entity_key": graphstore.EntityKey(payload),
	})
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		return nil
	}
	return executeEntityUpsert(conn, stmt, payload)
}

func relationEndpoint(payload model.RelationPayload, side string) model.EntityPayload {
	entity := model.EntityPayload{
		"__method__":                  "Update",
		"__first_observed_time__":     payload["__first_observed_time__"],
		"__last_observed_time__":      payload["__last_observed_time__"],
		"__keep_alive_seconds__":      payload["__keep_alive_seconds__"],
		"__placeholder_from_topo__":   true,
		"__placeholder_relation__":    asString(payload["__relation_type__"]),
		"__placeholder_endpoint__":    side,
		"__placeholder_entity_role__": side,
	}
	entity["__domain__"] = payload["__"+side+"_domain__"]
	entity["__entity_type__"] = payload["__"+side+"_entity_type__"]
	entity["__entity_id__"] = payload["__"+side+"_entity_id__"]
	return entity
}

func entityLabels(payload model.EntityPayload) []string {
	domain := asString(payload["__domain__"])
	entityType := asString(payload["__entity_type__"])
	labels := []string{}
	if entityType != "" {
		labels = append(labels, entityType)
	}
	if domain != "" && entityType != "" {
		labels = append(labels, domain+"@"+entityType)
	}
	return labels
}

func entityPayloadFromRow(row map[string]any) model.EntityPayload {
	payload := model.EntityPayload{}
	_ = json.Unmarshal([]byte(asString(row["properties"])), &payload)
	payload["__domain__"] = asString(row["domain"])
	payload["__entity_type__"] = asString(row["entity_type"])
	payload["__entity_id__"] = asString(row["entity_id"])
	payload["__method__"] = methodOf(payloadWithDefault(payload, "__method__", row["method"]))
	payload["__first_observed_time__"] = asInt64(row["first_observed_time"])
	payload["__last_observed_time__"] = asInt64(row["last_observed_time"])
	payload["__keep_alive_seconds__"] = asInt64(row["keep_alive_seconds"])
	payload["__deleted__"] = asBool(row["deleted"])
	return payload
}

func relationPayloadFromRow(row map[string]any) model.RelationPayload {
	payload := model.RelationPayload{}
	_ = json.Unmarshal([]byte(asString(row["properties"])), &payload)
	payload["src"] = asString(row["src"])
	payload["dest"] = asString(row["dest"])
	payload["relation"] = asString(row["relation"])
	payload["__relation_type__"] = asString(row["relation"])
	payload["__method__"] = methodOf(payloadWithDefault(payload, "__method__", row["method"]))
	payload["__first_observed_time__"] = asInt64(row["first_observed_time"])
	payload["__last_observed_time__"] = asInt64(row["last_observed_time"])
	payload["__keep_alive_seconds__"] = asInt64(row["keep_alive_seconds"])
	payload["__deleted__"] = asBool(row["deleted"])
	return payload
}

func payloadWithDefault(payload map[string]any, key string, fallback any) map[string]any {
	if asString(payload[key]) == "" {
		payload[key] = fallback
	}
	return payload
}

func entityRow(payload model.EntityPayload) map[string]any {
	row := cloneMap(map[string]any(payload))
	row["__deleted__"] = asBool(payload["__deleted__"])
	return row
}

func relationRow(payload model.RelationPayload) map[string]any {
	row := cloneMap(map[string]any(payload))
	if asString(row["src"]) == "" {
		row["src"] = strings.Join([]string{
			asString(payload["__src_domain__"]),
			asString(payload["__src_entity_type__"]),
			asString(payload["__src_entity_id__"]),
		}, "/")
	}
	if asString(row["dest"]) == "" {
		row["dest"] = strings.Join([]string{
			asString(payload["__dest_domain__"]),
			asString(payload["__dest_entity_type__"]),
			asString(payload["__dest_entity_id__"]),
		}, "/")
	}
	row["relation"] = asString(payload["__relation_type__"])
	row["__deleted__"] = asBool(payload["__deleted__"])
	return row
}

func entityMatches(payload model.EntityPayload, plan model.QueryPlan) bool {
	if asBool(payload["__deleted__"]) {
		return false
	}
	if !matchesFilter(asString(payload["__domain__"]), plan.Filters["domain"]) {
		return false
	}
	if !matchesFilter(asString(payload["__entity_type__"]), plan.Filters["name"]) {
		return false
	}
	if !matchesIDs(asString(payload["__entity_id__"]), plan.Filters["ids"]) {
		return false
	}
	if !matchesSearch(map[string]any(payload), plan.Filters["query"]) {
		return false
	}
	return visibleInRange(map[string]any(payload), plan.TimeRange)
}

func relationMatches(payload model.RelationPayload, plan model.QueryPlan) bool {
	if asBool(payload["__deleted__"]) && !hasTimeRange(plan.TimeRange) {
		return false
	}
	if !matchesFilter(asString(payload["__relation_type__"]), firstFilter(plan.Filters["relation_type"], plan.Filters["type"])) {
		return false
	}
	if !matchesFilter(relationEndpointKey(payload, "src"), plan.Filters["src"]) {
		return false
	}
	if !matchesFilter(relationEndpointKey(payload, "dest"), plan.Filters["dest"]) {
		return false
	}
	if plan.GraphCall != nil && len(plan.GraphCall.SeedIDs) > 0 {
		srcID := asString(payload["__src_entity_id__"])
		destID := asString(payload["__dest_entity_id__"])
		if !containsID(plan.GraphCall.SeedIDs, srcID) && !containsID(plan.GraphCall.SeedIDs, destID) {
			return false
		}
	}
	if !matchesSearch(map[string]any(payload), plan.Filters["query"]) {
		return false
	}
	return visibleInRange(map[string]any(payload), plan.TimeRange)
}

func relationEndpointKey(payload model.RelationPayload, side string) string {
	return strings.Join([]string{
		asString(payload["__"+side+"_domain__"]),
		asString(payload["__"+side+"_entity_type__"]),
		asString(payload["__"+side+"_entity_id__"]),
	}, "/")
}

func hasTimeRange(timeRange model.TimeRange) bool {
	return timeRange.From != nil || timeRange.To != nil
}

func visibleInRange(payload map[string]any, timeRange model.TimeRange) bool {
	if timeRange.From == nil && timeRange.To == nil {
		return true
	}

	first, hasFirst := int64Value(payload["__first_observed_time__"])
	last, hasLast := int64Value(payload["__last_observed_time__"])
	if !hasFirst || !hasLast {
		return true
	}

	keepAlive, _ := int64Value(payload["__keep_alive_seconds__"])
	from := int64(0)
	if timeRange.From != nil {
		from = timeRange.From.Unix()
	}
	to := time.Now().Add(100 * 365 * 24 * time.Hour).Unix()
	if timeRange.To != nil {
		to = timeRange.To.Unix()
	}
	if first >= to {
		return false
	}
	if asBool(payload["__deleted__"]) {
		return last > from
	}
	return last+keepAlive > from
}

func matchesFilter(value string, filter any) bool {
	if filter == nil || asString(filter) == "" || asString(filter) == "*" {
		return true
	}
	pattern := asString(filter)
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return value == pattern
}

func matchesIDs(value string, filter any) bool {
	if filter == nil {
		return true
	}
	switch ids := filter.(type) {
	case []string:
		for _, id := range ids {
			if id == value {
				return true
			}
		}
		return false
	case []any:
		for _, id := range ids {
			if asString(id) == value {
				return true
			}
		}
		return false
	default:
		return asString(filter) == "" || asString(filter) == value
	}
}

func matchesSearch(payload map[string]any, filter any) bool {
	query := strings.ToLower(asString(filter))
	if query == "" {
		return true
	}
	for _, value := range payload {
		if strings.Contains(strings.ToLower(asString(value)), query) {
			return true
		}
	}
	return false
}

func firstFilter(values ...any) any {
	for _, value := range values {
		if asString(value) != "" {
			return value
		}
	}
	return nil
}

func containsID(ids []string, value string) bool {
	for _, id := range ids {
		if id == value {
			return true
		}
	}
	return false
}

func methodOf(payload map[string]any) string {
	method := asString(payload["__method__"])
	if method == "" {
		return "Update"
	}
	return method
}

func isDeletedMethod(method string) bool {
	return method == "Expire" || method == "Delete"
}

func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case string:
		if typed == "" {
			return 0, false
		}
		var n int64
		_, err := fmt.Sscan(typed, &n)
		return n, err == nil
	default:
		return 0, false
	}
}

func cloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func runRows(conn *lbug.Connection, query string, args map[string]any) ([]map[string]any, error) {
	var (
		result *lbug.QueryResult
		err    error
	)
	if args == nil {
		result, err = conn.Query(query)
	} else {
		stmt, prepareErr := conn.Prepare(query)
		if prepareErr != nil {
			return nil, prepareErr
		}
		defer stmt.Close()
		result, err = conn.Execute(stmt, args)
	}
	if err != nil {
		return nil, err
	}
	defer result.Close()

	rows := []map[string]any{}
	columns := result.GetColumnNames()
	for result.HasNext() {
		tuple, err := result.Next()
		if err != nil {
			return nil, err
		}
		values, err := tuple.GetAsSlice()
		tuple.Close()
		if err != nil {
			return nil, err
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			if i < len(values) {
				row[column] = values[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func validateReadOnlyCypher(query string) error {
	return cypher.ValidateReadOnly(query)
}

func columnsFromRows(rows []map[string]any) []string {
	seen := map[string]struct{}{}
	columns := []string{}
	for _, row := range rows {
		for column := range row {
			if _, ok := seen[column]; ok {
				continue
			}
			seen[column] = struct{}{}
			columns = append(columns, column)
		}
	}
	sort.Strings(columns)
	return columns
}

func limitResultRows(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func summarizeItems(items []model.BatchItemResult) model.WriteResult {
	result := model.WriteResult{Items: items}
	for _, item := range items {
		if item.OK {
			result.Accepted++
		} else {
			result.Failed++
		}
	}
	return result
}

func ladybugPath(root string, workspace model.WorkspaceMetadata) string {
	if workspace.Paths.Root != "" {
		return filepath.Join(workspace.Paths.Root, "storage", "graph", "local", "ladybug")
	}
	return filepath.Join(root, "instances", workspace.ID, "storage", "graph", "local", "ladybug")
}

func boundedLimit(limit int) int {
	max := ladybugCapabilities().MaxLimit
	if limit < 0 {
		return max
	}
	if limit == 0 {
		return 100
	}
	if limit > max {
		return max
	}
	return limit
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func asInt64(value any) int64 {
	n, _ := int64Value(value)
	return n
}

func asBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	default:
		return false
	}
}

func ladybugCapabilities() model.GraphStoreCapabilities {
	return model.GraphStoreCapabilities{
		EntitySearch:       true,
		GraphMatch:         true,
		GraphCallNeighbors: true,
		ControlledCypher:   true,
		TimeVisibility:     true,
		ServerSideFilter:   false,
		MaxDepth:           10,
		MaxLimit:           10000,
		Timeout:            "60s",
	}
}
