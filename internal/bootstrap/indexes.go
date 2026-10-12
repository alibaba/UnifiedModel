package bootstrap

import (
	"context"

	"github.com/alibaba/UnifiedModel/pkg/model"
)

// Search and schema indexes are derived state. Restore them at startup without
// replaying entity writes, which would change Create and lifecycle semantics.
func restoreDerivedIndexes(ctx context.Context, app *App) error {
	req := model.WorkspaceListRequest{PageRequest: model.PageRequest{Limit: 100}}
	for {
		page, err := app.Workspace.ListWorkspaces(ctx, req)
		if err != nil {
			return err
		}
		for _, workspace := range page.Items {
			if err := app.UModel.RebuildIndex(ctx, workspace.ID); err != nil {
				return err
			}
			// An internal full scan rebuilds a fresh index, so public query limits
			// must not truncate a workspace's persisted entities.
			rows, err := app.GraphStore.QueryEntities(ctx, model.EntityQueryPlan{Workspace: workspace.ID, Limit: int(^uint(0) >> 1)})
			if err != nil {
				return err
			}
			entities := make([]model.EntityPayload, 0, len(rows.Rows))
			for _, row := range rows.Rows {
				entities = append(entities, model.EntityPayload(row))
			}
			if err := app.EntityStore.RestoreSearchIndex(ctx, workspace.ID, entities); err != nil {
				return err
			}
		}
		if page.NextToken == "" {
			return nil
		}
		req.PageToken = page.NextToken
	}
}
