package schemaspec

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

type Registry struct {
	schemas map[string]*Schema
}

func NewRegistry(efs embed.FS, dir string) (*Registry, error) {
	entries, err := efs.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read embedded dir %q: %w", dir, err)
	}
	reg := &Registry{schemas: make(map[string]*Schema)}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := dir + "/" + entry.Name()
		body, err := efs.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		s, err := parseSchemaBytes(body)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if s == nil {
			continue
		}
		if existing, ok := reg.schemas[s.Name]; ok {
			return nil, fmt.Errorf("duplicate schema name %q (in %s and earlier source)", s.Name, path)
		} else {
			_ = existing
		}
		reg.schemas[s.Name] = s
	}
	return reg, nil
}

func newRegistryFromFS(efs fs.ReadDirFS, dir string) (*Registry, error) {
	entries, err := efs.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %q: %w", dir, err)
	}
	reg := &Registry{schemas: make(map[string]*Schema)}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, err := fs.ReadFile(efs, dir+"/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		s, err := parseSchemaBytes(body)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		if s != nil {
			reg.schemas[s.Name] = s
		}
	}
	return reg, nil
}

func (r *Registry) Lookup(kind string) *Schema {
	if r == nil {
		return nil
	}
	return r.schemas[kind]
}

func (r *Registry) Kinds() []string {
	if r == nil {
		return nil
	}
	out := make([]string, 0, len(r.schemas))
	for k := range r.schemas {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var (
	defaultOnce sync.Once
	defaultReg  *Registry
	defaultErr  error
)

func Default() *Registry {
	defaultOnce.Do(func() {
		defaultReg, defaultErr = NewRegistry(embeddedFS, embeddedDir)
	})
	if defaultErr != nil {
		panic(fmt.Errorf("schemaspec.Default: %w", defaultErr))
	}
	return defaultReg
}

func parseSchemaBytes(body []byte) (*Schema, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	return schemaFromRaw(raw)
}

func schemaFromRaw(raw map[string]any) (*Schema, error) {
	name, _ := raw["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("schema missing name")
	}
	versionsRaw, _ := raw["versions"].([]any)
	if len(versionsRaw) == 0 {
		return nil, fmt.Errorf("schema %q has no versions", name)
	}
	v0, _ := versionsRaw[0].(map[string]any)
	if v0 == nil {
		return nil, fmt.Errorf("schema %q versions[0] is malformed", name)
	}
	versionName, _ := v0["name"].(string)
	v0Spec, _ := v0["spec"].(map[string]any)
	if v0Spec == nil {
		return nil, fmt.Errorf("schema %q versions[0] has no spec", name)
	}
	rootProps, _ := v0Spec["properties"].(map[string]any)
	if rootProps == nil {
		return nil, fmt.Errorf("schema %q root spec has no properties", name)
	}
	userSpecRaw, _ := rootProps["spec"].(map[string]any)
	if userSpecRaw == nil {
		return nil, fmt.Errorf("schema %q has no 'spec' property under versions[0].spec.properties", name)
	}
	p, err := parseProperty(userSpecRaw)
	if err != nil {
		return nil, fmt.Errorf("schema %q: %w", name, err)
	}
	return &Schema{
		Name:    name,
		Version: versionName,
		Spec:    p,
	}, nil
}

func parseProperty(raw map[string]any) (Property, error) {
	var p Property
	if v, ok := raw["type"].(string); ok {
		p.Type = v
	}
	if v, ok := raw["release_stage"].(string); ok {
		p.ReleaseStage = v
	}
	if propsRaw, ok := raw["properties"].(map[string]any); ok && propsRaw != nil {
		p.Properties = make(map[string]Property, len(propsRaw))
		for k, v := range propsRaw {
			vm, ok := v.(map[string]any)
			if !ok {
				continue
			}
			sub, err := parseProperty(vm)
			if err != nil {
				return p, fmt.Errorf(".properties.%s: %w", k, err)
			}
			p.Properties[k] = sub
		}
	}
	if cRaw, ok := raw["constraint"].(map[string]any); ok && cRaw != nil {
		c, err := parseConstraint(cRaw)
		if err != nil {
			return p, fmt.Errorf(".constraint: %w", err)
		}
		p.Constraint = &c
	}
	return p, nil
}

func parseConstraint(raw map[string]any) (Constraint, error) {
	var c Constraint
	if v, ok := raw["required"].(bool); ok {
		c.Required = v
	}
	if v, ok := raw["pattern"].(string); ok && v != "" {
		re, err := regexp.Compile(v)
		if err != nil {
			return c, fmt.Errorf("pattern %q: %w", v, err)
		}
		c.Pattern = re
		c.PatternStr = v
	}
	if v, ok := intFromAny(raw["min_len"]); ok {
		c.MinLen = &v
	}
	if v, ok := intFromAny(raw["max_len"]); ok {
		c.MaxLen = &v
	}
	if v, present := raw["default_value"]; present {
		c.DefaultValue = v
	}
	if e, ok := raw["enum"].(map[string]any); ok && e != nil {
		if vals, ok := e["values"].([]any); ok {
			c.Enum = vals
		}
	}
	if arr, ok := raw["array"].(map[string]any); ok && arr != nil {
		var spec ArraySpec
		if item, ok := arr["item"].(map[string]any); ok && item != nil {
			p, err := parseProperty(item)
			if err != nil {
				return c, fmt.Errorf("array.item: %w", err)
			}
			spec.Item = p
		}
		if v, ok := intFromAny(arr["min_size"]); ok {
			spec.MinSize = &v
		}
		if v, ok := intFromAny(arr["max_size"]); ok {
			spec.MaxSize = &v
		}
		c.Array = &spec
	}
	if m, ok := raw["map"].(map[string]any); ok && m != nil {
		var spec MapSpec
		if k, ok := m["key"].(map[string]any); ok && k != nil {
			p, err := parseProperty(k)
			if err != nil {
				return c, fmt.Errorf("map.key: %w", err)
			}
			spec.Key = p
		}
		if vv, ok := m["value"].(map[string]any); ok && vv != nil {
			p, err := parseProperty(vv)
			if err != nil {
				return c, fmt.Errorf("map.value: %w", err)
			}
			spec.Value = p
		}
		if v, ok := intFromAny(m["min_size"]); ok {
			spec.MinSize = &v
		}
		if v, ok := intFromAny(m["max_size"]); ok {
			spec.MaxSize = &v
		}
		c.Map = &spec
	}
	return c, nil
}

func intFromAny(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case uint64:
		return int(x), true
	case float64:
		return int(x), true
	}
	return 0, false
}
