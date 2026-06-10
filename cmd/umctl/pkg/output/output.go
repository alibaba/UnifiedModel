package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

var (
	globalFormat    = "json"
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
