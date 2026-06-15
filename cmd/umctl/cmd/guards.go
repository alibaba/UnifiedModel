package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var entityForbidden = map[string]string{
	"get":    "read entities through query run/explain",
	"list":   "read entities through query run/explain",
	"search": "read entities through query run/explain",
}

var topoForbidden = map[string]string{
	"neighbors": "read topology through query run/explain",
	"subgraph":  "read topology through query run/explain",
	"path":      "read topology through query run/explain",
}

var umodelForbidden = map[string]string{
	"get":   "read UModel data through query run/explain",
	"list":  "read UModel data through query run/explain",
	"graph": "read UModel data through query run/explain",
}

var workspaceForbidden = map[string]string{
	"start":   "workspace is metadata only",
	"stop":    "workspace is metadata only",
	"backup":  "workspace is metadata only",
	"restore": "workspace is metadata only",
	"export":  "workspace is metadata only",
	"import":  "workspace is metadata only",
}

func groupRunE(forbidden map[string]string, group string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			if reason, ok := forbidden[args[0]]; ok {
				return fmt.Errorf("%s %s is forbidden; %s", group, args[0], reason)
			}
		}
		return cmd.Help()
	}
}

func init() {
	entityCmd.RunE = groupRunE(entityForbidden, "entity")
	topoCmd.RunE = groupRunE(topoForbidden, "topo")
	umodelCmd.RunE = groupRunE(umodelForbidden, "umodel")
	workspaceCmd.RunE = groupRunE(workspaceForbidden, "workspace")
}
