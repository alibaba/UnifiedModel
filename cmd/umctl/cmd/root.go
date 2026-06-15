package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/client"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/config"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/output"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var (
	flagAddr    string
	flagOutput  string
	flagProfile string

	cfg       *config.Config
	apiClient *client.Client
)

var rootCmd = &cobra.Command{
	Use:           "umctl",
	Short:         "UModel CLI — manage models, entities, topology, and queries",
	Long:          "umctl is the command-line interface for UModel, an open-source object graph semantic layer.\n\nDomain reads go through the unified query service (umctl query run/explain).",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading if apiClient already set (test mode)
		if apiClient != nil {
			if flagOutput != "" {
				output.SetFormat(flagOutput)
			}
			return nil
		}

		var err error
		cfg, err = config.LoadConfig()
		if err != nil {
			response.ExitWithError(response.ExitParam, fmt.Sprintf("Failed to load config: %v", err), "Check ~/.umctl/config.yaml syntax.")
		}

		outputFmt := flagOutput
		if outputFmt == "" {
			outputFmt = cfg.OutputFormat
		}
		output.SetFormat(outputFmt)

		if flagProfile != "" {
			cfg.Current = flagProfile
		}

		addr := cfg.ResolveAddr(flagAddr, os.Getenv("UMCTL_ADDR"))
		if strings.TrimSpace(addr) == "" {
			response.ExitWithError(response.ExitParam,
				"No UModel server address configured.",
				"Run 'umctl configure' to set a server address, or pass '--addr http://localhost:8080'.")
		}
		apiClient = client.NewClient(addr)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAddr, "addr", "", "UModel server address (default from config or http://localhost:8080)")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Output format: json (default) or text")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "Configuration profile to use (default from config)")

	rootCmd.AddGroup(&cobra.Group{ID: "service", Title: "Service Commands:"})
	rootCmd.AddGroup(&cobra.Group{ID: "config", Title: "Configuration Commands:"})

	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(umodelCmd)
	rootCmd.AddCommand(entityCmd)
	rootCmd.AddCommand(topoCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(sampleCmd)

	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(metaCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
