package cmd

import (
	"net/http"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/hint"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
)

var healthCmd = &cobra.Command{
	Use:     "health",
	Short:   "Check UModel server health",
	GroupID: "service",
	Run: func(cmd *cobra.Command, args []string) {
		doRequest(http.MethodGet, "/healthz", nil,
			hint.ErrorContext{Command: "health"})
	},
}

func init() {
	registry.Global.Register(registry.CommandMeta{
		Group: "service", Resource: "health", Action: "check",
		Description: "Check UModel server health status",
	})
}
