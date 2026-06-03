import { memo, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import ReactDOM from 'react-dom'
import { Cable, CircleDashed, Copy, Crosshair, GitBranch, Trash2 } from 'lucide-react'
import {
  Background,
  BackgroundVariant,
  BaseEdge,
  Controls,
  EdgeLabelRenderer,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  getBezierPath,
  useReactFlow,
  type Edge,
  type EdgeProps,
  type Node,
  type NodeChange,
  type NodeProps,
} from '@xyflow/react'
import type { UModelElement } from '../../api/types'
import { useI18n } from '../../i18n'
import type { UModelEdgeData, UModelNodeData, GraphModel } from './graphModel'
import {
  colorForKind,
  entityLinkTypeForEdge,
  maxVisibleTags,
  nodeMinHeight,
  nodeWidth,
  type BackgroundStyle,
  type ZoomLevel,
  elementKey,
} from './model'
import { iconForKind } from './kindIcon'

export function GraphView({
  graph,
  focusIds,
  zoomLevel,
  layouting,
  backgroundStyle,
  forceFullMode,
  selectedId,
  onZoomLevelChange,
  onNodesChange,
  onSelect,
}: {
  graph: GraphModel
  focusIds: string[]
  zoomLevel: ZoomLevel
  layouting: boolean
  backgroundStyle: BackgroundStyle
  forceFullMode: boolean
  selectedId: string | null
  onZoomLevelChange: (level: ZoomLevel) => void
  onNodesChange: (changes: NodeChange<Node<UModelNodeData>>[]) => void
  onSelect: (element: UModelElement | null) => void
}) {
  const { t } = useI18n()
  const { fitView } = useReactFlow()
  const focusKey = focusIds.join('\u001f')
  const nodeIdsKey = graph.nodes.map((node) => node.id).join('\u001f')
  const displayNodes = useMemo(
    () => graph.nodes.map((node) => ({ ...node, selected: selectedId === node.id })),
    [graph.nodes, selectedId],
  )
  const displayEdges = useMemo(
    () => graph.edges.map((edge) => ({ ...edge, selected: selectedId === (edge.data?.element ? elementKey(edge.data.element) : '') })),
    [graph.edges, selectedId],
  )
  const ariaLabelConfig = useMemo(() => ({
    'controls.ariaLabel': t('umodelExplorer.flow.controlPanel'),
    'controls.zoomIn.ariaLabel': t('umodelExplorer.flow.zoomIn'),
    'controls.zoomOut.ariaLabel': t('umodelExplorer.flow.zoomOut'),
    'controls.fitView.ariaLabel': t('umodelExplorer.flow.fitView'),
    'minimap.ariaLabel': t('umodelExplorer.flow.miniMap'),
  }), [t])

  useEffect(() => {
    if (graph.nodes.length === 0 || layouting) return
    const focusNodes = focusIds.length > 0 ? graph.nodes.filter((node) => focusIds.includes(node.id)) : []
    const timer = window.setTimeout(() => {
      if (focusNodes.length > 0) fitView({ duration: 800, padding: 0.2, nodes: focusNodes })
      else fitView({ duration: 800, padding: 0.2 })
    }, 120)
    return () => window.clearTimeout(timer)
  }, [fitView, focusKey, nodeIdsKey, layouting])

  return (
    <div className="v2-graph-container" data-zoom={forceFullMode ? 'full' : zoomLevel}>
      <ReactFlow
        nodes={displayNodes}
        edges={displayEdges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        defaultViewport={{ x: 44, y: 220, zoom: 0.85 }}
        fitView={false}
        minZoom={0.04}
        maxZoom={3}
        onlyRenderVisibleElements={false}
        proOptions={{ hideAttribution: true }}
        onNodeClick={(_, node) => onSelect(node.data.element)}
        onEdgeClick={(_, edge) => onSelect(edge.data?.element || null)}
        onNodesChange={onNodesChange}
        onPaneClick={() => onSelect(null)}
        onMove={(_, viewport) => {
          const level = forceFullMode ? 'full' : viewport.zoom < 0.35 ? 'mini' : viewport.zoom < 0.55 ? 'compact' : 'full'
          if (level !== zoomLevel) onZoomLevelChange(level)
        }}
        ariaLabelConfig={ariaLabelConfig}
        nodesDraggable
        nodesConnectable={false}
        elementsSelectable
        style={{ background: 'var(--ume-color-bg)' }}
      >
        {backgroundStyle === 'dots' && <Background variant={BackgroundVariant.Dots} gap={28} size={2} color="#b0b0b0" />}
        {backgroundStyle === 'lines' && <Background variant={BackgroundVariant.Lines} gap={120} size={1.5} color="#c8c8c8" />}
        {backgroundStyle === 'cross' && <Background variant={BackgroundVariant.Cross} gap={28} size={8} color="#b0b0b0" />}
        <Controls showInteractive={false} aria-label={t('umodelExplorer.flow.controlPanel')} />
        <MiniMap ariaLabel={t('umodelExplorer.flow.miniMap')} nodeStrokeWidth={3} pannable zoomable style={{ background: 'var(--ume-color-bg-subtle)' }} />
      </ReactFlow>
      {layouting && <div className="ume-layout-badge">{t('umodelExplorer.loading.arrangingView')}</div>}
    </div>
  )
}

const UModelNodeCard = memo(({ data }: NodeProps<Node<UModelNodeData>>) => {
  const { t } = useI18n()
  const [menuOpen, setMenuOpen] = useState(false)
  const [menuRect, setMenuRect] = useState<DOMRect | null>(null)
  const menuButtonRef = useRef<HTMLButtonElement | null>(null)
  const color = data.color
  const hiddenTotal = Math.max(0, data.totalTagCount - Math.min(maxVisibleTags, data.tags.length))

  function stop(event: React.MouseEvent) {
    event.preventDefault()
    event.stopPropagation()
  }

  function runMenuAction(action: () => void) {
    action()
    setMenuOpen(false)
  }

  useEffect(() => {
    if (!menuOpen) return
    const updateRect = () => {
      if (menuButtonRef.current) setMenuRect(menuButtonRef.current.getBoundingClientRect())
    }
    const close = () => setMenuOpen(false)
    updateRect()
    window.addEventListener('resize', updateRect)
    window.addEventListener('scroll', updateRect, true)
    window.addEventListener('click', close)
    return () => {
      window.removeEventListener('resize', updateRect)
      window.removeEventListener('scroll', updateRect, true)
      window.removeEventListener('click', close)
    }
  }, [menuOpen])

  const nodeMenu =
    menuOpen && menuRect
      ? ReactDOM.createPortal(
          <div
            className="ume-node-menu floating"
            style={{
              left: Math.min(Math.max(8, menuRect.right - 198), window.innerWidth - 206),
              top: menuRect.bottom + 6,
            } as CSSProperties}
            onMouseDown={stop}
            onClick={stop}
          >
            <button onClick={() => runMenuAction(() => data.actions.onConnect(data.element))} type="button">
              <Cable size={13} />
              {t('umodelExplorer.nodeMenu.connectTo')}
            </button>
            <button onClick={() => runMenuAction(() => data.actions.onCopy(data.element))} type="button">
              <Copy size={13} />
              {t('umodelExplorer.nodeMenu.copyNode')}
            </button>
            <button onClick={() => runMenuAction(() => data.actions.onCopyCascade(data.element))} type="button">
              <GitBranch size={13} />
              {t('umodelExplorer.nodeMenu.copyWithEdges')}
            </button>
            <button className="danger" onClick={() => runMenuAction(() => data.actions.onDelete(data.element, true))} type="button">
              <Trash2 size={13} />
              {t('umodelExplorer.nodeMenu.deleteNode')}
            </button>
            <button className="danger" onClick={() => runMenuAction(() => data.actions.onDelete(data.element, true))} type="button">
              <GitBranch size={13} />
              {t('umodelExplorer.nodeMenu.deleteWithEdges')}
            </button>
          </div>,
          document.body,
        )
      : null

  if (data.kind === 'entity_set_link') {
    return (
      <div
        className={`v2-node-card ume-entity-link-node ${data.draftStatus ? `draft-${data.draftStatus}` : ''}`}
        style={{ '--node-color': color.color } as CSSProperties}
        onClick={(event) => {
          event.stopPropagation()
          data.actions.onSelect(data.element)
        }}
      >
        <div className="ume-entity-link-pill">
          <GitBranch size={12} />
          <span>{entityLinkTypeForEdge(data.element)}</span>
          {data.draftStatus && <CircleDashed size={11} />}
        </div>
        <Handle id="source" type="source" position={Position.Right} className="v2-handle-source" />
        <Handle id="target" type="target" position={Position.Left} className="v2-handle-target" />
      </div>
    )
  }

  return (
    <div
      className={`v2-node-card ${data.draftStatus ? `draft-${data.draftStatus}` : ''}`}
      style={{ '--node-color': color.color } as CSSProperties}
      onClick={(event) => {
        event.stopPropagation()
        data.actions.onSelect(data.element)
      }}
    >
      <div className="ume-node-top-menu" style={{ background: color.color }} onMouseDown={stop} onClick={stop}>
        <button onClick={() => data.actions.onFocus(data.element)} type="button" title={t('umodelExplorer.action.focusNode')}>
          <Crosshair size={12} />
        </button>
      </div>

      <div className="v2-zoom-mini" style={{ width: nodeWidth, minHeight: nodeMinHeight }}>
        <div className="v2-node-card-body ume-node-mini" style={{ borderColor: color.color }}>
          <span style={{ color: color.color }}>{data.title}</span>
        </div>
      </div>

      <div className="v2-zoom-compact" style={{ width: nodeWidth, minHeight: nodeMinHeight }}>
        <div className="v2-node-card-body ume-node-compact">
          <div className="ume-node-stripe" style={{ background: color.color }} />
          <div className="ume-node-compact-main">
            <span className="ume-kind-tag large" style={{ background: color.bg, color: color.text }}>
              {color.label}
            </span>
            <strong>{data.title}</strong>
          </div>
        </div>
      </div>

      <div className="v2-zoom-full">
        <div className="v2-node-card-body ume-node-full">
          <div className="ume-node-stripe thin" style={{ background: color.color }} />
          <div className="ume-node-content">
            <div className="ume-node-meta">
              {iconForKind(data.kind)}
              <span className="ume-kind-tag" style={{ background: color.bg, color: color.text }}>
                {color.label}
              </span>
              <code>{data.domain || t('umodelExplorer.misc.unknown')}</code>
              {data.draftStatus && (
                <span className={`ume-node-draft ${data.draftStatus}`}>
                  <CircleDashed size={10} />
                  {data.draftStatus === 'added' ? t('umodelExplorer.dialog.diff.type.added') : t('umodelExplorer.dialog.diff.type.modified')}
                </span>
              )}
              <button
                ref={menuButtonRef}
                className="ume-node-operation-button"
                onClick={(event) => {
                  stop(event)
                  setMenuOpen((value) => !value)
                }}
                type="button"
                aria-label={t('umodelExplorer.aria.operations')}
              >
                <span aria-hidden="true">•••</span>
              </button>
            </div>
            <strong className="ume-node-title">{data.title}</strong>
            {data.tags.length > 0 && (
              <div className="ume-node-tags">
                {data.tags.slice(0, maxVisibleTags).map((tag) => (
                  <NodeTag key={tag} text={tag} />
                ))}
                {hiddenTotal > 0 && <span>+{hiddenTotal}</span>}
              </div>
            )}
          </div>
        </div>
      </div>

      <Handle id="source" type="source" position={Position.Right} className="v2-handle-source" />
      <Handle id="target" type="target" position={Position.Left} className="v2-handle-target" />
      {nodeMenu}
    </div>
  )
})

function NodeTag({ text }: { text: string }) {
  const [hover, setHover] = useState(false)
  const ref = useRef<HTMLSpanElement | null>(null)
  return (
    <>
      <span ref={ref} onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}>
        {text}
      </span>
      {hover && ref.current && ReactDOM.createPortal(
        <div
          className="ume-node-tag-tooltip"
          style={{
            left: ref.current.getBoundingClientRect().left + ref.current.getBoundingClientRect().width / 2,
            top: ref.current.getBoundingClientRect().top - 6,
          } as CSSProperties}
          onClick={(event) => event.stopPropagation()}
          onPointerDown={(event) => event.stopPropagation()}
        >
          {text}
        </div>,
        document.body,
      )}
    </>
  )
}

function UModelEdge({
  id,
  source,
  target,
  selected,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}: EdgeProps<Edge<UModelEdgeData>>) {
  const { t } = useI18n()
  let edgePath: string
  let labelX: number
  let labelY: number
  if (source === target) {
    const radiusX = Math.max(80, Math.abs(sourceX - targetX) * 0.8 || 120)
    const radiusY = 100
    edgePath = `M ${sourceX - 5} ${sourceY} A ${radiusX} ${radiusY} 0 1 0 ${targetX + 2} ${targetY}`
    labelX = (sourceX + targetX) / 2
    labelY = sourceY - radiusY - 80
  } else {
    const bezier = getBezierPath({
      sourceX,
      sourceY,
      targetX,
      targetY,
      sourcePosition,
      targetPosition,
    })
    edgePath = bezier[0]
    labelX = bezier[1]
    labelY = bezier[2]
  }
  const safeId = id.replace(/[^a-zA-Z0-9_-]/g, '_')
  const sourceColor = data?.sourceColor || colorForKind(data?.sourceKind || 'entity_set').color
  const targetColor = data?.targetColor || colorForKind(data?.targetKind || data?.kind || 'data_link').color
  const isDraft = Boolean(data?.draftStatus)
  const label = data?.kind === 'entity_set_link'
    ? entityLinkTypeForEdge(data.element)
    : data?.draftStatus === 'added'
      ? t('umodelExplorer.dialog.diff.type.added')
      : data?.draftStatus === 'modified'
        ? t('umodelExplorer.dialog.diff.type.modified')
        : undefined
  return (
    <>
      <defs>
        <linearGradient
          id={`v2-edge-grad-${safeId}`}
          gradientUnits="userSpaceOnUse"
          x1={source === target ? sourceX - 96 : sourceX}
          y1={source === target ? sourceY - 80 : sourceY}
          x2={targetX}
          y2={targetY}
        >
          <stop offset="0%" stopColor={sourceColor} stopOpacity={0.34} />
          <stop offset="100%" stopColor={targetColor} stopOpacity={0.72} />
        </linearGradient>
      </defs>
      <path className="v2-edge-hitarea" d={edgePath} fill="none" stroke="transparent" strokeWidth={16} />
      {selected && (
        <BaseEdge
          path={edgePath}
          style={{
            stroke: targetColor,
            strokeWidth: 8,
            opacity: 0.16,
            pointerEvents: 'none',
          }}
        />
      )}
      <BaseEdge
        id={id}
        path={edgePath}
        style={{
          stroke: `url(#v2-edge-grad-${safeId})`,
          strokeWidth: selected ? 3.1 : 2.4,
          strokeDasharray: isDraft ? '6 4' : undefined,
          opacity: selected ? 0.95 : 0.8,
          pointerEvents: 'none',
        }}
      />
      <circle cx={targetX} cy={targetY} r={4.2} fill={targetColor} opacity={selected ? 0.95 : 0.78} />
      {data && label && (
        <EdgeLabelRenderer>
          <div className={`ume-edge-label ${data.draftStatus ? `draft-${data.draftStatus}` : ''}`} style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)` }}>
            {data.draftStatus && <CircleDashed size={10} />}
            {label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  )
}

const nodeTypes = { umodel: UModelNodeCard }
const edgeTypes = { umodel: UModelEdge }
