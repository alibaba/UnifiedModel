import React, {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useI18n, type TFunction } from '../../../i18n'
import { CosmosEngine } from './cosmosEngine'
import { CosmosLabels } from './cosmosLabels'
import type {
  TopoData,
  TopoDataUpdateOptions,
  TopoGraphProps,
  TopoGraphRef,
  ITopoGraph,
  LayoutOptions,
  HoverState,
  CosmosVectorIconOverlayItem,
  CosmosNodeActionInfo,
} from './types'
import {
  TOPO_ICON_GLYPH_STROKE_WIDTH,
  TOPO_ICON_RING_WIDTH,
  getSvgIconId,
  isGenericIconClass,
  normalizeTopoIconPreset,
  resolveLucideIconPreset,
  resolveTopoVisualIdentity,
} from './topoVisualIdentity'

/* ═══════════════════════════════════════════════════════════
   Helpers
   ═══════════════════════════════════════════════════════════ */

const emptyData: TopoData = { nodes: [], edges: [] }
const FOCUSED_DETAIL_NODE_LIMIT = 120

function areLayoutOptionsEqual(left: LayoutOptions, right: LayoutOptions): boolean {
  const keys = new Set([...Object.keys(left), ...Object.keys(right)])
  for (const key of keys) {
    const leftValue = (left as any)[key]
    const rightValue = (right as any)[key]
    if (Array.isArray(leftValue) && Array.isArray(rightValue)) {
      if (leftValue.length !== rightValue.length) return false
      for (let i = 0; i < leftValue.length; i += 1) {
        if (leftValue[i] !== rightValue[i]) return false
      }
      continue
    }
    if (leftValue !== rightValue) return false
  }
  return true
}

function buildNeighborSubgraph(data: TopoData, centerId: string): TopoData {
  const ids = new Set<string>([centerId])
  data.edges.forEach(e => {
    if (e.source === centerId) ids.add(e.target)
    if (e.target === centerId) ids.add(e.source)
  })
  return {
    nodes: data.nodes.filter(n => ids.has(n.id)),
    edges: data.edges.filter(e => ids.has(e.source) && ids.has(e.target)),
  }
}

function withFocusedDetailLayout(base: LayoutOptions, renderData: TopoData, isolatedNodeId: string | null): LayoutOptions {
  const forceDetailedView = Boolean(
    base.forceDetailedView ||
    (
      isolatedNodeId &&
      renderData.nodes.length > 0 &&
      renderData.nodes.length <= FOCUSED_DETAIL_NODE_LIMIT
    ),
  )
  return forceDetailedView
    ? { ...base, forceDetailedView, showLabels: true }
    : { ...base, forceDetailedView: false }
}

/* ═══════════════════════════════════════════════════════════
   Styles
   ═══════════════════════════════════════════════════════════ */

const rootStyle: React.CSSProperties = {
  position: 'relative',
  width: '100%',
  height: '100%',
  overflow: 'hidden',
  background: '#fff',
}

const canvasHostStyle: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
}

const isolationBarStyle: React.CSSProperties = {
  position: 'absolute',
  bottom: 16,
  left: '50%',
  transform: 'translateX(-50%)',
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  padding: '6px 14px',
  borderRadius: 24,
  background: 'rgba(255,255,255,0.9)',
  backdropFilter: 'blur(8px)',
  boxShadow: '0 2px 8px rgba(0,0,0,0.08), 0 0 0 1px rgba(0,0,0,0.04)',
  fontSize: 12,
  fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, sans-serif',
  color: '#374151',
  zIndex: 10,
}

const isolationBtnStyle: React.CSSProperties = {
  padding: '4px 12px',
  borderRadius: 16,
  border: '1px solid rgba(0,0,0,0.08)',
  background: '#fff',
  fontSize: 11,
  fontWeight: 500,
  cursor: 'pointer',
  color: '#5b5bd6',
  fontFamily: 'inherit',
  transition: 'background 150ms',
}

const HOVER_PANEL_MARGIN = 12
const HOVER_PANEL_OFFSET = 14

const graphRenderingOverlayStyle: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
  zIndex: 40,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  background: 'rgba(255, 255, 255, 0.62)',
  backdropFilter: 'blur(2px)',
  pointerEvents: 'auto',
}

const graphRenderingPillStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: 8,
  padding: '8px 14px',
  borderRadius: 999,
  border: '1px solid rgba(148, 163, 184, 0.28)',
  background: 'rgba(255, 255, 255, 0.94)',
  color: '#475569',
  fontSize: 12,
  fontWeight: 650,
  fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, sans-serif',
  boxShadow: '0 10px 28px rgba(15, 23, 42, 0.12)',
}

const graphRenderingSpinnerStyle: React.CSSProperties = {
  width: 14,
  height: 14,
  borderRadius: '50%',
  border: '2px solid rgba(79, 70, 229, 0.18)',
  borderTopColor: '#4f46e5',
  animation: 'cosmos-render-spin 0.8s linear infinite',
}

interface HoverPanelInfo {
  kind: 'node' | 'edge'
  nodeId?: string
  x: number
  y: number
  title: string
  color: string
  iconClass?: string
  iconUrl?: string
  iconPreset?: string
  lucideIcon?: string
  ringWidth?: number
  typeLabel?: string
  domain?: string
  rows?: Array<[string, string]>
  source?: HoverEntityInfo
  target?: HoverEntityInfo
}

interface HoverEntityInfo {
  role: 'Source' | 'Target'
  title: string
  typeLabel: string
  domain?: string
  color: string
  iconClass?: string
  iconUrl?: string
  iconPreset?: string
  lucideIcon?: string
  ringWidth?: number
}

function getEntityInfo(engine: CosmosEngine, nodeId: string, role: 'Source' | 'Target', t: TFunction): HoverEntityInfo {
  const node = engine.getNodeById().get(nodeId)
  const raw = engine.getRawNode(nodeId) as any
  const style = raw?.data?.style ?? {}
  const visual = resolveTopoVisualIdentity({
    cluster: node?.cluster || raw?.data?.cluster || raw?.type,
    title: node?.title || raw?.title || raw?.data?.title || raw?.data?.name || nodeId,
    data: raw?.data,
    style,
    fallbackColor: node?.iconFill || node?.color,
  })
  const cluster = node?.cluster || raw?.data?.cluster || raw?.type || ''
  return {
    role,
    title: node?.title || raw?.title || raw?.data?.title || raw?.data?.name || nodeId,
    typeLabel: node?.subTitle || raw?.data?.subTitle || cluster || t('entityTopoExplorer.detail.entity'),
    domain: cluster.split('@')[0] || undefined,
    color: node?.iconFill || visual.iconFill,
    iconClass: node?.iconClass || visual.iconClass,
    iconUrl: node?.iconUrl || visual.iconUrl,
    iconPreset: node?.iconPreset || visual.iconPreset,
    lucideIcon: node?.lucideIcon || visual.lucideIcon,
    ringWidth: node?.ringWidth || visual.ringWidth,
  }
}

function buildHoverPanelInfo(hover: HoverState, engine: CosmosEngine, t: TFunction): HoverPanelInfo | null {
  if (hover.lockedNodeId || hover.lockedLinkIndex !== undefined) return null
  const pos = hover.screenPosition
  if (!pos || hover.hoverProgress <= 0) return null

  if (hover.hoveredNodeId) {
    const node = engine.getNodeById().get(hover.hoveredNodeId)
    const raw = engine.getRawNode(hover.hoveredNodeId)
    if (!node) return null
    const degree = engine.getNeighbors().get(hover.hoveredNodeId)?.size ?? node.degree ?? 0
    const entity = getEntityInfo(engine, hover.hoveredNodeId, 'Source', t)
    return {
      kind: 'node',
      nodeId: hover.hoveredNodeId,
      x: pos[0],
      y: pos[1],
      title: node.title || raw?.title || node.id,
      color: entity.color,
      iconClass: entity.iconClass,
      iconUrl: entity.iconUrl,
      iconPreset: entity.iconPreset,
      lucideIcon: entity.lucideIcon,
      ringWidth: entity.ringWidth,
      typeLabel: entity.typeLabel,
      domain: entity.domain,
      rows: [
        [t('entityTopoExplorer.detail.degree'), String(degree)],
        [t('entityTopoExplorer.detail.id'), raw?.data?.id ? String(raw.data.id) : node.id],
      ],
    }
  }

  if (hover.hoveredLinkIndex !== undefined) {
    const focused = engine.getFocusedEdges(1)[0]
    if (!focused) return null
    return {
      kind: 'edge',
      x: pos[0],
      y: pos[1],
      title: focused.label,
      color: '#4f46e5',
      typeLabel: t('entityTopoExplorer.detail.relationTitle'),
      source: getEntityInfo(engine, focused.sourceId, 'Source', t),
      target: getEntityInfo(engine, focused.targetId, 'Target', t),
    }
  }

  return null
}

function CosmosNodeGlyph({
  color,
  iconClass,
  iconUrl,
  iconPreset,
  lucideIcon,
  ringWidth,
  focusRing = false,
  label,
  domain,
  size = 28,
  glow = true,
}: {
  color: string
  iconClass?: string
  iconUrl?: string
  iconPreset?: string
  lucideIcon?: string
  ringWidth?: number
  focusRing?: boolean
  label: string
  domain?: string
  size?: number
  glow?: boolean
}) {
  const iconId = getSvgIconId(iconClass)
  let effectiveLabel = (label || '?').trim()
  if (domain && effectiveLabel.startsWith(domain + '.') && effectiveLabel.length > domain.length + 1) {
    effectiveLabel = effectiveLabel.slice(domain.length + 1)
  }
  const letter = effectiveLabel.slice(0, 1).toUpperCase()
  const shouldUseSymbol = iconId && !isGenericIconClass(iconClass)
  const innerSize = Math.max(14, size * 0.52)
  const hasSvgSymbol = shouldUseSymbol && typeof document !== 'undefined' && document.getElementById(iconId)
  const strokeWidth = Math.max(1, ringWidth ?? TOPO_ICON_RING_WIDTH)
  const shadow = focusRing
    ? `0 0 0 1px rgba(91, 91, 214, 0.42), 0 0 0 3px rgba(91, 91, 214, 0.07)`
    : glow ? `0 0 0 4px ${color}10` : undefined

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: size,
        height: size,
        border: `${strokeWidth}px solid ${color}`,
        background: '#fff',
        color,
        boxShadow: shadow,
        fontSize: Math.max(10, size * 0.42),
        fontWeight: 600,
        lineHeight: 1,
        flexShrink: 0,
        borderRadius: '50%',
        boxSizing: 'border-box',
      }}
    >
      {iconUrl ? (
        <img src={iconUrl} alt="" draggable={false} style={{ width: innerSize, height: innerSize, display: 'block', objectFit: 'contain' }} />
      ) : hasSvgSymbol ? (
        <svg width={innerSize} height={innerSize} aria-hidden="true" style={{ display: 'block', fill: 'currentColor' }}>
          <use xlinkHref={`#${iconId}`} />
        </svg>
      ) : shouldUseSymbol ? (
        <span className={`iconfont ${iconClass}`} style={{ fontSize: innerSize, lineHeight: 1 }} />
      ) : (
        <CosmosFallbackGlyph label={label} letter={letter} preset={iconPreset} lucideIcon={lucideIcon} size={size} />
      )}
    </span>
  )
}

function CosmosFallbackGlyph({
  label,
  letter,
  preset,
  lucideIcon,
  size,
}: { label: string; letter: string; preset?: string; lucideIcon?: string; size: number }) {
  const value = label.toLowerCase()
  const kind = normalizeTopoIconPreset(preset) || resolveLucideIconPreset(lucideIcon)
  const strokeWidth = TOPO_ICON_GLYPH_STROKE_WIDTH
  const props = {
    width: Math.max(14, size * 0.54),
    height: Math.max(14, size * 0.54),
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    'aria-hidden': true,
  }

  if (kind === 'pod' || value.includes('pod')) {
    return (
      <svg {...props}>
        <path d="M12 3.2 19 7.2v9.6l-7 4-7-4V7.2l7-4Z" />
        <path d="m5 7.4 7 4 7-4M12 11.4v9.2" />
      </svg>
    )
  }
  if (kind === 'service' || value.includes('service') || value.includes('服务')) {
    return (
      <svg {...props}>
        <path d="M12 4v6M6 18l6-8 6 8" />
        <circle cx="12" cy="4" r="2.1" />
        <circle cx="6" cy="18" r="2.1" />
        <circle cx="18" cy="18" r="2.1" />
      </svg>
    )
  }
  if (kind === 'deployment' || kind === 'application' || value.includes('deployment') || value.includes('应用')) {
    return (
      <svg {...props}>
        <rect x="5" y="5" width="14" height="14" rx="2.4" />
        <path d="M9 9h2.8M9 12h6M9 15h4.6" />
      </svg>
    )
  }
  if (kind === 'node' || kind === 'instance' || value.includes('node') || value.includes('instance') || value.includes('ecs') || value.includes('主机') || value.includes('节点')) {
    return (
      <svg {...props}>
        <rect x="5" y="4.8" width="14" height="14.4" rx="2.2" />
        <path d="M8.5 10h7M8.5 14h7" />
      </svg>
    )
  }
  if (kind === 'disk' || kind === 'database' || value.includes('disk') || value.includes('数据库') || value.includes('rds')) {
    return (
      <svg {...props}>
        <ellipse cx="12" cy="6.5" rx="6.4" ry="3.2" />
        <path d="M5.6 6.5v10.6c0 1.8 2.9 3.2 6.4 3.2s6.4-1.4 6.4-3.2V6.5M5.6 12c0 1.8 2.9 3.2 6.4 3.2s6.4-1.4 6.4-3.2" />
      </svg>
    )
  }
  if (kind === 'redis' || value.includes('redis')) {
    return (
      <svg {...props}>
        <path d="m5 8 7-3.8L19 8l-7 3.8L5 8Z" />
        <path d="m5 12.2 7 3.8 7-3.8M5 16.2l7 3.8 7-3.8" />
      </svg>
    )
  }
  if (kind === 'network' || value.includes('vpc') || value.includes('network') || value.includes('网络')) {
    return (
      <svg {...props}>
        <circle cx="12" cy="12" r="8.2" />
        <path d="M3.8 12h16.4M12 3.8c2.3 2.3 3.4 5 3.4 8.2s-1.1 5.9-3.4 8.2M12 3.8c-2.3 2.3-3.4 5-3.4 8.2s1.1 5.9 3.4 8.2" />
      </svg>
    )
  }
  if (kind === 'loadbalancer' || value.includes('load') || value.includes('slb') || value.includes('balancer')) {
    return (
      <svg {...props}>
        <path d="M12 5v5M12 10 6.5 17M12 10l5.5 7" />
        <rect x="9" y="3" width="6" height="4" rx="1.2" />
        <rect x="3.5" y="16" width="6" height="4" rx="1.2" />
        <rect x="14.5" y="16" width="6" height="4" rx="1.2" />
      </svg>
    )
  }
  if (kind === 'endpoint' || value.includes('http') || value.includes('endpoint')) {
    return (
      <svg {...props}>
        <circle cx="12" cy="12" r="7.5" />
        <path d="M8.5 12h7M13 8.5l3.5 3.5-3.5 3.5" />
      </svg>
    )
  }

  return <span style={{ lineHeight: 1 }}>{letter}</span>
}

const vectorIconLayerStyle: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
  pointerEvents: 'none',
  zIndex: 3,
  overflow: 'hidden',
  contain: 'layout paint',
}

function CosmosVectorIconOverlay({
  engine,
  tick,
}: {
  engine: CosmosEngine | null
  tick: number
}) {
  const icons = useMemo<CosmosVectorIconOverlayItem[]>(
    () => engine?.getVectorIconOverlays() ?? [],
    [engine, tick],
  )

  if (!icons.length) return null

  return (
    <div style={vectorIconLayerStyle} aria-hidden="true" data-cosmos-vector-icon-overlay="true">
      {icons.map((icon) => (
        <div
          key={icon.id}
          style={{
            position: 'absolute',
            left: icon.x,
            top: icon.y,
            width: icon.size,
            height: icon.size,
            transform: 'translate3d(-50%, -50%, 0)',
            opacity: icon.opacity,
            transition: 'opacity 120ms ease',
            willChange: 'transform, opacity',
          }}
        >
          <CosmosNodeGlyph
            color={icon.color}
            iconClass={icon.iconClass}
            iconUrl={icon.iconUrl}
            iconPreset={icon.iconPreset}
            lucideIcon={icon.lucideIcon}
            ringWidth={icon.ringWidth}
            focusRing={icon.active}
            label={icon.typeLabel || icon.label}
            domain={icon.domain}
            size={icon.size}
            glow={false}
          />
        </div>
      ))}
    </div>
  )
}

function HoverEntityBlock({ entity }: { entity: HoverEntityInfo }) {
  const { t } = useI18n()
  const roleLabel = entity.role === 'Source' ? t('entityTopoExplorer.detail.sourceEntity') : t('entityTopoExplorer.detail.targetEntity')

  return (
    <div
      style={{
        minWidth: 0,
        maxWidth: '100%',
        boxSizing: 'border-box',
        overflow: 'hidden',
        padding: '8px 9px',
        borderRadius: 8,
        border: '1px solid rgba(148, 163, 184, 0.20)',
        background: 'rgba(248, 250, 252, 0.74)',
      }}
    >
      <div style={{ fontSize: 10, fontWeight: 700, letterSpacing: 0.2, color: '#94a3b8', marginBottom: 6 }}>
        {roleLabel}
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0, maxWidth: '100%', overflow: 'hidden' }}>
        <CosmosNodeGlyph
          color={entity.color}
          iconClass={entity.iconClass}
          iconUrl={entity.iconUrl}
          iconPreset={entity.iconPreset}
          lucideIcon={entity.lucideIcon}
          ringWidth={entity.ringWidth}
          label={entity.typeLabel}
          domain={entity.domain}
          size={26}
        />
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{
            fontSize: 11,
            fontWeight: 700,
            color: entity.color,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {entity.typeLabel}
          </div>
          <div style={{
            marginTop: 2,
            fontSize: 12,
            fontWeight: 650,
            color: '#334155',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {entity.title}
          </div>
        </div>
      </div>
    </div>
  )
}

function CosmosHoverPanel({ info }: { info: HoverPanelInfo | null }) {
  if (!info) return null

  return <CosmosHoverPanelContent info={info} />
}

function CosmosHoverPanelContent({ info }: { info: HoverPanelInfo }) {
  const { t } = useI18n()
  const panelRef = useRef<HTMLDivElement | null>(null)

  const width = info.kind === 'edge' ? 338 : 312
  const preferredLeft =
    info.x > width + HOVER_PANEL_MARGIN + HOVER_PANEL_OFFSET
      ? info.x - width - HOVER_PANEL_OFFSET
      : info.x + HOVER_PANEL_OFFSET
  const preferredTop = info.y + HOVER_PANEL_OFFSET
  const [position, setPosition] = useState({ left: preferredLeft, top: preferredTop })
  const typeLabel = info.typeLabel || (info.kind === 'edge' ? t('entityTopoExplorer.detail.relationTitle') : t('entityTopoExplorer.detail.entity'))

  useLayoutEffect(() => {
    const panel = panelRef.current
    if (!panel) return

    const parent = panel.parentElement
    const parentWidth = parent?.clientWidth || (typeof window !== 'undefined' ? window.innerWidth : width)
    const parentHeight = parent?.clientHeight || (typeof window !== 'undefined' ? window.innerHeight : 0)
    const panelWidth = panel.offsetWidth || width
    const panelHeight = panel.offsetHeight || 0
    const maxLeft = Math.max(HOVER_PANEL_MARGIN, parentWidth - panelWidth - HOVER_PANEL_MARGIN)
    const maxTop = Math.max(HOVER_PANEL_MARGIN, parentHeight - panelHeight - HOVER_PANEL_MARGIN)
    const nextLeft = Math.min(Math.max(HOVER_PANEL_MARGIN, preferredLeft), maxLeft)
    const nextTop = Math.min(Math.max(HOVER_PANEL_MARGIN, preferredTop), maxTop)

    setPosition((current) => {
      if (current.left === nextLeft && current.top === nextTop) return current
      return { left: nextLeft, top: nextTop }
    })
  }, [preferredLeft, preferredTop, width, info])

  return (
    <div
      ref={panelRef}
      style={{
        position: 'absolute',
        left: position.left,
        top: position.top,
        zIndex: 9,
        width,
        maxWidth: `calc(100% - ${HOVER_PANEL_MARGIN * 2}px)`,
        boxSizing: 'border-box',
        padding: 12,
        borderRadius: 10,
        border: '1px solid rgba(148, 163, 184, 0.26)',
        background: 'rgba(255, 255, 255, 0.97)',
        boxShadow: '0 18px 48px rgba(15, 23, 42, 0.14), 0 0 0 3px rgba(79, 70, 229, 0.04)',
        backdropFilter: 'blur(12px)',
        overflow: 'hidden',
        pointerEvents: 'none',
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Inter, Roboto, sans-serif',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: info.kind === 'edge' ? 10 : 9, minWidth: 0 }}>
        {info.kind === 'node' ? (
          <CosmosNodeGlyph
            color={info.color}
            iconClass={info.iconClass}
            iconUrl={info.iconUrl}
            iconPreset={info.iconPreset}
            lucideIcon={info.lucideIcon}
            ringWidth={info.ringWidth}
            label={typeLabel}
            domain={info.domain}
            size={34}
          />
        ) : (
          <span
            style={{
              width: 11,
              height: 11,
              borderRadius: '50%',
              background: info.color,
              boxShadow: `0 0 0 4px ${info.color}14`,
              flexShrink: 0,
            }}
          />
        )}
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{
            fontSize: 13,
            fontWeight: 750,
            color: '#1f2937',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {info.title}
          </div>
          <div style={{
            marginTop: 1,
            fontSize: 11,
            color: info.kind === 'node' ? info.color : '#64748b',
            fontWeight: info.kind === 'node' ? 700 : 500,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {typeLabel}
          </div>
        </div>
      </div>

      {info.kind === 'edge' && info.source && info.target ? (
        <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr)', gap: 7, minWidth: 0, maxWidth: '100%', overflow: 'hidden' }}>
          <HoverEntityBlock entity={info.source} />
          <HoverEntityBlock entity={info.target} />
        </div>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: '64px minmax(0, 1fr)', gap: '5px 8px', fontSize: 11, marginLeft: 44 }}>
          {(info.rows ?? []).map(([key, value]) => (
            <React.Fragment key={key}>
              <span style={{ color: '#94a3b8' }}>{key}</span>
              <span style={{ color: '#334155', fontWeight: 650, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{value}</span>
            </React.Fragment>
          ))}
        </div>
      )}
    </div>
  )
}

function CosmosNodeActionMenu({
  engine,
  tick,
  onIsolate,
}: {
  engine: CosmosEngine | null
  tick: number
  onIsolate: (nodeId: string) => void
}) {
  const { t } = useI18n()
  const [hoveredAction, setHoveredAction] = useState<string | null>(null)
  const info = useMemo<CosmosNodeActionInfo | null>(
    () => engine?.getLockedNodeAction() ?? null,
    [engine, tick],
  )

  if (!info) return null

  const actionSize = Math.max(16, Math.min(22, info.size * 0.42))
  const iconSize = Math.max(10, actionSize * 0.58)
  const radius = Math.max(info.size * 0.62, info.size / 2 + actionSize * 0.65 + 6)
  const actions = [
    {
      key: 'focus',
      label: t('entityTopoExplorer.filter.focus'),
      angle: -90,
      onClick: () => onIsolate(info.nodeId),
      icon: (
        <svg width={iconSize} height={iconSize} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <circle cx="12" cy="12" r="3.2" />
          <path d="M12 2.8v3.4M12 17.8v3.4M2.8 12h3.4M17.8 12h3.4" />
          <circle cx="12" cy="12" r="8.2" />
        </svg>
      ),
    },
  ]

  return (
    <div
      style={{
        position: 'absolute',
        left: info.x,
        top: info.y,
        width: 1,
        height: 1,
        zIndex: 12,
        pointerEvents: 'none',
      }}
    >
      <style>
        {`
          @keyframes cosmos-node-action-pop {
            from {
              opacity: 0;
              transform: translate(-50%, -50%) scale(0.36);
            }
            to {
              opacity: 1;
              transform: translate(calc(-50% + var(--cosmos-action-x)), calc(-50% + var(--cosmos-action-y))) scale(1);
            }
          }
        `}
      </style>
      {actions.map((action) => {
        const rad = action.angle * Math.PI / 180
        const x = Math.cos(rad) * radius
        const y = Math.sin(rad) * radius
        const showTooltip = hoveredAction === action.key
        const actionStyle = {
          '--cosmos-action-x': `${x}px`,
          '--cosmos-action-y': `${y}px`,
        } as React.CSSProperties
        return (
          <button
            key={action.key}
            type="button"
            title={action.label}
            aria-label={action.label}
            onMouseEnter={() => setHoveredAction(action.key)}
            onMouseLeave={() => setHoveredAction(null)}
            onMouseDown={(event) => {
              event.preventDefault()
              event.stopPropagation()
            }}
            onClick={(event) => {
              event.preventDefault()
              event.stopPropagation()
              action.onClick()
            }}
            style={{
              position: 'absolute',
              left: 0,
              top: 0,
              width: actionSize,
              height: actionSize,
              transform: `translate(calc(-50% + ${x}px), calc(-50% + ${y}px))`,
              animation: 'cosmos-node-action-pop 170ms cubic-bezier(0.16, 1, 0.3, 1)',
              ...actionStyle,
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              borderRadius: '50%',
              border: `1px solid ${info.color}`,
              background: 'rgba(255, 255, 255, 0.98)',
              color: info.color,
              boxShadow: '0 10px 24px rgba(15, 23, 42, 0.12), 0 0 0 3px rgba(255, 255, 255, 0.86)',
              cursor: 'pointer',
              pointerEvents: 'auto',
              padding: 0,
            }}
          >
            {action.icon}
            {showTooltip && (
              <span
                style={{
                  position: 'absolute',
                  left: '50%',
                  bottom: 'calc(100% + 7px)',
                  transform: 'translateX(-50%)',
                  padding: '3px 7px',
                  borderRadius: 6,
                  background: 'rgba(15, 23, 42, 0.92)',
                  color: '#fff',
                  fontSize: 11,
                  fontWeight: 600,
                  whiteSpace: 'nowrap',
                  pointerEvents: 'none',
                }}
              >
                {action.label}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}

interface MiniMapPoint {
  x: number
  y: number
  color: string
}

interface MiniMapData {
  points: MiniMapPoint[]
  viewport: Array<[number, number]>
  bounds: {
    minX: number
    maxX: number
    minY: number
    maxY: number
  }
}

const MINI_MAP_WIDTH = 168
const MINI_MAP_HEIGHT = 112
const MINI_MAP_PAD = 8

const miniMapStyle: React.CSSProperties = {
  position: 'absolute',
  right: 14,
  bottom: 14,
  width: MINI_MAP_WIDTH,
  height: MINI_MAP_HEIGHT,
  borderRadius: 8,
  border: '1px solid rgba(148, 163, 184, 0.24)',
  background: 'rgba(255, 255, 255, 0.88)',
  boxShadow: '0 12px 32px rgba(15, 23, 42, 0.10)',
  backdropFilter: 'blur(10px)',
  zIndex: 7,
  pointerEvents: 'auto',
  overflow: 'hidden',
  cursor: 'crosshair',
  userSelect: 'none',
  touchAction: 'none',
}

function buildMiniMapData(engine: CosmosEngine | null): MiniMapData | null {
  if (!engine) return null
  const graph = engine.getGraph()
  if (!graph) return null

  const nodes = engine.getNodes() as any[]
  if (!nodes.length) return null

  const positions = engine.getPointPositionsForRender()
  if (!positions || !positions.length) return null

  const maxPoints = 1400
  const stride = Math.max(1, Math.ceil(nodes.length / maxPoints))
  const points: MiniMapPoint[] = []
  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity

  for (let i = 0; i < nodes.length; i += 1) {
    const x = positions[i * 2]
    const y = positions[i * 2 + 1]
    if (!Number.isFinite(x) || !Number.isFinite(y)) continue
    minX = Math.min(minX, x)
    maxX = Math.max(maxX, x)
    minY = Math.min(minY, y)
    maxY = Math.max(maxY, y)
    if (i % stride === 0) points.push({ x, y, color: nodes[i].iconFill || nodes[i].color || '#2563eb' })
  }

  const container = (engine as any).container as HTMLDivElement | undefined
  const width = container?.clientWidth || 0
  const height = container?.clientHeight || 0
  const viewportSpace: Array<[number, number]> = []
  if (width && height) {
    ;[[0, 0], [width, 0], [width, height], [0, height]].forEach((screen) => {
      try {
        const point = graph.screenToSpacePosition(screen as [number, number])
        if (Number.isFinite(point[0]) && Number.isFinite(point[1])) {
          viewportSpace.push(point)
          minX = Math.min(minX, point[0])
          maxX = Math.max(maxX, point[0])
          minY = Math.min(minY, point[1])
          maxY = Math.max(maxY, point[1])
        }
      } catch { /* ignore */ }
    })
  }

  if (!Number.isFinite(minX) || !Number.isFinite(maxX) || minX === maxX || minY === maxY) return null

  const w = MINI_MAP_WIDTH
  const h = MINI_MAP_HEIGHT
  const pad = MINI_MAP_PAD
  const scaleX = (x: number) => pad + ((x - minX) / (maxX - minX)) * (w - pad * 2)
  const scaleY = (y: number) => pad + (1 - ((y - minY) / (maxY - minY))) * (h - pad * 2)

  return {
    points: points.map(point => ({ ...point, x: scaleX(point.x), y: scaleY(point.y) })),
    viewport: viewportSpace.map(point => [scaleX(point[0]), scaleY(point[1])] as [number, number]),
    bounds: { minX, maxX, minY, maxY },
  }
}

function CosmosMiniMap({ engine, tick }: { engine: CosmosEngine | null; tick: number }) {
  const { t } = useI18n()
  const data = useMemo(() => buildMiniMapData(engine), [engine, Math.floor(tick / 5)])
  const mapRef = useRef<HTMLDivElement>(null)
  const draggingRef = useRef(false)

  const moveView = useCallback((event: React.PointerEvent<HTMLDivElement>, duration: number) => {
    if (!data || !engine || !mapRef.current) return
    const rect = mapRef.current.getBoundingClientRect()
    const x = Math.max(MINI_MAP_PAD, Math.min(MINI_MAP_WIDTH - MINI_MAP_PAD, event.clientX - rect.left))
    const y = Math.max(MINI_MAP_PAD, Math.min(MINI_MAP_HEIGHT - MINI_MAP_PAD, event.clientY - rect.top))
    const width = Math.max(1, MINI_MAP_WIDTH - MINI_MAP_PAD * 2)
    const height = Math.max(1, MINI_MAP_HEIGHT - MINI_MAP_PAD * 2)
    const spaceX = data.bounds.minX + ((x - MINI_MAP_PAD) / width) * (data.bounds.maxX - data.bounds.minX)
    const spaceY = data.bounds.maxY - ((y - MINI_MAP_PAD) / height) * (data.bounds.maxY - data.bounds.minY)
    engine.centerViewAtSpacePosition([spaceX, spaceY], duration)
    event.preventDefault()
    event.stopPropagation()
  }, [data, engine])

  const handlePointerDown = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    draggingRef.current = true
    event.currentTarget.setPointerCapture?.(event.pointerId)
    moveView(event, 180)
  }, [moveView])

  const handlePointerMove = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    if (!draggingRef.current) return
    moveView(event, 40)
  }, [moveView])

  const handlePointerUp = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    draggingRef.current = false
    event.currentTarget.releasePointerCapture?.(event.pointerId)
    event.preventDefault()
    event.stopPropagation()
  }, [])

  if (!data) return null

  const viewportPath = data.viewport.length === 4
    ? data.viewport.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point[0].toFixed(1)} ${point[1].toFixed(1)}`).join(' ') + ' Z'
    : ''

  return (
    <div
      ref={mapRef}
      style={miniMapStyle}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
    >
      <svg width={MINI_MAP_WIDTH} height={MINI_MAP_HEIGHT} viewBox={`0 0 ${MINI_MAP_WIDTH} ${MINI_MAP_HEIGHT}`} role="img" aria-label={t('entityTopoExplorer.graph.miniMap')}>
        <rect x="0" y="0" width={MINI_MAP_WIDTH} height={MINI_MAP_HEIGHT} fill="#fff" />
        {data.points.map((point, index) => (
          <circle key={index} cx={point.x} cy={point.y} r="1.15" fill={point.color} opacity="0.72" />
        ))}
        {viewportPath && (
          <path d={viewportPath} fill="rgba(79, 70, 229, 0.08)" stroke="#4f46e5" strokeWidth="1.2" />
        )}
      </svg>
    </div>
  )
}

/* ═══════════════════════════════════════════════════════════
   Component
   ═══════════════════════════════════════════════════════════ */

export const CosmosTopoGraph = forwardRef<TopoGraphRef, Omit<TopoGraphProps, 'type'>>(
  (props, ref) => {
    const { t } = useI18n()
    const hostRef = useRef<HTMLDivElement>(null)
    const engineRef = useRef<CosmosEngine | null>(null)
    const graphApiRef = useRef<ITopoGraph | null>(null)
    const mountedRef = useRef(false)

    /* Data state */
    const [data, setData] = useState<TopoData>(props.data || emptyData)
    const [layout, setLayout] = useState<LayoutOptions>({ mode: 'clustered', ...(props.layout || {}) })
    const [isolatedNodeId, setIsolatedNodeId] = useState<string | null>(null)
    const [selectedFilterKeys, setSelectedFilterKeys] = useState<string[]>([])
    const [tick, setTick] = useState(0)
    const [hoverPanel, setHoverPanel] = useState<HoverPanelInfo | null>(null)
    const [rendering, setRendering] = useState(false)

    /* Refs for stable access in callbacks */
    const dataRef = useRef(data)
    const initialDataRef = useRef(props.data || emptyData)
    const layoutRef = useRef(layout)
    const tRef = useRef(t)
    const renderDataRef = useRef<TopoData>(data)
    const lastLoadedDataRef = useRef<TopoData | null>(null)
    const isolatedNodeIdRef = useRef(isolatedNodeId)
    const renderSeqRef = useRef(0)
    const tickRafRef = useRef<number | null>(null)

    dataRef.current = data
    layoutRef.current = layout
    tRef.current = t
    isolatedNodeIdRef.current = isolatedNodeId

    const handleNodeClickRef = useRef(props.handleNodeClick)
    const handleEdgeClickRef = useRef(props.handleEdgeClick)
    const handleCloseInfoRef = useRef(props.handleCloseInfo)
    const handleNodeIsolateRef = useRef(props.handleNodeIsolate)
    handleNodeClickRef.current = props.handleNodeClick
    handleEdgeClickRef.current = props.handleEdgeClick
    handleCloseInfoRef.current = props.handleCloseInfo
    handleNodeIsolateRef.current = props.handleNodeIsolate

    useEffect(() => {
      mountedRef.current = true
      return () => {
        mountedRef.current = false
        renderSeqRef.current += 1
        if (tickRafRef.current !== null) {
          window.cancelAnimationFrame(tickRafRef.current)
          tickRafRef.current = null
        }
      }
    }, [])

    const requestTick = useCallback(() => {
      if (!mountedRef.current || tickRafRef.current !== null) return
      tickRafRef.current = window.requestAnimationFrame(() => {
        tickRafRef.current = null
        if (mountedRef.current) setTick(c => c + 1)
      })
    }, [])

    const beginRendering = useCallback(() => {
      const seq = renderSeqRef.current + 1
      renderSeqRef.current = seq
      setRendering(true)
      return seq
    }, [])

    const finishRendering = useCallback((seq = renderSeqRef.current) => {
      window.requestAnimationFrame(() => {
        window.requestAnimationFrame(() => {
          if (!mountedRef.current || renderSeqRef.current !== seq) return
          setRendering(false)
        })
      })
    }, [])

    /* Derived data (isolation) */
    const renderData = useMemo<TopoData>(() => {
      if (!isolatedNodeId) return data
      return buildNeighborSubgraph(data, isolatedNodeId)
    }, [data, isolatedNodeId])
    renderDataRef.current = renderData

    /* Sync props */
    useEffect(() => {
      if (props.data) {
        const seq = beginRendering()
        dataRef.current = props.data
        setData(props.data)
        finishRendering(seq)
      }
    }, [beginRendering, finishRendering, props.data])

    useEffect(() => {
      if (props.layout) {
        const next = { ...layoutRef.current, ...props.layout }
        if (areLayoutOptionsEqual(layoutRef.current, next)) return
        const seq = beginRendering()
        layoutRef.current = next
        setLayout(next)
        engineRef.current?.setRuntimeLayout(withFocusedDetailLayout(next, renderDataRef.current, isolatedNodeIdRef.current))
        finishRendering(seq)
      }
    }, [beginRendering, finishRendering, props.layout])

    /* ── Engine lifecycle ── */
    useEffect(() => {
      const host = hostRef.current
      const initialRenderData = renderDataRef.current
      if (!host || initialRenderData.nodes.length === 0) return

      const engine = new CosmosEngine(host, {
        callbacks: {
          onNodeClick: (rawNode) => {
            handleNodeClickRef.current?.(rawNode)
          },
          onBackgroundClick: () => {
            handleCloseInfoRef.current?.()
          },
          onEdgeClick: (sourceId, targetId, edgeIndex) => {
            handleEdgeClickRef.current?.({ source: sourceId, target: targetId, edgeIndex })
          },
          onSimulationTick: () => {
            requestTick()
          },
          onZoom: () => {
            requestTick()
          },
          onHoverChange: (hover) => {
            if (!mountedRef.current) return
            setHoverPanel(buildHoverPanelInfo(hover, engine as any, tRef.current))
          },
        },
      })

      engine.loadData(
        initialRenderData,
        withFocusedDetailLayout(layoutRef.current, initialRenderData, isolatedNodeIdRef.current),
      )
      engineRef.current = engine
      lastLoadedDataRef.current = initialRenderData
      requestTick()
      finishRendering()

      return () => {
        engine.destroy()
        engineRef.current = null
        lastLoadedDataRef.current = null
        setHoverPanel(null)
      }
    }, [
      layout.mode,
      layout.graphvizEngine,
      layout.graphvizRankdir,
      layout.clusterByType,
      layout.simulationFriction,
      layout.simulationGravity,
      layout.simulationRepulsion,
    ]) // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => {
      const engine = engineRef.current
      if (!engine || renderData.nodes.length === 0) {
        finishRendering()
        return
      }
      if (lastLoadedDataRef.current === renderData) return
      const seq = renderSeqRef.current
      engine.setRuntimeLayout(withFocusedDetailLayout(layoutRef.current, renderData, isolatedNodeIdRef.current))
      engine.updateData(renderData)
      lastLoadedDataRef.current = renderData
      setHoverPanel(null)
      requestTick()
      finishRendering(seq)
    }, [finishRendering, renderData, requestTick])

    /* ── Drag toggle (without recreating engine) ── */
    useEffect(() => {
      if (!engineRef.current) {
        finishRendering()
        return
      }
      const seq = renderSeqRef.current
      engineRef.current.updateLayout(withFocusedDetailLayout(layout, renderDataRef.current, isolatedNodeIdRef.current))
      finishRendering(seq)
    }, [finishRendering, layout.enableDrag]) // eslint-disable-line react-hooks/exhaustive-deps

    /* ── Filter sync ── */
    useEffect(() => {
      if (!engineRef.current) {
        finishRendering()
        return
      }
      const seq = renderSeqRef.current
      if (selectedFilterKeys.length === 0 || isolatedNodeId) {
        engineRef.current.filterByClusters(null)
      } else {
        engineRef.current.filterByClusters(selectedFilterKeys)
      }
      finishRendering(seq)
    }, [finishRendering, selectedFilterKeys, isolatedNodeId])

    /* ── Build cluster summaries for filter API ── */
    const clusterInfo = useMemo(() => {
      const engine = engineRef.current
      if (!engine) return [] as any[]
      return engine.getClusters().map(c => ({
        key: c.key,
        label: c.label,
        color: c.iconFill,
        count: c.count,
        visual: resolveTopoVisualIdentity({
          cluster: c.key,
          title: c.label,
          style: { iconFill: c.iconFill },
          fallbackColor: c.iconFill,
        }),
      }))
    }, [renderData]) // eslint-disable-line react-hooks/exhaustive-deps

    const clusterKeys = useMemo(() => clusterInfo.map(c => c.key), [clusterInfo])

    const isolateHoveredNode = useCallback((nodeId: string) => {
      const seq = beginRendering()
      if (layoutRef.current.externalFocusMode) {
        engineRef.current?.clearHover()
        setHoverPanel(null)
        handleNodeIsolateRef.current?.(nodeId)
        finishRendering(seq)
        return
      }
      setIsolatedNodeId(nodeId)
      engineRef.current?.clearHover()
      setHoverPanel(null)
      handleNodeIsolateRef.current?.(nodeId)
      finishRendering(seq)
    }, [beginRendering, finishRendering])

    /* ── ITopoGraph imperative API ── */
    const getGraphApi = useCallback((): ITopoGraph => {
      if (graphApiRef.current) return graphApiRef.current

      const api: ITopoGraph = {
        setData: (next: TopoData, _opts?: TopoDataUpdateOptions) => {
          const seq = beginRendering()
          dataRef.current = next
          setData(next)
          finishRendering(seq)
        },
        getData: () => dataRef.current,
        resetData: () => {
          const seq = beginRendering()
          dataRef.current = initialDataRef.current
          setData(initialDataRef.current)
          setSelectedFilterKeys([])
          setIsolatedNodeId(null)
          handleNodeIsolateRef.current?.(null)
          finishRendering(seq)
        },
        setLayout: (next: LayoutOptions) => {
          const seq = beginRendering()
          layoutRef.current = { ...layoutRef.current, ...next }
          setLayout({ ...layoutRef.current })
          engineRef.current?.setRuntimeLayout(withFocusedDetailLayout(layoutRef.current, renderDataRef.current, isolatedNodeIdRef.current))
          finishRendering(seq)
        },
        setRuntimeLayout: (next: LayoutOptions) => {
          layoutRef.current = { ...layoutRef.current, ...next }
          engineRef.current?.setRuntimeLayout(withFocusedDetailLayout(layoutRef.current, renderDataRef.current, isolatedNodeIdRef.current))
        },
        getLayout: () => layoutRef.current,

        focusNodeView: (nodeId: string) => {
          const engine = engineRef.current
          if (!engine) return
          engine.lockNode(nodeId)
        },
        resetView: () => {
          engineRef.current?.clearHover()
          engineRef.current?.fitView()
        },
        clearFocus: () => {
          engineRef.current?.clearHover()
        },

        isolateNode: (nodeId: string) => {
          const seq = beginRendering()
          if (layoutRef.current.externalFocusMode) {
            handleNodeIsolateRef.current?.(nodeId)
            finishRendering(seq)
            return
          }
          setIsolatedNodeId(nodeId)
          handleNodeIsolateRef.current?.(nodeId)
          finishRendering(seq)
        },
        clearIsolation: () => {
          const seq = beginRendering()
          if (layoutRef.current.externalFocusMode) {
            handleNodeIsolateRef.current?.(null)
            finishRendering(seq)
            return
          }
          setIsolatedNodeId(null)
          handleNodeIsolateRef.current?.(null)
          finishRendering(seq)
        },
        setIsolationLayout: () => { /* cosmos uses GPU layout, no CPU isolation layout switch */ },
        getIsolationLayout: () => 'force' as any,

        getFilterState: () => ({
          items: clusterInfo,
          selectedKeys: selectedFilterKeys,
          activeKeys: selectedFilterKeys.length > 0 ? selectedFilterKeys : clusterKeys,
          isFilterActive: selectedFilterKeys.length > 0,
          activeNodeCount: 0,
          totalNodeCount: dataRef.current.nodes.length,
          activeEdgeCount: 0,
          totalEdgeCount: dataRef.current.edges.length,
        }),
        setSelectedFilterKeys: (keys: string[]) => {
          const seq = beginRendering()
          const valid = new Set(clusterKeys)
          setSelectedFilterKeys(keys.filter(k => valid.has(k)))
          finishRendering(seq)
        },
        toggleFilterKey: (key: string) => {
          const seq = beginRendering()
          setSelectedFilterKeys(cur => {
            if (cur.includes(key)) return cur.filter(k => k !== key)
            return [...cur, key]
          })
          finishRendering(seq)
        },
        clearFilter: () => {
          const seq = beginRendering()
          setSelectedFilterKeys([])
          finishRendering(seq)
        },

        render: async () => { engineRef.current?.getGraph()?.render() },
        fitView: async () => { engineRef.current?.fitView() },
        fitCenter: async () => { engineRef.current?.fitView() },
        getViewState: () => engineRef.current?.getViewState() ?? null,
        restoreViewState: (state, options) => {
          engineRef.current?.restoreViewState(state, options?.duration ?? 0)
        },

        setOptions: (opts) => {
          const seq = opts.data || opts.layout ? beginRendering() : null
          if (opts.data) { dataRef.current = opts.data; setData(opts.data) }
          if (opts.layout) {
            layoutRef.current = { ...layoutRef.current, ...opts.layout }
            setLayout({ ...layoutRef.current })
            engineRef.current?.setRuntimeLayout(withFocusedDetailLayout(layoutRef.current, renderDataRef.current, isolatedNodeIdRef.current))
          }
          if (seq !== null) finishRendering(seq)
        },

        on: () => api,
        once: () => api,
        off: () => api,
      }

      graphApiRef.current = api
      return api
    }, [clusterKeys, selectedFilterKeys]) // eslint-disable-line react-hooks/exhaustive-deps

    useImperativeHandle(ref, () => ({
      getGraph: () => getGraphApi(),
    }), [getGraphApi])

    /* ── Render ── */
    return (
      <div style={{ ...rootStyle, ...props.style }} className={props.className}>
        {rendering && (
          <>
            <style>{'@keyframes cosmos-render-spin{to{transform:rotate(360deg)}}'}</style>
            <div style={graphRenderingOverlayStyle}>
              <div style={graphRenderingPillStyle}>
                <span style={graphRenderingSpinnerStyle} />
                <span>{t('entityTopoExplorer.graph.rendering')}</span>
              </div>
            </div>
          </>
        )}
        <div ref={hostRef} style={canvasHostStyle} />
        <CosmosVectorIconOverlay engine={engineRef.current} tick={tick} />
        <CosmosLabels
          engine={engineRef.current}
          isIsolationMode={Boolean(isolatedNodeId) || Boolean(layout.forceDetailedView && renderData.nodes.length <= FOCUSED_DETAIL_NODE_LIMIT)}
          showLabels={
            Boolean((isolatedNodeId || layout.forceDetailedView) && renderData.nodes.length <= FOCUSED_DETAIL_NODE_LIMIT) ||
            layout.showLabels !== false
          }
          showClusterLabels={layout.showClusterLabels !== false}
          showEdgeLabels={layout.showEdgeLabels !== false}
          tick={tick}
        />
        <CosmosHoverPanel info={hoverPanel} />
        {props.handleNodeIsolate && <CosmosNodeActionMenu engine={engineRef.current} tick={tick} onIsolate={isolateHoveredNode} />}
        {layout.showMiniMap !== false && <CosmosMiniMap engine={engineRef.current} tick={tick} />}
        {isolatedNodeId && (
          <div style={isolationBarStyle}>
            <span style={{ fontWeight: 500 }}>{t('entityTopoExplorer.graph.focusView')}</span>
            <span style={{ color: '#9ca3af' }}>|</span>
            <span style={{ color: '#6b7280' }}>{isolatedNodeId}</span>
            <button
              type="button"
              style={isolationBtnStyle}
              onClick={() => {
                const seq = beginRendering()
                setIsolatedNodeId(null)
                handleNodeIsolateRef.current?.(null)
                finishRendering(seq)
              }}
            >
              {t('entityTopoExplorer.action.exit')}
            </button>
          </div>
        )}
        {props.children}
      </div>
    )
  },
)
