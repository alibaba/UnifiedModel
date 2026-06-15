package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

func doRequest(method, path string, payload any, ctx hint.ErrorContext) {
	body, statusCode, err := apiClient.DoJSON(method, path, payload)
	if err != nil {
		hint.HandleConnectionError(err, apiClient.Addr)
	}
	if statusCode >= 400 {
		hint.HandleHTTPError(statusCode, body, ctx)
	}
	exitWithResponse(body)
}

func doRawRequest(method, path string, rawBody []byte, ctx hint.ErrorContext) {
	body, statusCode, err := apiClient.DoRaw(method, path, rawBody)
	if err != nil {
		hint.HandleConnectionError(err, apiClient.Addr)
	}
	if statusCode >= 400 {
		hint.HandleHTTPError(statusCode, body, ctx)
	}
	exitWithResponse(body)
}

func exitWithResponse(body []byte) {
	if responseHasFailures(body) {
		response.ExitWithCode(response.ExitOperation, json.RawMessage(body))
	}
	response.ExitWithSuccess(json.RawMessage(body))
}

func responseHasFailures(body []byte) bool {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	failed, ok := payload["failed"]
	if !ok {
		return false
	}
	switch v := failed.(type) {
	case float64:
		return v > 0
	case int:
		return v > 0
	case json.Number:
		n, err := v.Int64()
		return err == nil && n > 0
	default:
		return false
	}
}

func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func readRawFile(path string) ([]byte, error) {
	trimmed := strings.TrimSpace(path)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return []byte(path), nil
	}
	return os.ReadFile(path)
}

func parseJSONObject(s string) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func wrapPayload(payload []byte, field string) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, errEmpty
	}

	if trimmed[0] == '{' {
		var object map[string]any
		if err := json.Unmarshal(trimmed, &object); err != nil {
			return nil, err
		}
		if _, ok := object[field]; ok {
			return trimmed, nil
		}
		return marshalJSONBytes(map[string]any{field: []any{object}})
	}

	if trimmed[0] == '[' {
		var array []any
		if err := json.Unmarshal(trimmed, &array); err != nil {
			return nil, err
		}
		return marshalJSONBytes(map[string]any{field: array})
	}

	return nil, errEmpty
}

func marshalJSONBytes(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// resolveWorkspace returns workspace from -w flag or first positional arg.
// It shifts remaining args if workspace was taken from positional.
func resolveWorkspace(cmd *cobra.Command, args []string) (string, []string) {
	ws, _ := cmd.Flags().GetString("workspace")
	if ws != "" {
		return ws, args
	}
	if len(args) > 0 {
		return args[0], args[1:]
	}
	return "", args
}

var errEmpty = &emptyError{}

type emptyError struct{}

func (e *emptyError) Error() string { return "empty JSON payload" }
