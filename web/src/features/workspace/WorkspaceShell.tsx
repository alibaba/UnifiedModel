import { Suspense, lazy, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { ArrowLeft, PanelLeftClose, PanelLeftOpen, RefreshCcw } from 'lucide-react'
import type { HealthResponse, WorkspaceMetadata } from '../../api/types'
import { UModelApi } from '../../api/client'
import { Brand, HealthBadge } from '../../App'
import { Button, Badge, IconButton } from '../../design/components'
import { useI18n, type MessageKey } from '../../i18n'
import { formatError } from '../../lib/json'
import type { WorkspaceView } from '../../routes'

const UModelPage = lazy(() => import('../umodel/UModelPage').then(({ UModelPage }) => ({ default: UModelPage })))
const EntityTopoPage = lazy(() => import('../entityTopo/EntityTopoPage').then(({ EntityTopoPage }) => ({ default: EntityTopoPage })))
const QueryPage = lazy(() => import('../query/QueryPage').then(({ QueryPage }) => ({ default: QueryPage })))
const ImportsPage = lazy(() => import('../imports/ImportsPage').then(({ ImportsPage }) => ({ default: ImportsPage })))
const SettingsPage = lazy(() => import('../settings/SettingsPage').then(({ SettingsPage }) => ({ default: SettingsPage })))
const ApiMapPage = lazy(() => import('../settings/ApiMapPage').then(({ ApiMapPage }) => ({ default: ApiMapPage })))

interface NavItem {
  value: WorkspaceView
  label: string
  icon: ReactNode
}

export function WorkspaceShell({
  api,
  workspaceId,
  workspace,
  health,
  view,
  navItems,
  onViewChange,
  onWorkspaceChange,
  onHealthChange,
  onBack,
}: {
  api: UModelApi
  workspaceId: string
  workspace: WorkspaceMetadata | null
  health: HealthResponse | null
  view: WorkspaceView
  navItems: NavItem[]
  onViewChange: (view: WorkspaceView) => void
  onWorkspaceChange: (workspace: WorkspaceMetadata | null) => void
  onHealthChange: (health: HealthResponse | null) => void
  onBack: () => void
}) {
  const { t } = useI18n()
  const [error, setError] = useState('')
  const [refreshToken, setRefreshToken] = useState(0)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)

  const refresh = useCallback(async () => {
    setError('')
    try {
      const [nextHealth, nextWorkspace] = await Promise.all([
        api.health().catch(() => null),
        api.getWorkspace(workspaceId),
      ])
      onHealthChange(nextHealth)
      onWorkspaceChange(nextWorkspace)
      setRefreshToken((value) => value + 1)
    } catch (nextError) {
      setError(formatError(nextError))
    }
  }, [api, onHealthChange, onWorkspaceChange, workspaceId])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const page = useMemo(() => {
    switch (view) {
      case 'umodel':
        return <UModelPage api={api} workspaceId={workspaceId} refreshToken={refreshToken} />
      case 'entityTopo':
        return <EntityTopoPage api={api} workspaceId={workspaceId} refreshToken={refreshToken} />
      case 'query':
        return <QueryPage api={api} workspaceId={workspaceId} />
      case 'imports':
        return <ImportsPage api={api} workspaceId={workspaceId} onChanged={() => setRefreshToken((value) => value + 1)} />
      case 'settings':
        return (
          <SettingsPage
            api={api}
            workspaceId={workspaceId}
            workspace={workspace}
            onWorkspaceChange={onWorkspaceChange}
            onBack={onBack}
          />
        )
      case 'apiDebug':
        return <ApiMapPage api={api} workspaceId={workspaceId} />
      default:
        return null
    }
  }, [api, onBack, onWorkspaceChange, refreshToken, view, workspace, workspaceId])

  const canvasHost = view === 'umodel' || view === 'entityTopo'
  const topbarHidden = canvasHost || view === 'query' || view === 'imports' || view === 'settings' || view === 'apiDebug'

  return (
    <div className={`workspace-shell app-shell ${sidebarCollapsed ? 'collapsed' : ''} ${canvasHost ? 'canvas-host' : ''}`}>
      <aside className="workspace-sidebar">
        <div className="workspace-sidebar-header">
          <Brand compact />
          <div className="workspace-sidebar-title" style={{ minWidth: 0 }}>
            <strong style={{ display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {workspace?.name || workspaceId}
            </strong>
            <span className="workspace-id">{workspaceId}</span>
          </div>
          <IconButton
            className="workspace-collapse-button"
            label={sidebarCollapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')}
            onClick={() => setSidebarCollapsed((value) => !value)}
            type="button"
          >
            {sidebarCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
          </IconButton>
        </div>
        <nav className="workspace-nav">
          {navItems.map((item) => (
            <button
              key={item.value}
              className={view === item.value ? 'active' : ''}
              onClick={() => onViewChange(item.value)}
              type="button"
              title={item.label}
            >
              {item.icon}
              <span className="workspace-nav-label">{item.label}</span>
            </button>
          ))}
        </nav>
        <div className="workspace-sidebar-footer">
          <Button className="workspace-back-button" variant="ghost" onClick={onBack}>
            <ArrowLeft size={16} />
            <span className="workspace-back-label">{t('nav.workspaces')}</span>
          </Button>
        </div>
      </aside>

      <section className={`workspace-main ${topbarHidden ? 'workspace-main-no-topbar' : ''} ${canvasHost ? 'canvas-main-host' : ''}`}>
        {!topbarHidden && (
        <header className="workspace-topbar">
          <div className="row" style={{ minWidth: 0 }}>
            <Badge tone="indigo">{t(viewLabelKey(view))}</Badge>
            <span className="small muted" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {error || workspace?.paths.root || t('settings.metadata.notLoaded')}
            </span>
          </div>
          <div className="row">
            <HealthBadge health={health} />
            <Button variant="ghost" onClick={() => void refresh()}>
              <RefreshCcw size={15} />
              {t('common.refresh')}
            </Button>
          </div>
        </header>
        )}
        <main className={`workspace-content ${canvasHost ? 'workspace-content-canvas' : ''}`}>
          <Suspense fallback={<div className="workspace-page-loading">{t('common.loading')}</div>}>{page}</Suspense>
        </main>
      </section>
    </div>
  )
}

function viewLabelKey(view: WorkspaceView): MessageKey {
  switch (view) {
    case 'umodel':
      return 'nav.umodel'
    case 'entityTopo':
      return 'nav.entityTopo'
    case 'query':
      return 'nav.query'
    case 'imports':
      return 'nav.imports'
    case 'settings':
      return 'nav.settings'
    case 'apiDebug':
      return 'nav.apiMap'
  }
}

export type { WorkspaceView }
