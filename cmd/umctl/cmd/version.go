package cmd

import (
	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildTime = "unknown"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Show umctl version information",
	GroupID: "config",
	Run: func(cmd *cobra.Command, args []string) {
		response.ExitWithSuccess(map[string]string{
			"version":   version,
			"commit":    gitCommit,
			"buildTime": buildTime,
		})
	},
}
