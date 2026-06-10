package response

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/output"
)

const (
	ExitParam      = 1 // flag missing or invalid value
	ExitAuth       = 2 // reserved: authentication failure
	ExitNotFound   = 3 // resource not found
	ExitQuota      = 4 // reserved: quota exceeded
	ExitServer     = 5 // server error (5xx)
	ExitInputError = 6 // file not found, JSON/YAML parse failure, stdin format error
)

// ExitFunc is the function called to terminate the process.
// Tests can override this to prevent real process exit.
var ExitFunc = os.Exit

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Suggest string `json:"suggest,omitempty"`
}

func ExitWithSuccess(data any) {
	output.Print(data)
	ExitFunc(0)
}

func ExitWithError(code int, errMsg, suggest string) {
	resp := ErrorResponse{
		Success: false,
		Error:   errMsg,
		Suggest: suggest,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintln(os.Stdout, string(b))
	ExitFunc(code)
}
