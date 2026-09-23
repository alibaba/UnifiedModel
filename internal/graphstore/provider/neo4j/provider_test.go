package neo4j

import (
	"strings"
	"testing"
	"time"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
)

func TestConfiguration(t *testing.T) {
	t.Setenv("NEO4J_DIALECT", "neo4j")
	t.Setenv("NEO4J_URI", "neo4j+s://example.com")
	t.Setenv("NEO4J_USERNAME", "environment-user")
	t.Setenv("NEO4J_PASSWORD", "environment-password")
	t.Setenv("NEO4J_DATABASE", "environment-db")
	t.Setenv("NEO4J_TIMEOUT", "3s")
	c, err := configuration(nil)
	if err != nil || c.uri != "neo4j+s://example.com" || c.username != "environment-user" || c.password != "environment-password" || c.database != "environment-db" || c.timeout != 3*time.Second {
		t.Fatalf("environment configuration not applied: %v", err)
	}
	c, err = configuration(map[string]string{"uri": "bolt://localhost:17687", "username": "option-user", "password": "option-password", "database": "option-db", "timeout": "1s"})
	if err != nil || c.uri != "bolt://localhost:17687" || c.username != "option-user" || c.password != "option-password" || c.database != "option-db" || c.timeout != time.Second {
		t.Fatalf("options must override the environment: %v", err)
	}
	for _, options := range []map[string]string{
		{"password": ""}, {"username": ""}, {"database": ""}, {"timeout": "0s"}, {"timeout": "invalid"},
		{"uri": "http://localhost:7474"}, {"uri": "bolt://"}, {"uri": "bolt://localhost/database"},
		{"uri": "bolt://user:private-password@localhost:7687"}, {"uri": "bolt://user:private-password@%"},
	} {
		_, err := configuration(options)
		if err == nil {
			t.Fatal("invalid configuration was accepted")
		}
		if strings.Contains(err.Error(), "private-password") || strings.Contains(err.Error(), "environment-password") {
			t.Fatal("configuration error exposed credentials")
		}
	}
}

func TestOpenCypherConfiguration(t *testing.T) {
	t.Setenv("NEO4J_DIALECT", "opencypher")
	t.Setenv("NEO4J_URI", "bolt://localhost:7687")
	t.Setenv("NEO4J_USERNAME", "test")
	t.Setenv("NEO4J_PASSWORD", "test-password")
	t.Setenv("NEO4J_DATABASE", "")
	t.Setenv("NEO4J_TIMEOUT", "5s")
	c, err := configuration(nil)
	if err != nil || c.dialect != dialectOpenCypher || c.database != "" {
		t.Fatalf("openCypher must permit the default database: %+v", err)
	}
	for _, uri := range []string{"bolt://localhost:7687", "bolt+s://example.com:7687", "bolt+ssc://example.com:7687"} {
		if _, err := configuration(map[string]string{"uri": uri}); err != nil {
			t.Fatal(err)
		}
	}
	for _, uri := range []string{"neo4j://localhost:7687", "neo4j+s://example.com:7687"} {
		if _, err := configuration(map[string]string{"uri": uri}); err == nil {
			t.Fatal("openCypher accepted a routing URI")
		}
	}
	if _, err := configuration(map[string]string{"dialect": "unknown"}); err == nil {
		t.Fatal("unknown dialect accepted")
	}
	c, err = configuration(map[string]string{"dialect": "neo4j", "database": "neo4j"})
	if err != nil || c.dialect != dialectNeo4j {
		t.Fatalf("dialect option did not override environment: %v", err)
	}
}

func TestNeo4jRegistryRequiresConfiguration(t *testing.T) {
	// An explicit empty option must override any locally configured credential,
	// so the offline test cannot connect to a user's database.
	_, err := graphstore.NewProvider(graphstore.ProviderConfig{
		Type: graphstore.ProviderTypeNeo4j, Options: map[string]string{"password": "", "dialect": "neo4j", "uri": "bolt://localhost:7687"},
	})
	if err == nil || !strings.Contains(err.Error(), "NEO4J_PASSWORD") {
		t.Fatalf("expected registered provider configuration error, got %v", err)
	}
}
