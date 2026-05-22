package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/internal/graphstore"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dataRoot := flag.String("data", "data", "UModel data root")
	provider := flag.String("graphstore", graphstore.DefaultProviderType, "GraphStore provider: local.ladybug, memory, or file.memory")
	uiDir := flag.String("ui-dir", "", "Optional directory containing built UModel web UI assets")
	quickStart := flag.Bool("quickstart", false, "Create a demo workspace and import bundled quickstart data before serving")
	quickStartWorkspace := flag.String("quickstart-workspace", bootstrap.DefaultQuickStartWorkspaceID, "Workspace id used by --quickstart")
	quickStartSample := flag.String("quickstart-sample", bootstrap.DefaultQuickStartSample, "Sample package imported by --quickstart")
	flag.Parse()

	graphstoreExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "graphstore" {
			graphstoreExplicit = true
		}
	})
	*provider = resolveProviderForQuickStart(*provider, *quickStart, graphstoreExplicit)

	ctx := context.Background()
	app, err := bootstrap.NewAppWithGraphStore(*dataRoot, graphstore.ProviderConfig{Type: *provider, DataRoot: *dataRoot})
	if err != nil {
		log.Fatal(err)
	}
	if health, err := app.GraphStore.Health(ctx); err != nil {
		log.Fatal(err)
	} else if health.Status == "unavailable" {
		log.Fatalf("graphstore provider %s unavailable: %s", health.Provider, health.Message)
	}
	if *quickStart {
		result, err := app.LoadQuickStart(ctx, bootstrap.QuickStartOptions{
			WorkspaceID: *quickStartWorkspace,
			Sample:      *quickStartSample,
		})
		if err != nil {
			log.Fatalf("quickstart import failed: %v", err)
		}
		log.Printf("quickstart loaded workspace=%s sample=%s umodel_imported=%d umodel_skipped=%d entities=%d relations=%d",
			result.Workspace,
			result.Sample,
			result.UModel.Imported,
			result.UModel.Skipped,
			result.EntityCount,
			result.RelationCount,
		)
	}
	log.Printf("umodel-server listening on %s", *addr)
	if *uiDir != "" {
		log.Printf("serving web UI from %s", *uiDir)
	}
	if err := http.ListenAndServe(*addr, app.HandlerWithUI(*uiDir)); err != nil {
		log.Fatal(err)
	}
}

func resolveProviderForQuickStart(provider string, quickStart bool, graphstoreExplicit bool) string {
	if quickStart && !graphstoreExplicit {
		return graphstore.ProviderTypeMemory
	}
	return provider
}
