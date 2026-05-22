import { Badge, Panel } from '../../design/components'

const rows = [
  ['Workspace chooser', 'GET /api/v1/workspaces', 'List selectable workspaces.'],
  ['Create workspace', 'POST /api/v1/workspaces', 'Create workspace metadata.'],
  ['Workspace shell', 'GET /api/v1/workspaces/{workspace}', 'Load selected workspace.'],
  ['Workspace settings', 'PUT /api/v1/workspaces/{workspace}', 'Update name, description, labels, config.'],
  ['Workspace delete', 'DELETE /api/v1/workspaces/{workspace}', 'Soft-delete workspace metadata.'],
  ['Explorer graph/table', 'POST /api/v1/query/{workspace}/execute', 'Run .umodel queries.'],
  ['Element edit', 'POST /api/v1/umodel/{workspace}/validate', 'Validate JSON editor payload.'],
  ['Element save', 'POST /api/v1/umodel/{workspace}/elements', 'Write UModel elements.'],
  ['Element delete', 'DELETE /api/v1/umodel/{workspace}/elements', 'Request deletion by IDs.'],
  ['Import by path', 'POST /api/v1/umodel/{workspace}/import', 'Import YAML/JSON from server-readable path.'],
  ['Import sample data', 'POST /api/v1/samples/{workspace}/multi-domain-quickstart:import', 'Import bundled UModel, entities, and topology.'],
  ['Entity write', 'POST /api/v1/entitystore/{workspace}/entities:write', 'Write CMS 2.0 entities.'],
  ['Relation write', 'POST /api/v1/entitystore/{workspace}/relations:write', 'Write CMS 2.0 relations.'],
  ['Expire entities', 'POST /api/v1/entitystore/{workspace}/entities:expire', 'Expire entity IDs.'],
  ['Expire relations', 'POST /api/v1/entitystore/{workspace}/relations:expire', 'Expire relation IDs.'],
  ['Query console', 'POST /api/v1/query/{workspace}/execute', 'Execute .umodel, .entity, or .topo SPL.'],
  ['Query explain', 'POST /api/v1/query/{workspace}/explain', 'Read provider plan and limits.'],
  ['Agent discovery', 'GET /api/v1/agent/{workspace}/discover', 'Read tools, resources, next actions.'],
  ['Agent resources', 'POST /api/v1/agent/{workspace}/resources:read', 'Read safe metadata resources.'],
  ['Agent tools', 'POST /api/v1/agent/{workspace}/tools:execute', 'Execute enabled tools.'],
]

export function ApiMapPage() {
  return (
    <div className="page-grid">
      <Panel
        title={<strong>Frontend API Map</strong>}
        action={<Badge tone="indigo">{rows.length} bindings</Badge>}
      >
        <div className="stack">
          <p className="muted" style={{ marginTop: 0 }}>
            The UI is implemented entirely on public UModel REST contracts. No internal packages, obviz modules, or cloud console APIs are required.
          </p>
          <div style={{ overflow: 'auto' }}>
            <table className="om-table">
              <thead>
                <tr>
                  <th>UI surface</th>
                  <th>API</th>
                  <th>Purpose</th>
                </tr>
              </thead>
              <tbody>
                {rows.map(([surface, api, purpose]) => (
                  <tr key={`${surface}-${api}`}>
                    <td>{surface}</td>
                    <td className="mono small">{api}</td>
                    <td>{purpose}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </Panel>
    </div>
  )
}
