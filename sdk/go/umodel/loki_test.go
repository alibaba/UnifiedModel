package umodel

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLokiTLSVerifyFalseRoundTrip(t *testing.T) {
	verify := false
	spec := LokiV100Spec{Endpoint: "https://loki.example.com", TlsVerify: &verify}
	jsonData, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsonData), `"tls_verify":false`) {
		t.Fatalf("explicit false missing from JSON: %s", jsonData)
	}
	var fromJSON LokiV100Spec
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatal(err)
	}
	if fromJSON.TlsVerify == nil || *fromJSON.TlsVerify {
		t.Fatalf("JSON round trip lost explicit false: %#v", fromJSON.TlsVerify)
	}
	yamlData, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(yamlData), "tls_verify: false") {
		t.Fatalf("explicit false missing from YAML: %s", yamlData)
	}
	var fromYAML LokiV100Spec
	if err := yaml.Unmarshal(yamlData, &fromYAML); err != nil {
		t.Fatal(err)
	}
	if fromYAML.TlsVerify == nil || *fromYAML.TlsVerify {
		t.Fatalf("YAML round trip lost explicit false: %#v", fromYAML.TlsVerify)
	}
}
