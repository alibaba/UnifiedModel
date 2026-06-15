package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

var (
	globalFormat   = "json"
	mu             sync.Mutex
	textFormatters = map[string]func(any) string{}
	activeTextKey  string
)

func SetFormat(f string) {
	mu.Lock()
	defer mu.Unlock()
	switch f {
	case "text", "json":
		globalFormat = f
	default:
		globalFormat = "json"
	}
}

func GetFormat() string {
	mu.Lock()
	defer mu.Unlock()
	return globalFormat
}

func RegisterTextFormatter(key string, fn func(any) string) {
	mu.Lock()
	defer mu.Unlock()
	textFormatters[key] = fn
}

func SetActiveTextKey(key string) {
	mu.Lock()
	defer mu.Unlock()
	activeTextKey = key
}

func Print(data any) {
	fmt.Fprintln(os.Stdout, Format(data))
}

func Format(data any) string {
	mu.Lock()
	f := globalFormat
	key := activeTextKey
	fn := textFormatters[key]
	mu.Unlock()

	if f == "text" && fn != nil {
		return fn(data)
	}
	if f == "text" {
		return formatText(data)
	}
	return formatJSON(data)
}

func formatJSON(data any) string {
	if raw, ok := data.(json.RawMessage); ok {
		return string(raw)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(b)
}

func formatText(data any) string {
	parsed := data
	if raw, ok := data.(json.RawMessage); ok {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&parsed); err != nil {
			return string(raw)
		}
	}
	return renderText(parsed)
}

func renderText(data any) string {
	switch v := data.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s: %s", key, renderScalar(v[key])))
		}
		return strings.Join(lines, "\n")
	case []any:
		lines := make([]string, 0, len(v))
		for _, item := range v {
			lines = append(lines, "- "+renderScalar(item))
		}
		return strings.Join(lines, "\n")
	case string:
		return v
	default:
		return renderScalar(v)
	}
}

func renderScalar(data any) string {
	switch v := data.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case json.Number:
		return v.String()
	case float64:
		return fmt.Sprintf("%v", v)
	case float32:
		return fmt.Sprintf("%v", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
