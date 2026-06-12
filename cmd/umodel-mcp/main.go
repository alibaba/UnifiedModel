package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(args []string, in io.Reader, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("umodel-mcp", flag.ContinueOnError)
	flags.SetOutput(errOut)
	workspace := flags.String("workspace", "demo", "default workspace for MCP requests")
	dataRoot := flags.String("data", ".umodel-data", "local UModel data root")
	provider := flags.String("graphstore", graphstore.DefaultProviderType, "GraphStore provider: local.ladybug, memory, or file.memory")
	quickStart := flags.Bool("quickstart", false, "Create a demo workspace and import bundled quickstart data before serving MCP")
	quickStartWorkspace := flags.String("quickstart-workspace", bootstrap.DefaultQuickStartWorkspaceID, "Workspace id used by --quickstart")
	quickStartSample := flags.String("quickstart-sample", bootstrap.DefaultQuickStartSample, "Sample package imported by --quickstart")
	importRoot := flags.String("import-root", "", "Confine UModel imports to this directory (default: current working directory; use \"/\" to allow any path)")
	manifest := flags.Bool("manifest", false, "print tools/resources manifest and exit")
	transport := flags.String("transport", "stdio", "MCP transport: stdio or http")
	addr := flags.String("addr", "127.0.0.1:8090", "HTTP MCP listen address when --transport=http")
	mcpPath := flags.String("mcp-path", "/mcp", "Streamable HTTP MCP endpoint path")
	if err := flags.Parse(args); err != nil {
		return err
	}

	graphstoreExplicit := false
	workspaceExplicit := false
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "graphstore":
			graphstoreExplicit = true
		case "workspace":
			workspaceExplicit = true
		}
	})
	*provider = resolveProviderForQuickStart(*provider, *quickStart, graphstoreExplicit)
	if *quickStart && !workspaceExplicit {
		*workspace = *quickStartWorkspace
	}

	app, err := bootstrap.NewAppWithGraphStore(*dataRoot, graphstore.ProviderConfig{Type: *provider, DataRoot: *dataRoot}, bootstrap.WithImportRoot(*importRoot))
	if err != nil {
		return err
	}
	ctx := context.Background()
	if health, err := app.GraphStore.Health(ctx); err != nil {
		return err
	} else if health.Status == "unavailable" {
		return fmt.Errorf("graphstore provider %s unavailable: %s", health.Provider, health.Message)
	}
	if *quickStart {
		result, err := app.LoadQuickStart(ctx, bootstrap.QuickStartOptions{
			WorkspaceID: *quickStartWorkspace,
			Sample:      *quickStartSample,
		})
		if err != nil {
			return fmt.Errorf("quickstart import failed: %w", err)
		}
		fmt.Fprintf(errOut, "quickstart loaded workspace=%s sample=%s umodel_imported=%d umodel_skipped=%d entities=%d relations=%d\n",
			result.Workspace,
			result.Sample,
			result.UModel.Imported,
			result.UModel.Skipped,
			result.EntityCount,
			result.RelationCount,
		)
	}
	if *manifest {
		discovery, err := app.AgentGateway.Discover(ctx, *workspace)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(discovery)
	}

	switch strings.ToLower(*transport) {
	case "stdio":
		return runStdio(ctx, app, *workspace, in, out)
	case "http", "streamable-http", "sse":
		return serveHTTP(ctx, app, *workspace, *addr, *mcpPath, errOut)
	default:
		return fmt.Errorf("unsupported MCP transport %q", *transport)
	}
}

func resolveProviderForQuickStart(provider string, quickStart bool, graphstoreExplicit bool) string {
	if quickStart && !graphstoreExplicit {
		return graphstore.ProviderTypeMemory
	}
	return provider
}

func runStdio(ctx context.Context, app *bootstrap.App, defaultWorkspace string, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		resp, shouldWrite := handleRawRPC(ctx, app, defaultWorkspace, []byte(line))
		if shouldWrite {
			writeResponse(out, resp)
		}
	}
	return scanner.Err()
}

func writeResponse(out io.Writer, resp rpcResponse) {
	_ = json.NewEncoder(out).Encode(resp)
}
