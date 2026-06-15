package hint

import (
	"encoding/json"
	"fmt"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

type ErrorContext struct {
	Command    string
	SubCommand string
	Operation  string
	Params     map[string]string
}

func ClassifyHTTPStatus(statusCode int) (exitCode int, errType string) {
	switch {
	case statusCode == 400:
		return response.ExitParam, "invalid_parameter"
	case statusCode == 401 || statusCode == 403:
		return response.ExitAuth, "authentication"
	case statusCode == 404:
		return response.ExitNotFound, "not_found"
	case statusCode == 429:
		return response.ExitQuota, "quota_exceeded"
	case statusCode >= 500:
		return response.ExitServer, "server_error"
	default:
		return response.ExitServer, "unexpected"
	}
}

func suggestForType(errType string, ctx ErrorContext) string {
	switch errType {
	case "invalid_parameter":
		return fmt.Sprintf("Check the parameters for '%s %s'. Use --help to see valid options.", ctx.Command, ctx.SubCommand)
	case "authentication":
		return "Server returned 401/403. Authentication may be required in a future version."
	case "not_found":
		return fmt.Sprintf("The requested resource was not found. Use 'umctl query run' to list available resources.")
	case "quota_exceeded":
		return "Request rate limited. Retry after a brief pause."
	case "server_error":
		return "The server encountered an internal error. Check server logs or retry."
	default:
		return "An unexpected error occurred. Check the server status with 'umctl health'."
	}
}

func HandleHTTPError(statusCode int, body []byte, ctx ErrorContext) {
	exitCode, errType := ClassifyHTTPStatus(statusCode)

	var serverMsg string
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err == nil {
		if msg, ok := parsed["error"].(string); ok {
			serverMsg = msg
		} else if msg, ok := parsed["message"].(string); ok {
			serverMsg = msg
		}
	}
	if serverMsg == "" {
		serverMsg = fmt.Sprintf("HTTP %d: %s", statusCode, string(body))
	}

	response.ExitWithError(exitCode, serverMsg, suggestForType(errType, ctx))
}

func HandleConnectionError(err error, addr string) {
	response.ExitWithError(response.ExitServer,
		fmt.Sprintf("Cannot connect to server at %s: %v", addr, err),
		"Ensure the UModel server is running. Start with: umodel-server --quickstart")
}

func HandleInputError(err error, source string) {
	response.ExitWithError(response.ExitInputError,
		fmt.Sprintf("Failed to read input from %s: %v", source, err),
		"Ensure the file exists and contains valid JSON.")
}
