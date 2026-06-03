export type WorkspaceView = 'umodel' | 'entityTopo' | 'query' | 'imports' | 'settings' | 'apiDebug'

export const defaultWorkspaceView: WorkspaceView = 'umodel'

export const workspaceViewSegments = {
  umodel: 'umodel',
  entityTopo: 'entity-topo',
  query: 'query',
  imports: 'imports',
  settings: 'settings',
  apiDebug: 'api-debug',
} as const satisfies Record<WorkspaceView, string>

export function workspacePath(workspaceId: string, view: WorkspaceView = defaultWorkspaceView) {
  return `/workspaces/${encodeURIComponent(workspaceId)}/${workspaceViewSegments[view]}`
}

export function workspaceViewFromSegment(segment: string | undefined): WorkspaceView | null {
  if (!segment) return null
  const entry = Object.entries(workspaceViewSegments).find(([, value]) => value === segment)
  return entry ? (entry[0] as WorkspaceView) : null
}
