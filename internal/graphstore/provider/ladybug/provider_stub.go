//go:build !ladybug

package ladybug

import (
	"context"

	"github.com/alibaba/UnifiedModel/internal/graphstore"
	"github.com/alibaba/UnifiedModel/pkg/contract"
	apperrors "github.com/alibaba/UnifiedModel/pkg/errors"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type Provider struct{}

func NewProvider(config graphstore.ProviderConfig) (*Provider, error) {
	return &Provider{}, nil
}

func init() {
	graphstore.RegisterProvider(graphstore.ProviderTypeLadybug, func(config graphstore.ProviderConfig) (contract.GraphStore, error) {
		return NewProvider(config)
	})
}

func (p *Provider) OpenWorkspace(ctx context.Context, workspace model.WorkspaceMetadata) error {
	return unavailable()
}

func (p *Provider) EnsureSchema(ctx context.Context, workspace string) error {
	return unavailable()
}

func (p *Provider) PutUModelElements(ctx context.Context, batch model.UModelElementBatch) (model.WriteResult, error) {
	return model.WriteResult{}, unavailable()
}

func (p *Provider) DeleteUModelElements(ctx context.Context, workspace string, ids []string) (model.WriteResult, error) {
	return model.WriteResult{}, unavailable()
}

func (p *Provider) GetUModelSnapshot(ctx context.Context, req model.UModelSnapshotRequest) (model.UModelSnapshot, error) {
	return model.UModelSnapshot{}, unavailable()
}

func (p *Provider) WriteEntities(ctx context.Context, batch model.EntityWriteBatch) (model.WriteResult, error) {
	return model.WriteResult{}, unavailable()
}

func (p *Provider) WriteRelations(ctx context.Context, batch model.RelationWriteBatch) (model.WriteResult, error) {
	return model.WriteResult{}, unavailable()
}

func (p *Provider) QueryEntities(ctx context.Context, plan model.EntityQueryPlan) (model.QueryResult, error) {
	return model.QueryResult{}, unavailable()
}

func (p *Provider) QueryTopo(ctx context.Context, plan model.TopoQueryPlan) (model.QueryResult, error) {
	return model.QueryResult{}, unavailable()
}

func (p *Provider) Capabilities(ctx context.Context) (model.GraphStoreCapabilities, error) {
	return ladybugCapabilities(), nil
}

func (p *Provider) Health(ctx context.Context) (model.GraphStoreHealth, error) {
	return model.GraphStoreHealth{Provider: graphstore.ProviderTypeLadybug, Status: "unavailable", Message: "build with -tags ladybug to enable local.ladybug"}, nil
}

func unavailable() error {
	return apperrors.New(apperrors.CodeProviderUnavailable, "local.ladybug provider is disabled in this build")
}

func ladybugCapabilities() model.GraphStoreCapabilities {
	return model.GraphStoreCapabilities{
		EntitySearch:       true,
		GraphMatch:         true,
		GraphCallNeighbors: true,
		ControlledCypher:   true,
		TimeVisibility:     true,
		ServerSideFilter:   false,
		MaxDepth:           10,
		MaxLimit:           1000,
		Timeout:            "60s",
	}
}
