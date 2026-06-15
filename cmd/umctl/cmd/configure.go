package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/config"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/registry"
	"github.com/alibaba/UnifiedModel/cmd/umctl/pkg/response"
)

var configureCmd = &cobra.Command{
	Use:     "configure",
	Short:   "Configure umctl profiles",
	Long:    "Interactive wizard to set up a configuration profile. Currently configures the server address.",
	GroupID: "config",
	Run:     runConfigure,
}

var configureListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured profiles",
	Run:   runConfigureList,
}

var configureShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current profile details",
	Run:   runConfigureShow,
}

func init() {
	configureCmd.AddCommand(configureListCmd)
	configureCmd.AddCommand(configureShowCmd)

	registry.Global.Register(registry.CommandMeta{
		Group: "config", Resource: "configure", Action: "setup",
		Description: "Interactive wizard to configure a umctl profile",
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "config", Resource: "configure", Action: "list",
		Description: "List all configured profiles",
	})
	registry.Global.Register(registry.CommandMeta{
		Group: "config", Resource: "configure", Action: "show",
		Description: "Show current profile details",
	})
}

func runConfigure(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		response.ExitWithError(response.ExitParam, fmt.Sprintf("Failed to load config: %v", err), "Check ~/.umctl/config.yaml")
	}

	profileName := flagProfile
	if profileName == "" {
		profileName = cfg.Current
	}
	if profileName == "" {
		profileName = "default"
	}

	profile := cfg.Profiles[profileName]

	fmt.Fprintf(os.Stderr, "Configuring profile: %s\n\n", profileName)

	scanner := bufio.NewScanner(os.Stdin)

	profile.Addr = prompt(scanner, "Server Address", profile.Addr)

	cfg.OutputFormat = prompt(scanner, "Default Output Format (json/text)", cfg.OutputFormat)
	if cfg.OutputFormat != "json" && cfg.OutputFormat != "text" {
		cfg.OutputFormat = "json"
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]config.Profile)
	}
	cfg.Profiles[profileName] = profile
	cfg.Current = profileName

	if err := config.SaveConfig(cfg); err != nil {
		response.ExitWithError(response.ExitServer, fmt.Sprintf("Failed to save config: %v", err), "Check permissions on ~/.umctl/")
	}

	fmt.Fprintf(os.Stderr, "\nConfiguration saved to %s\n", config.ConfigPath())
}

func runConfigureList(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		response.ExitWithError(response.ExitParam, fmt.Sprintf("Failed to load config: %v", err), "Check ~/.umctl/config.yaml")
	}

	for name, profile := range cfg.Profiles {
		marker := "  "
		if name == cfg.Current {
			marker = "* "
		}
		fmt.Fprintf(os.Stdout, "%s%-16s addr=%s\n", marker, name, profile.Addr)
	}
}

func runConfigureShow(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		response.ExitWithError(response.ExitParam, fmt.Sprintf("Failed to load config: %v", err), "Check ~/.umctl/config.yaml")
	}

	profileName := flagProfile
	if profileName == "" {
		profileName = cfg.Current
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		response.ExitWithError(response.ExitNotFound, fmt.Sprintf("Profile %q not found", profileName), "Use 'umctl configure list' to see available profiles.")
	}

	fmt.Fprintf(os.Stdout, "Profile:       %s\n", profileName)
	fmt.Fprintf(os.Stdout, "Server Addr:   %s\n", profile.Addr)
	fmt.Fprintf(os.Stdout, "Output Format: %s\n", cfg.OutputFormat)
}

func prompt(scanner *bufio.Scanner, label, current string) string {
	if current != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, current)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val != "" {
			return val
		}
	}
	return current
}
