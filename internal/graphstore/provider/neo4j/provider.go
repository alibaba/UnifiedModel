// Package neo4j persists GraphStore records through Neo4j's Bolt driver. Query
// evaluation stays in the shared Go engine so workspace and lifecycle semantics
// are identical to memory/file.memory, including controlled read-only Cypher.
package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	"github.com/alibaba/UnifiedModel/pkg/model"
	driver "github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Provider struct {
	driver    driver.DriverWithContext
	database  string
	timeout   time.Duration
	dialect   string
	writeGate chan struct{}
}

var _ contract.GraphStore = (*Provider)(nil)

const (
	dialectNeo4j      = "neo4j"
	dialectOpenCypher = "opencypher"
)

type settings struct {
	uri, username, password, database string
	timeout                           time.Duration
	dialect                           string
}

func configuration(options map[string]string) (settings, error) {
	value := func(key, fallback string) string {
		if v, ok := options[key]; ok {
			return v
		}
		if v, ok := os.LookupEnv("NEO4J_" + strings.ToUpper(key)); ok {
			return v
		}
		return fallback
	}
	dialect := strings.ToLower(strings.TrimSpace(value("dialect", dialectNeo4j)))
	if dialect != dialectNeo4j && dialect != dialectOpenCypher {
		return settings{}, fmt.Errorf("neo4j: dialect must be neo4j or opencypher")
	}
	database := "neo4j"
	if dialect == dialectOpenCypher {
		database = ""
	}
	c := settings{
		uri: value("uri", "bolt://localhost:7687"), username: value("username", "neo4j"),
		password: value("password", ""), database: value("database", database), dialect: dialect,
	}
	u, err := url.Parse(c.uri)
	if err != nil || u.Hostname() == "" {
		return c, fmt.Errorf("neo4j: uri must be a valid Bolt or Neo4j URI")
	}
	switch u.Scheme {
	case "bolt", "bolt+s", "bolt+ssc", "neo4j", "neo4j+s", "neo4j+ssc":
	default:
		return c, fmt.Errorf("neo4j: unsupported URI scheme")
	}
	if c.dialect == dialectOpenCypher && !strings.HasPrefix(u.Scheme, "bolt") {
		return c, fmt.Errorf("neo4j: opencypher requires a direct bolt://, bolt+s://, or bolt+ssc:// URI; routing is not portable")
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.Fragment != "" {
		return c, fmt.Errorf("neo4j: URI must not contain credentials, a path, or a fragment; use username/password/database options")
	}
	if c.username == "" || c.password == "" || (c.dialect == dialectNeo4j && c.database == "") {
		return c, fmt.Errorf("neo4j: username, password, and database are required (set NEO4J_USERNAME, NEO4J_PASSWORD, NEO4J_DATABASE or ProviderConfig.Options)")
	}
	c.timeout, err = time.ParseDuration(value("timeout", "10s"))
	if err != nil || c.timeout <= 0 {
		return c, fmt.Errorf("neo4j: timeout must be a positive duration")
	}
	return c, nil
}

func init() {
	graphstore.RegisterProvider(graphstore.ProviderTypeNeo4j, func(config graphstore.ProviderConfig) (contract.GraphStore, error) {
		return NewProvider(config)
	})
}

func NewProvider(config graphstore.ProviderConfig) (*Provider, error) {
	c, err := configuration(config.Options)
	if err != nil {
		return nil, err
	}
	d, err := driver.NewDriverWithContext(c.uri, driver.BasicAuth(c.username, c.password, ""), func(config *driver.Config) {
		config.SocketConnectTimeout = c.timeout
		config.ConnectionAcquisitionTimeout = c.timeout
		config.MaxTransactionRetryTime = c.timeout
		config.TelemetryDisabled = c.dialect == dialectOpenCypher
	})
	if err != nil {
		return nil, fmt.Errorf("neo4j: create driver: %w", err)
	}
	p := &Provider{driver: d, database: c.database, timeout: c.timeout, dialect: c.dialect, writeGate: make(chan struct{}, 1)}
	if err := p.initialize(context.Background()); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("neo4j: initialize %s: %w", c.dialect, err)
	}
	return p, nil
}

func (p *Provider) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	return p.driver.Close(ctx)
}

// Every operation gets its own session. Native Neo4j sessions share bookmarks.
// openCypher uses a direct endpoint and serializes all writes in this provider
// instance; portable Cypher alone cannot guarantee cross-process uniqueness.
func (p *Provider) transaction(ctx context.Context, write bool, work func(context.Context, driver.ManagedTransaction) (any, error)) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	if write && p.dialect == dialectOpenCypher {
		select {
		case p.writeGate <- struct{}{}:
			defer func() { <-p.writeGate }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	mode := driver.AccessModeRead
	if write {
		mode = driver.AccessModeWrite
	}
	config := driver.SessionConfig{DatabaseName: p.database, AccessMode: mode}
	if p.dialect != dialectOpenCypher {
		config.BookmarkManager = p.driver.ExecuteQueryBookmarkManager()
	}
	session := p.driver.NewSession(ctx, config)
	defer session.Close(ctx)
	callback := func(tx driver.ManagedTransaction) (any, error) { return work(ctx, tx) }
	if write {
		return session.ExecuteWrite(ctx, callback, driver.WithTxTimeout(p.timeout))
	}
	return session.ExecuteRead(ctx, callback, driver.WithTxTimeout(p.timeout))
}

func consume(ctx context.Context, tx driver.ManagedTransaction, statement string, params map[string]any) error {
	result, err := tx.Run(ctx, statement, params)
	if err != nil {
		return err
	}
	_, err = result.Consume(ctx)
	return err
}

func (p *Provider) initialize(ctx context.Context) error {
	if p.dialect == dialectOpenCypher {
		// Query-only connectivity check: index/constraint DDL is backend-specific.
		_, err := p.Health(ctx)
		return err
	}
	for _, statement := range []string{
		`CREATE CONSTRAINT umodel_workspace_id IF NOT EXISTS FOR (w:UModelWorkspace) REQUIRE w.id IS UNIQUE`,
		`CREATE CONSTRAINT umodel_element_key IF NOT EXISTS FOR (n:UModelElement) REQUIRE (n.workspace, n.key) IS UNIQUE`,
		`CREATE CONSTRAINT umodel_entity_key IF NOT EXISTS FOR (n:UModelEntity) REQUIRE (n.workspace, n.key) IS UNIQUE`,
		`CREATE INDEX umodel_relation_key IF NOT EXISTS FOR ()-[r:UMODEL_RELATION]-() ON (r.workspace, r.key)`,
	} {
		if _, err := p.transaction(ctx, true, func(ctx context.Context, tx driver.ManagedTransaction) (any, error) {
			return nil, consume(ctx, tx, statement, nil)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provider) OpenWorkspace(ctx context.Context, workspace model.WorkspaceMetadata) error {
	if workspace.ID == "" {
		return fmt.Errorf("workspace is required")
	}
	_, err := p.transaction(ctx, true, func(ctx context.Context, tx driver.ManagedTransaction) (any, error) {
		return nil, p.prepareWorkspace(ctx, tx, workspace.ID, false)
	})
	return err
}

func (p *Provider) EnsureSchema(ctx context.Context, workspace string) error {
	// Native constraints are installed at startup. openCypher manages no schema.
	return p.OpenWorkspace(ctx, model.WorkspaceMetadata{ID: workspace})
}

type collection int

const (
	models collection = 1 << iota
	entities
	relations
)

// keys == nil loads the entire selected collection. Writes load only the keys
// they modify, keeping lifecycle evaluation proportional to the input batch.
func loadSnapshot(ctx context.Context, tx driver.ManagedTransaction, workspace string, collections collection, keys []string) (graphstore.WorkspaceSnapshot, error) {
	snapshot := graphstore.WorkspaceSnapshot{
		UModels: make(map[string]model.UModelElement), Entities: make(map[string]model.EntityPayload),
		Relations: make(map[string]model.RelationPayload),
	}
	params := map[string]any{"workspace": workspace, "keys": nil}
	if keys != nil {
		params["keys"] = keys
	}
	for _, c := range []struct {
		kind  collection
		match string
	}{
		{models, "MATCH (n:UModelElement)"},
		{entities, "MATCH (n:UModelEntity)"},
		{relations, "MATCH ()-[n:UMODEL_RELATION]->()"},
	} {
		if collections&c.kind == 0 {
			continue
		}
		result, err := tx.Run(ctx, c.match+` WHERE n.workspace = $workspace AND n.data IS NOT NULL AND ($keys IS NULL OR n.key IN $keys) RETURN n.key AS key, n.data AS data`, params)
		if err != nil {
			return snapshot, err
		}
		for result.Next(ctx) {
			record := result.Record()
			key, keyOK := record.Values[0].(string)
			data, dataOK := record.Values[1].(string)
			if !keyOK || !dataOK {
				return snapshot, fmt.Errorf("neo4j: invalid stored key or data")
			}
			// Without native constraints an external writer could create duplicate
			// keys. Never silently collapse those records into the in-memory map.
			_, modelExists := snapshot.UModels[key]
			_, entityExists := snapshot.Entities[key]
			_, relationExists := snapshot.Relations[key]
			if (c.kind == models && modelExists) || (c.kind == entities && entityExists) || (c.kind == relations && relationExists) {
				return snapshot, fmt.Errorf("neo4j: duplicate graph record %q in workspace %q; repair duplicate data and ensure only one writer in opencypher mode", key, workspace)
			}
			decode := func(target any) error {
				decoder := json.NewDecoder(strings.NewReader(data))
				decoder.UseNumber()
				if err := decoder.Decode(target); err != nil {
					return fmt.Errorf("neo4j: decode record %q: %w", key, err)
				}
				return nil
			}
			switch c.kind {
			case models:
				var value model.UModelElement
				err = decode(&value)
				snapshot.UModels[key] = value
			case entities:
				var value model.EntityPayload
				err = decode(&value)
				snapshot.Entities[key] = value
			case relations:
				var value model.RelationPayload
				err = decode(&value)
				snapshot.Relations[key] = value
			}
			if err != nil {
				return snapshot, err
			}
		}
		if err := result.Err(); err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}

// Native Neo4j uses a database workspace lock for cross-process writers.
// openCypher relies on transaction's single-instance gate instead. Both reload
// and re-evaluate the batch on retries; no local cache survives a failed commit.
func (p *Provider) mutate(ctx context.Context, workspace string, c collection, keys []string, apply func(context.Context, *graphstore.MemoryStore) (model.WriteResult, error)) (model.WriteResult, error) {
	if workspace == "" {
		return model.WriteResult{}, fmt.Errorf("workspace is required")
	}
	value, err := p.transaction(ctx, true, func(ctx context.Context, tx driver.ManagedTransaction) (any, error) {
		if err := p.prepareWorkspace(ctx, tx, workspace, true); err != nil {
			return nil, err
		}
		snapshot, err := loadSnapshot(ctx, tx, workspace, c, keys)
		if err != nil {
			return nil, err
		}
		memory := graphstore.NewMemoryStoreFromSnapshot(workspace, snapshot)
		result, err := apply(ctx, memory)
		if err != nil || result.Accepted == 0 {
			return result, err
		}
		if err := saveChanges(ctx, tx, workspace, c, memory.SnapshotWorkspace(workspace), result); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return model.WriteResult{}, err
	}
	return value.(model.WriteResult), nil
}

func (p *Provider) prepareWorkspace(ctx context.Context, tx driver.ManagedTransaction, workspace string, lock bool) error {
	statement := `MERGE (w:UModelWorkspace {id: $workspace})`
	if lock && p.dialect != dialectOpenCypher {
		statement += ` SET w.revision = coalesce(w.revision, 0) + 1`
	}
	result, err := tx.Run(ctx, statement+` RETURN count(w) AS count`, map[string]any{"workspace": workspace})
	if err != nil {
		return err
	}
	record, err := result.Single(ctx)
	if err != nil {
		return err
	}
	count, ok := record.Get("count")
	if !ok || count != int64(1) {
		return fmt.Errorf("neo4j: workspace %q has duplicate or invalid workspace markers; repair data before writing", workspace)
	}
	return nil
}

func saveChanges(ctx context.Context, tx driver.ManagedTransaction, workspace string, c collection, snapshot graphstore.WorkspaceSnapshot, result model.WriteResult) error {
	rows := make([]map[string]any, 0, result.Accepted)
	deleted := []string{}
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
			element, exists := snapshot.UModels[item.ID]
			if !exists {
				deleted = append(deleted, item.ID)
				continue
			}
			payload = element
			row["domain"], row["kind"], row["name"] = element.Domain, element.Kind, element.Name
		case entities:
			entity := snapshot.Entities[item.ID]
			payload = entity
			row["domain"], row["entity_type"], row["entity_id"] = entity["__domain__"], entity["__entity_type__"], entity["__entity_id__"]
		case relations:
			relation := snapshot.Relations[item.ID]
			payload = relation
			row["src"], row["dest"] = endpointKey(relation, "src"), endpointKey(relation, "dest")
			row["relation_type"] = relation["__relation_type__"]
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("neo4j: encode record %q: %w", item.ID, err)
		}
		row["data"] = string(data)
		rows = append(rows, row)
	}
	params := map[string]any{"workspace": workspace, "rows": rows, "deleted": deleted}
	if len(deleted) > 0 {
		if err := consume(ctx, tx, `UNWIND $deleted AS key MATCH (n:UModelElement {workspace: $workspace, key: key}) DELETE n`, params); err != nil {
			return err
		}
	}
	if len(rows) == 0 {
		return nil
	}
	var statement string
	switch c {
	case models:
		statement = `UNWIND $rows AS row MERGE (n:UModelElement {workspace: $workspace, key: row.key}) SET n.data = row.data, n.domain = row.domain, n.kind = row.kind, n.name = row.name`
	case entities:
		statement = `UNWIND $rows AS row MERGE (n:UModelEntity {workspace: $workspace, key: row.key}) SET n.data = row.data, n.domain = row.domain, n.entity_type = row.entity_type, n.entity_id = row.entity_id`
	case relations:
		// Endpoint placeholders have no data and are excluded from entity reads.
		statement = `UNWIND $rows AS row
MERGE (s:UModelEntity {workspace: $workspace, key: row.src})
MERGE (d:UModelEntity {workspace: $workspace, key: row.dest})
MERGE (s)-[r:UMODEL_RELATION {workspace: $workspace, key: row.key}]->(d)
SET r.data = row.data, r.relation_type = row.relation_type`
	}
	return consume(ctx, tx, statement, params)
}

func endpointKey(payload model.RelationPayload, side string) string {
	return graphstore.EntityKey(model.EntityPayload{
		"__domain__": payload["__"+side+"_domain__"], "__entity_type__": payload["__"+side+"_entity_type__"],
		"__entity_id__": payload["__"+side+"_entity_id__"],
	})
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
	value, err := p.transaction(ctx, false, func(ctx context.Context, tx driver.ManagedTransaction) (any, error) {
		return loadSnapshot(ctx, tx, workspace, c, nil)
	})
	if err != nil {
		return nil, err
	}
	return graphstore.NewMemoryStoreFromSnapshot(workspace, value.(graphstore.WorkspaceSnapshot)), nil
}

func (p *Provider) GetUModelSnapshot(ctx context.Context, req model.UModelSnapshotRequest) (model.UModelSnapshot, error) {
	s, err := p.read(ctx, req.Workspace, models)
	if err != nil {
		return model.UModelSnapshot{}, err
	}
	if req.Version == "" {
		req.Version = graphstore.ProviderTypeNeo4j
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
	_, err := p.transaction(ctx, false, func(ctx context.Context, tx driver.ManagedTransaction) (any, error) {
		return nil, consume(ctx, tx, "RETURN 1", nil)
	})
	health := model.GraphStoreHealth{Provider: graphstore.ProviderTypeNeo4j, Status: "ok"}
	if p.dialect == dialectOpenCypher {
		health.Message = "opencypher mode: use one provider instance for all writes; schema and bookmarks are not managed"
	}
	if err != nil {
		health.Status = "unavailable"
		health.Message = "Neo4j database is unavailable"
	}
	return health, err
}
