import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Navigate, Route, Routes, useNavigate, useParams } from 'react-router'
import { GitBranch, Layers, Network, PanelLeft, Settings2, TerminalSquare, UploadCloud } from 'lucide-react'
import { UModelApi } from './api/client'
import type { HealthResponse, WorkspaceMetadata } from './api/types'
import { Button, Badge, StatusDot, Field, TextInput } from './design/components'
import { useI18n } from './i18n'
import { scheduleMonacoPreload } from './lib/preloadMonaco'
import { useLocalStorageState } from './lib/storage'
import { WorkspaceLanding } from './features/workspaces/WorkspaceLanding'
import { WorkspaceShell } from './features/workspace/WorkspaceShell'
import { defaultWorkspaceView, workspacePath, workspaceViewFromSegment, type WorkspaceView } from './routes'

const storageKeys = {
  apiBase: 'openumodel.apiBase',
  workspace: 'openumodel.workspace',
}

export function App() {
  const { t } = useI18n()
  const [apiBase, setApiBase] = useLocalStorageState(storageKeys.apiBase, '')
  const [, setLastWorkspace] = useLocalStorageState<string | null>(storageKeys.workspace, null)
  const [health, setHealth] = useState<HealthResponse | null>(null)
  const [workspace, setWorkspace] = useState<WorkspaceMetadata | null>(null)

  const api = useMemo(() => new UModelApi(apiBase), [apiBase])
  const navItems = useMemo(
    () => [
      { value: 'umodel' as const, label: t('nav.umodel'), icon: <GitBranch size={16} /> },
      { value: 'entityTopo' as const, label: t('nav.entityTopo'), icon: <Network size={16} /> },
      { value: 'query' as const, label: t('nav.query'), icon: <TerminalSquare size={16} /> },
      { value: 'imports' as const, label: t('nav.imports'), icon: <UploadCloud size={16} /> },
      { value: 'settings' as const, label: t('nav.settings'), icon: <Settings2 size={16} /> },
      { value: 'apiDebug' as const, label: t('nav.apiMap'), icon: <Layers size={16} /> },
    ],
    [t],
  )

  useEffect(() => scheduleMonacoPreload(), [])

  return (
    <Routes>
      <Route
        path="/"
        element={
          <LandingRoute
            api={api}
            apiBase={apiBase}
            onApiBaseChange={setApiBase}
            health={health}
            onHealthChange={setHealth}
            onWorkspaceOpen={(nextWorkspace) => {
              setLastWorkspace(nextWorkspace.id)
              setWorkspace(nextWorkspace)
            }}
          />
        }
      />
      <Route path="/workspaces/:workspaceId" element={<WorkspaceDefaultRedirect />} />
      <Route
        path="/workspaces/:workspaceId/:viewSegment"
        element={
          <WorkspaceRoute
            api={api}
            workspace={workspace}
            health={health}
            navItems={navItems}
            onWorkspaceChange={setWorkspace}
            onHealthChange={setHealth}
            onWorkspaceOpen={setLastWorkspace}
          />
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

function LandingRoute({
  api,
  apiBase,
  onApiBaseChange,
  health,
  onHealthChange,
  onWorkspaceOpen,
}: {
  api: UModelApi
  apiBase: string
  onApiBaseChange: (value: string) => void
  health: HealthResponse | null
  onHealthChange: (health: HealthResponse | null) => void
  onWorkspaceOpen: (workspace: WorkspaceMetadata) => void
}) {
  const navigate = useNavigate()

  return (
    <WorkspaceLanding
      api={api}
      apiBase={apiBase}
      onApiBaseChange={onApiBaseChange}
      health={health}
      onHealthChange={onHealthChange}
      onOpenWorkspace={(nextWorkspace) => {
        onWorkspaceOpen(nextWorkspace)
        navigate(workspacePath(nextWorkspace.id))
      }}
    />
  )
}

function WorkspaceDefaultRedirect() {
  const { workspaceId } = useParams()
  return <Navigate to={workspaceId ? workspacePath(workspaceId, defaultWorkspaceView) : '/'} replace />
}

function WorkspaceRoute({
  api,
  workspace,
  health,
  navItems,
  onWorkspaceChange,
  onHealthChange,
  onWorkspaceOpen,
}: {
  api: UModelApi
  workspace: WorkspaceMetadata | null
  health: HealthResponse | null
  navItems: Array<{ value: WorkspaceView; label: string; icon: ReactNode }>
  onWorkspaceChange: (workspace: WorkspaceMetadata | null) => void
  onHealthChange: (health: HealthResponse | null) => void
  onWorkspaceOpen: (workspaceId: string | null) => void
}) {
  const navigate = useNavigate()
  const { workspaceId, viewSegment } = useParams()
  const view = workspaceViewFromSegment(viewSegment)
  const activeWorkspace = workspace?.id === workspaceId ? workspace : null

  useEffect(() => {
    if (workspaceId) onWorkspaceOpen(workspaceId)
  }, [onWorkspaceOpen, workspaceId])

  if (!workspaceId) return <Navigate to="/" replace />
  if (!view) return <Navigate to={workspacePath(workspaceId)} replace />

  return (
    <WorkspaceShell
      api={api}
      workspaceId={workspaceId}
      workspace={activeWorkspace}
      health={health}
      view={view}
      navItems={navItems}
      onViewChange={(nextView) => navigate(workspacePath(workspaceId, nextView))}
      onWorkspaceChange={onWorkspaceChange}
      onHealthChange={onHealthChange}
      onBack={() => {
        onWorkspaceOpen(null)
        onWorkspaceChange(null)
        navigate('/')
      }}
    />
  )
}

export function HealthBadge({ health }: { health: HealthResponse | null }) {
  const { t } = useI18n()
  if (!health) {
    return (
      <Badge>
        <StatusDot />
        {t('common.health.unknown')}
      </Badge>
    )
  }
  const ok = health.status === 'ok' && health.graphstore.status === 'ok'
  return (
    <Badge tone={ok ? 'success' : 'warning'}>
      <StatusDot status={ok ? 'ok' : 'warn'} />
      {health.graphstore.provider}
    </Badge>
  )
}

export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className={compact ? 'brand brand-compact' : 'brand'} aria-label="UModel">
      <div className="brand-mark">
        <img
          className="brand-logo-image"
          src={compact ? '/openumodel-mark.svg' : '/openumodel-logo.svg'}
          alt=""
          draggable={false}
        />
      </div>
    </div>
  )
}

export function ApiBaseField({
  apiBase,
  onApiBaseChange,
}: {
  apiBase: string
  onApiBaseChange: (value: string) => void
}) {
  const { t } = useI18n()
  return (
    <Field label={t('landing.api.endpoint')}>
      <TextInput
        value={apiBase}
        onChange={(event) => onApiBaseChange(event.target.value)}
        placeholder={t('landing.api.placeholder')}
      />
    </Field>
  )
}

export function SmallReloadButton({ onClick }: { onClick: () => void }) {
  const { t } = useI18n()
  return (
    <Button variant="ghost" onClick={onClick}>
      <PanelLeft size={15} />
      {t('common.refresh')}
    </Button>
  )
}
