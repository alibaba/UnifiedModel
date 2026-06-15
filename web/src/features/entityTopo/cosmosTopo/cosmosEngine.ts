import { Graph } from '@cosmos.gl/graph'
import type {
  TopoData,
  TopoNode,
  TopoEdge,
  LayoutOptions,
  NodeRecord,
  ClusterRecord,
  HoverState,
  FocusedEdgeRecord,
  CosmosVectorIconOverlayItem,
  CosmosNodeActionInfo,
  CosmosEngineConfig,
  CosmosEngineCallbacks,
  TopoGraphViewState,
} from './types'
import { computeGraphvizLayout } from './graphvizLayout'
import { createTopoHashColorResolver, getTopoTypeColorSeed, resolveTopoVisualIdentity } from './topoVisualIdentity'

/* ═══════════════════════════════════════════════════════════ */

const BG_COLOR = '#ffffff'

const POINT_SIZE_BASE = 16
const POINT_SIZE_MAX = 38
const IMAGE_SIZE = 32
const IMAGE_VISIBILITY_MIN_SIZE = 8.5

const LINK_DEFAULT_COLOR = '#8fa1b7'
const LINK_WIDTH = 1.35
const LINK_HOVER_WIDTH = 4.2
const LINK_OPACITY = 0.5
const LINK_ACTIVE_OPACITY = 0.96
const LINK_GREYOUT_OPACITY = 0.035

const HOVER_RING_COLOR = '#5b5bd6'
const FOCUSED_RING_COLOR = '#5b5bd6'

const HOVER_ENTER_DELAY = 80
const HOVER_LEAVE_DELAY = 260
const HOVER_FADE_STEPS = 6
const LARGE_HOVER_FOCUS_THRESHOLD = 800
const FORCE_SIMULATION_DURATION = 5000
const DEFAULT_MIN_ZOOM_LEVEL = 0.04
const DEFAULT_MAX_ZOOM_LEVEL = 8
const LOCKED_NODE_CANCEL_RADIUS = 26
const LOCKED_LINK_CANCEL_DISTANCE = 10
const LINK_CLICK_HIT_DISTANCE = 6
const CURVED_LINK_WEIGHT = 0.5
const CURVED_LINK_CONTROL_POINT_DISTANCE = 0.25
const CURVED_LINK_HIT_SEGMENTS = 18
const COSMOS_SPACE_SIZE = 8192
const COSMOS_SPACE_CENTER = COSMOS_SPACE_SIZE / 2
const FORCE_SPACE_MARGIN = 640
const GOLDEN_ANGLE = 2.399963229728653
const TIMELINE_ACTIVE_NODE_OPACITY = 1
const TIMELINE_PAST_NODE_OPACITY = 0.08
const TIMELINE_FUTURE_NODE_OPACITY = 0.035
const TIMELINE_ACTIVE_EDGE_OPACITY = 0.72
const TIMELINE_PAST_EDGE_OPACITY = 0.035
const TIMELINE_FUTURE_EDGE_OPACITY = 0.018

export const SIM_DEFAULTS = {
  friction: 0.88,
  gravity: 0.08,
  repulsion: 2.4,
  decay: 7000,
  linkSpring: 0.38,
  linkDistance: 48,
  cluster: 0.85,
  center: 0.04,
}

function getForceLayoutSpacing(total: number): number {
  if (total >= 8000) return 34
  if (total >= 2500) return 42
  if (total >= 1000) return 52
  if (total >= 500) return 50
  if (total >= 160) return 42
  if (total >= 40) return 34
  return 28
}

function getInitialVisualZoomFactor(total: number): number {
  if (total <= 12) return 4.4
  if (total <= 24) return 4.0
  if (total <= 120) return 3.25
  if (total <= 240) return 2.0
  if (total <= 500) return 1.45
  return 1
}

/* ═══════════════════════════════════════════════════════════ */

function parseColor(color: string | undefined): [number, number, number] {
  if (!color) return [0.5, 0.5, 0.5]
  if (color.startsWith('#')) {
    const c = color.replace('#', '')
    return [
      (parseInt(c.slice(0, 2), 16) || 0) / 255,
      (parseInt(c.slice(2, 4), 16) || 0) / 255,
      (parseInt(c.slice(4, 6), 16) || 0) / 255,
    ]
  }
  const m = color.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/)
  if (m) return [+m[1] / 255, +m[2] / 255, +m[3] / 255]
  return [0.5, 0.5, 0.5]
}

function ensureVisibleOnLight(r: number, g: number, b: number): [number, number, number] {
  const lum = 0.299 * r + 0.587 * g + 0.114 * b
  if (lum > 0.78) return [r * 0.6, g * 0.6, b * 0.6]
  return [r, g, b]
}

function hashToUnit(value: string): number {
  let hash = 2166136261
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return (hash >>> 0) / 4294967295
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

function readOpacity(...values: any[]): number {
  for (const value of values) {
    if (value === undefined || value === null || value === '') continue
    const numeric = Number(value)
    if (Number.isFinite(numeric)) return clamp(numeric, 0, 1)
  }
  return 1
}

function smoothstep(value: number): number {
  const t = clamp(value, 0, 1)
  return t * t * (3 - 2 * t)
}

function roundToHalf(value: number): number {
  return Math.round(value * 2) / 2
}

function interpolate(start: number, end: number, progress: number): number {
  return start + (end - start) * clamp(progress, 0, 1)
}

function normalizeTimelinePresence(value: any): number[] | undefined {
  if (!Array.isArray(value)) return undefined
  const items = value
    .map((item) => Number(item))
    .filter((item) => Number.isFinite(item))
    .sort((a, b) => a - b)
  return items.length > 0 ? items : undefined
}

function timelinePresenceHas(presence: number[] | undefined, index: number): boolean {
  if (!presence) return false
  return presence.includes(index)
}

function getTimelineState(
  presence: number[] | undefined,
  playhead: number,
): 'active' | 'past' | 'future' | undefined {
  if (!presence || presence.length === 0) return undefined
  const currentIndex = Math.floor(playhead)
  if (timelinePresenceHas(presence, currentIndex) || timelinePresenceHas(presence, currentIndex + 1)) return 'active'
  const first = presence[0]
  const last = presence[presence.length - 1]
  return currentIndex > last ? 'past' : currentIndex < first ? 'future' : 'past'
}

function getTimelineOpacity(
  id: string,
  presence: number[] | undefined,
  playhead: number | undefined,
  opacityMeta: { active: number; past: number; future: number },
): number | undefined {
  if (!presence || !Number.isFinite(Number(playhead))) return undefined
  const safePlayhead = Number(playhead)
  const baseIndex = Math.floor(safePlayhead)
  const nextIndex = baseIndex + 1
  const progress = clamp(safePlayhead - baseIndex, 0, 1)
  const inBase = timelinePresenceHas(presence, baseIndex)
  const inNext = timelinePresenceHas(presence, nextIndex)

  if (inBase && inNext) return opacityMeta.active
  if (inBase && !inNext) {
    const stagger = hashToUnit(`${id}:exit`) * 0.56
    const local = smoothstep((progress - stagger) / 0.42)
    return interpolate(opacityMeta.active, opacityMeta.past, local)
  }
  if (!inBase && inNext) {
    const stagger = hashToUnit(`${id}:enter`) * 0.56
    const local = smoothstep((progress - stagger) / 0.42)
    return interpolate(opacityMeta.future, opacityMeta.active, local)
  }

  const state = getTimelineState(presence, safePlayhead)
  return state ? opacityMeta[state] : undefined
}

function distanceToSegment(point: [number, number], start: [number, number], end: [number, number]): number {
  const vx = end[0] - start[0]
  const vy = end[1] - start[1]
  const wx = point[0] - start[0]
  const wy = point[1] - start[1]
  const lenSq = vx * vx + vy * vy
  if (lenSq === 0) return Math.hypot(point[0] - start[0], point[1] - start[1])
  const t = Math.max(0, Math.min(1, (wx * vx + wy * vy) / lenSq))
  const px = start[0] + t * vx
  const py = start[1] + t * vy
  return Math.hypot(point[0] - px, point[1] - py)
}

function conicCurvePoint(
  start: [number, number],
  end: [number, number],
  control: [number, number],
  t: number,
  weight: number,
): [number, number] {
  const oneMinusT = 1 - t
  const weighted = 2 * oneMinusT * t * weight
  const divisor = oneMinusT * oneMinusT + weighted + t * t
  return [
    (oneMinusT * oneMinusT * start[0] + weighted * control[0] + t * t * end[0]) / divisor,
    (oneMinusT * oneMinusT * start[1] + weighted * control[1] + t * t * end[1]) / divisor,
  ]
}

function distanceToRenderedLink(point: [number, number], start: [number, number], end: [number, number]): number {
  const dx = end[0] - start[0]
  const dy = end[1] - start[1]
  const distance = Math.hypot(dx, dy)
  if (distance <= 0.001) return Math.hypot(point[0] - start[0], point[1] - start[1])

  const control: [number, number] = [
    (start[0] + end[0]) / 2 - (dy / distance) * distance * CURVED_LINK_CONTROL_POINT_DISTANCE,
    (start[1] + end[1]) / 2 + (dx / distance) * distance * CURVED_LINK_CONTROL_POINT_DISTANCE,
  ]

  let best = Infinity
  let previous = start
  for (let i = 1; i <= CURVED_LINK_HIT_SEGMENTS; i += 1) {
    const current = conicCurvePoint(start, end, control, i / CURVED_LINK_HIT_SEGMENTS, CURVED_LINK_WEIGHT)
    best = Math.min(best, distanceToSegment(point, previous, current))
    previous = current
  }
  return best
}

function buildHexOffsets(count: number, spacing: number): [number, number][] {
  if (count <= 0) return []
  const offsets: [number, number][] = [[0, 0]]
  const directions: Array<[number, number]> = [
    [0, -1],
    [-1, 0],
    [-1, 1],
    [0, 1],
    [1, 0],
    [1, -1],
  ]

  for (let ring = 1; offsets.length < count; ring += 1) {
    let q = ring
    let r = 0
    for (const [dq, dr] of directions) {
      for (let step = 0; step < ring && offsets.length < count; step += 1) {
        offsets.push([
          spacing * (q + r / 2),
          spacing * (Math.sqrt(3) / 2) * r,
        ])
        q += dq
        r += dr
      }
    }
  }

  return offsets.slice(0, count)
}

function relaxCircleCenters(
  centers: Map<string, [number, number]>,
  radii: Map<string, number>,
  padding: number,
): void {
  const keys = Array.from(centers.keys())
  for (let iter = 0; iter < 220; iter += 1) {
    let maxShift = 0
    for (let i = 0; i < keys.length; i += 1) {
      for (let j = i + 1; j < keys.length; j += 1) {
        const a = centers.get(keys[i])!
        const b = centers.get(keys[j])!
        let dx = b[0] - a[0]
        let dy = b[1] - a[1]
        let distance = Math.hypot(dx, dy)
        if (distance < 0.001) {
          const angle = (i + j + 1) * GOLDEN_ANGLE
          dx = Math.cos(angle)
          dy = Math.sin(angle)
          distance = 1
        }
        const minDistance = (radii.get(keys[i]) ?? 0) + (radii.get(keys[j]) ?? 0) + padding
        if (distance >= minDistance) continue
        const shift = (minDistance - distance) * 0.5
        const ux = dx / distance
        const uy = dy / distance
        a[0] -= ux * shift
        a[1] -= uy * shift
        b[0] += ux * shift
        b[1] += uy * shift
        maxShift = Math.max(maxShift, shift)
      }
    }

    let cx = 0
    let cy = 0
    keys.forEach((key) => {
      const center = centers.get(key)!
      cx += center[0]
      cy += center[1]
    })
    cx /= Math.max(1, keys.length)
    cy /= Math.max(1, keys.length)
    keys.forEach((key) => {
      const center = centers.get(key)!
      center[0] -= cx
      center[1] -= cy
    })

    if (maxShift < 0.2) break
  }
}

function packCircleCentersByGrid(
  centers: Map<string, [number, number]>,
  radii: Map<string, number>,
  padding: number,
): void {
  const keys = Array.from(radii.keys()).sort((a, b) => (radii.get(b) ?? 0) - (radii.get(a) ?? 0) || a.localeCompare(b))
  if (!keys.length) return

  let area = 0
  keys.forEach((key) => {
    const radius = (radii.get(key) ?? 0) + padding * 0.5
    area += Math.PI * radius * radius
  })

  const targetWidth = Math.max(800, Math.sqrt(area) * 1.25)
  let cursorX = 0
  let cursorY = 0
  let rowHeight = 0

  keys.forEach((key) => {
    const radius = (radii.get(key) ?? 0) + padding * 0.5
    const diameter = radius * 2
    if (cursorX > 0 && cursorX + diameter > targetWidth) {
      cursorX = 0
      cursorY += rowHeight
      rowHeight = 0
    }

    centers.set(key, [cursorX + radius, cursorY + radius])
    cursorX += diameter
    rowHeight = Math.max(rowHeight, diameter)
  })

  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  centers.forEach(([x, y]) => {
    minX = Math.min(minX, x)
    maxX = Math.max(maxX, x)
    minY = Math.min(minY, y)
    maxY = Math.max(maxY, y)
  })
  const cx = (minX + maxX) / 2
  const cy = (minY + maxY) / 2
  centers.forEach((center) => {
    center[0] -= cx
    center[1] -= cy
  })
}

/* ═══════════════════════════════════════════════════════════ */

export class CosmosEngine {
  private graph: Graph | null = null
  private container: HTMLDivElement
  private callbacks: CosmosEngineCallbacks
  private destroyed = false

  private nodes: NodeRecord[] = []
  private rawNodes: TopoNode[] = []
  private edges: TopoEdge[] = []
  private nodeById = new Map<string, NodeRecord>()
  private rawNodeById = new Map<string, TopoNode>()
  private nodeIdToIndex = new Map<string, number>()
  private indexToNodeId = new Map<number, string>()
  private neighbors = new Map<string, Set<string>>()
  private clusters: ClusterRecord[] = []
  private clusterByKey = new Map<string, ClusterRecord>()

  private pointBaseColors: Float32Array = new Float32Array(0)
  private pointBaseRgb: Float32Array = new Float32Array(0)
  private pointPositions: Float32Array = new Float32Array(0)
  private pointColors: Float32Array = new Float32Array(0)
  private pointSizes: Float32Array = new Float32Array(0)
  private pointImageIndices: Float32Array = new Float32Array(0)
  private pointImageSizes: Float32Array = new Float32Array(0)
  private linkColors: Float32Array = new Float32Array(0)
  private linkBaseColors: Float32Array = new Float32Array(0)
  private linkBaseOpacities: Float32Array = new Float32Array(0)
  private linkWidths: Float32Array = new Float32Array(0)
  private validEdgeIndices: number[] = []
  private edgeTimelinePresence: Array<number[] | undefined> = []
  private clusterPositions = new Map<string, [number, number]>()
  private clusterLabelPositions = new Map<string, [number, number]>()
  private graphvizLayoutRunId = 0

  private hover: HoverState = {
    hoveredNodeId: undefined,
    hoveredNeighborIds: undefined,
    hoveredLinkIndex: undefined,
    lockedLinkIndex: undefined,
    lockedNodeId: undefined,
    hoverProgress: 0,
  }
  private hoverTimer: ReturnType<typeof setTimeout> | null = null
  private hoverFadeRaf: number | null = null
  private hoverFadeStep = 0
  private hoverTrackRafId: number | null = null
  private pendingHoverTrackPoint: [number, number] | null = null
  private activeFilterKeys: Set<string> | null = null
  private currentLayout: LayoutOptions = {}
  private zoomLevel = 1
  private labelSyncRafId: number | null = null
  private visualScaleRafId: number | null = null
  private lastVisualScaleKey = ''
  private currentPointScreenSize = POINT_SIZE_BASE
  private zoomBaseline = 0
  private lastPointerPosition: [number, number] | undefined
  private dragState: { index: number; pointerId: number; start: [number, number]; moved: boolean } | null = null
  private lockedBackgroundPress: { pointerId: number; start: [number, number]; moved: boolean } | null = null
  private suppressNextClick = false
  private ignoreNextLockedBackgroundClick = false
  private ignoreNextLockedBackgroundClickTimer: ReturnType<typeof setTimeout> | null = null
  private forceStopTimer: ReturnType<typeof setTimeout> | null = null
  private zoomBaselineTimer: ReturnType<typeof setTimeout> | null = null
  private forceRunId = 0

  constructor(container: HTMLDivElement, config?: CosmosEngineConfig) {
    this.container = container
    this.callbacks = config?.callbacks ?? {}
    this.container.addEventListener('pointerdown', this.handlePointerDown, true)
    this.container.addEventListener('pointermove', this.handlePointerTrack, { passive: true })
  }

  /* ─────────── Public API ─────────── */

  loadData(data: TopoData, layout: LayoutOptions): void {
    this.currentLayout = layout
    this.zoomBaseline = 0
    this.buildIndex(data)
    if (this.isRelationsMode(layout)) this.computeRelationPositions(data)
    this.destroyGraph()
    this.createGraph(layout)
    this.pushAllBuffers()
    if (this.isForceMode(layout)) {
      this.renderStaticForceLayout()
    } else {
      this.graph!.render(0)
      this.syncClusterLabelPositionsFromRenderedPoints()
    }
    this.applyAsyncGraphvizLayout(data, layout)
    this.scheduleLabelSync()
  }

  updateData(data: TopoData): void {
    this.clearTransientStateForDataChange()
    this.clearZoomBaselineTimer()
    const previousPositions = this.captureNodePositions()
    const preserveView = Boolean(this.currentLayout.preserveViewOnDataUpdate)
    const skipFitView = Boolean(this.currentLayout.skipFitViewOnDataUpdate)
    this.buildIndex(data)
    if (preserveView) this.restoreStableNodePositions(previousPositions)
    if (!preserveView && this.isRelationsMode(this.currentLayout)) this.computeRelationPositions(data)
    if (!preserveView) this.zoomBaseline = 0
    if (!this.graph) return
    this.pushAllBuffers(preserveView)
    if (this.isForceMode(this.currentLayout)) {
      if (preserveView) {
        this.graph.render(0)
        this.syncClusterLabelPositionsFromRenderedPoints()
      }
      else this.renderStaticForceLayout(300, !skipFitView)
    } else {
      this.graph.render(0)
      this.syncClusterLabelPositionsFromRenderedPoints()
    }
    if (!preserveView) {
      this.applyAsyncGraphvizLayout(data, this.currentLayout)
      if (!skipFitView) this.fitView(300)
    }
    const playhead = Number(this.currentLayout.timelinePlayhead)
    if (Number.isFinite(playhead) && this.hasTimelinePresence()) {
      this.applyTimelinePlayhead(playhead, false)
    }
    this.scheduleLabelSync()
  }

  setRuntimeLayout(layout: LayoutOptions): void {
    const previousPlayhead = this.currentLayout.timelinePlayhead
    const previousTimelinePlaying = this.currentLayout.timelinePlaying === true
    const previousForceDetailedView = this.currentLayout.forceDetailedView === true
    this.currentLayout = layout
    const nextForceDetailedView = layout.forceDetailedView === true
    if (previousForceDetailedView !== nextForceDetailedView) {
      this.zoomBaseline = 0
      this.lastVisualScaleKey = ''
    }
    this.applyZoomBounds()
    const nextPlayhead = layout.timelinePlayhead
    const nextTimelinePlaying = layout.timelinePlaying === true
    if (
      Number.isFinite(Number(previousPlayhead)) &&
      !Number.isFinite(Number(nextPlayhead)) &&
      this.hasTimelinePresence()
    ) {
      this.resetTimelineVisualState()
      return
    }
    if (Number.isFinite(Number(nextPlayhead)) && nextPlayhead !== previousPlayhead && this.hasTimelinePresence()) {
      this.applyTimelinePlayhead(Number(nextPlayhead), !nextTimelinePlaying || previousTimelinePlaying !== nextTimelinePlaying)
      return
    }
    if (previousTimelinePlaying !== nextTimelinePlaying && Number.isFinite(Number(nextPlayhead)) && this.hasTimelinePresence()) {
      this.applyTimelinePlayhead(Number(nextPlayhead), true)
      return
    }
    this.scheduleVisualScaleSync()
  }

  updateLayout(layout: LayoutOptions): void {
    this.currentLayout = layout
    if (!this.graph) return
    const nodeCount = this.nodes.length
    const forceRepulsion = nodeCount >= 2500 ? 8.5 : nodeCount >= 800 ? 6.2 : SIM_DEFAULTS.repulsion
    const forceGravity = nodeCount >= 2500 ? 0.025 : nodeCount >= 800 ? 0.04 : SIM_DEFAULTS.gravity
    this.graph.setConfig({
      simulationFriction: layout.simulationFriction ?? SIM_DEFAULTS.friction,
      simulationGravity: layout.simulationGravity ?? forceGravity,
      simulationRepulsion: layout.simulationRepulsion ?? forceRepulsion,
      enableDrag: layout.enableDrag === true,
    })
    this.applyZoomBounds()
    this.applyAdaptiveNodeSizes(true)
    if (this.isRelationsMode(layout)) {
      this.computeRelationPositions({ nodes: this.rawNodes, edges: this.edges })
      this.buildPointPositions()
      this.graph.render(0)
      this.syncClusterLabelPositionsFromRenderedPoints()
      this.fitView(300)
      this.scheduleLabelSync()
    } else if (this.isForceMode(layout)) {
      if (this.shouldClusterByType(layout)) this.computeClusteredPositions({ nodes: this.rawNodes, edges: this.edges })
      else this.computeForcePositions({ nodes: this.rawNodes, edges: this.edges })
      this.buildPointPositions(true)
      this.renderStaticForceLayout(300)
    }
    else {
      this.graph.render(0)
      this.syncClusterLabelPositionsFromRenderedPoints()
      this.applyAsyncGraphvizLayout({ nodes: this.rawNodes, edges: this.edges }, layout)
    }
  }

  highlightNode(nodeId: string | null): void {
    if (!this.graph) return
    if (!nodeId) {
      this.graph.unselectPoints()
      this.graph.setConfig({ focusedPointIndex: undefined })
      return
    }
    const idx = this.nodeIdToIndex.get(nodeId)
    if (idx === undefined) return
    this.graph.unselectPoints()
    this.graph.setConfig({ focusedPointIndex: undefined })
  }

  filterByClusters(keys: string[] | null): void {
    if (!this.graph) return
    if (!keys || keys.length === 0) {
      this.activeFilterKeys = null
      this.graph.setPointColors(this.pointColors)
      this.graph.setLinkColors(this.linkColors)
      this.graph.render()
      this.scheduleLabelSync()
      return
    }
    const activeSet = new Set(keys)
    this.activeFilterKeys = activeSet

    const pc = new Float32Array(this.pointColors)
    for (let i = 0; i < this.nodes.length; i++) {
      if (!activeSet.has(this.nodes[i].cluster)) pc[i * 4 + 3] = 0
    }
    this.graph.setPointColors(pc)

    const lc = new Float32Array(this.linkColors)
    for (let i = 0; i < this.validEdgeIndices.length; i++) {
      const eIdx = this.validEdgeIndices[i]
      const edge = this.edges[eIdx]
      const sn = this.nodeById.get(edge.source)
      const tn = this.nodeById.get(edge.target)
      if (!sn || !tn || !activeSet.has(sn.cluster) || !activeSet.has(tn.cluster)) {
        lc[i * 4 + 3] = 0
      }
    }
    this.graph.setLinkColors(lc)
    this.graph.render()
    this.scheduleLabelSync()
  }

  fitView(duration = 250): void {
    this.zoomBaseline = 0
    this.graph?.fitView(duration, this.currentLayout.forceDetailedView ? 0.035 : 0.1)
    this.scheduleZoomBaselineStabilization(duration)
  }

  fitViewByNodeIds(ids: string[], duration = 300): void {
    if (!this.graph) return
    const indices = ids.map(id => this.nodeIdToIndex.get(id)).filter((i): i is number => i !== undefined)
    this.zoomBaseline = 0
    if (indices.length) this.graph.fitViewByPointIndices(indices, duration, 0.12)
    else this.graph.fitView(duration, 0.1)
    this.scheduleZoomBaselineStabilization(duration)
  }

  zoomToNode(nodeId: string, duration = 500): void {
    if (!this.graph) return
    const idx = this.nodeIdToIndex.get(nodeId)
    if (idx !== undefined) this.graph.zoomToPointByIndex(idx, duration, 3)
  }

  centerViewAtSpacePosition(position: [number, number], duration = 160): void {
    if (!this.graph) return
    const [minZoom, maxZoom] = this.getZoomBounds()
    const zoom = clamp(this.graph.getZoomLevel(), minZoom, maxZoom)
    const graphAny = this.graph as any
    if (typeof graphAny.setZoomTransformByPointPositions === 'function') {
      graphAny.setZoomTransformByPointPositions([position[0], position[1]], duration, zoom)
    } else {
      this.zoomToNearestPoint(position, duration, zoom)
    }
    this.zoomLevel = zoom
    this.scheduleVisualScaleSync()
    this.scheduleLabelSync()
  }

  centerNodeView(nodeId: string, duration = 240): void {
    if (!this.graph) return
    const index = this.nodeIdToIndex.get(nodeId)
    if (index === undefined) return
    const position = this.getViewNodePosition(index)
    const x = position?.[0]
    const y = position?.[1]
    if (x === undefined || y === undefined || !Number.isFinite(x) || !Number.isFinite(y)) return
    this.centerViewAtSpacePosition([x, y], duration)
  }

  getZoomLevel(): number { return this.zoomLevel }
  getViewState(): TopoGraphViewState | null {
    if (!this.graph) return null
    let zoomLevel = this.zoomLevel
    try { zoomLevel = this.graph.getZoomLevel() } catch { /* Keep the last tracked zoom. */ }
    const transform = (this.graph as any)?.zoomInstance?.eventTransform
    const hasTransform = (
      Number.isFinite(Number(transform?.x)) &&
      Number.isFinite(Number(transform?.y)) &&
      Number.isFinite(Number(transform?.k))
    )
    return {
      zoomLevel,
      zoomBaseline: this.zoomBaseline > 0 ? this.zoomBaseline : zoomLevel,
      transform: hasTransform
        ? { x: Number(transform.x), y: Number(transform.y), k: Number(transform.k) }
        : undefined,
    }
  }

  restoreViewState(state: TopoGraphViewState | null | undefined, duration = 0): void {
    if (!this.graph || !state) return
    const [minZoom, maxZoom] = this.getZoomBounds()
    const nextZoom = clamp(Number(state.transform?.k ?? state.zoomLevel), minZoom, maxZoom)
    const graphAny = this.graph as any
    const zoomInstance = graphAny.zoomInstance
    const selection = graphAny.canvasD3Selection
    const behavior = zoomInstance?.behavior
    const currentTransform = zoomInstance?.eventTransform

    if (
      state.transform &&
      selection &&
      behavior?.transform &&
      currentTransform?.constructor &&
      Number.isFinite(nextZoom)
    ) {
      try {
        const nextTransform = new currentTransform.constructor(
          nextZoom,
          Number(state.transform.x),
          Number(state.transform.y),
        )
        if (duration > 0 && typeof selection.transition === 'function') {
          selection.transition().duration(duration).call(behavior.transform, nextTransform)
        } else {
          selection.call(behavior.transform, nextTransform)
        }
      } catch {
        this.graph.setZoomLevel(nextZoom, duration)
      }
    } else if (Number.isFinite(nextZoom)) {
      this.graph.setZoomLevel(nextZoom, duration)
    }

    this.zoomLevel = Number.isFinite(nextZoom) ? nextZoom : this.zoomLevel
    const nextBaseline = Number(state.zoomBaseline)
    this.zoomBaseline = Number.isFinite(nextBaseline) && nextBaseline > 0
      ? clamp(nextBaseline, minZoom, maxZoom)
      : this.zoomLevel
    this.lastVisualScaleKey = ''
    this.applyAdaptiveNodeSizes(true)
    this.scheduleRestoredViewVisualSync(duration)
    this.scheduleLabelSync()
  }

  getGraph(): Graph | null { return this.graph }
  getNodes(): readonly NodeRecord[] { return this.nodes }
  getRawNode(id: string): TopoNode | undefined { return this.rawNodeById.get(id) }
  getClusters(): readonly ClusterRecord[] { return this.clusters }
  getHoverState(): HoverState { return this.hover }
  getNodeIdToIndex(): ReadonlyMap<string, number> { return this.nodeIdToIndex }
  getNodeById(): ReadonlyMap<string, NodeRecord> { return this.nodeById }
  getNeighbors(): ReadonlyMap<string, Set<string>> { return this.neighbors }
  getClusterPositionsMap(): ReadonlyMap<string, [number, number]> { return this.clusterPositions }
  getClusterLabelPositionsMap(): ReadonlyMap<string, [number, number]> { return this.clusterLabelPositions }
  getLayout(): LayoutOptions { return this.currentLayout }
  getEdges(): readonly TopoEdge[] { return this.edges }
  getValidEdgeIndices(): readonly number[] { return this.validEdgeIndices }

  getPointPositionsForRender(): ArrayLike<number> | null {
    if (this.canUseCachedHitPositions()) {
      if (this.pointPositions.length === this.nodes.length * 2) return this.pointPositions
      const positions = new Float32Array(this.nodes.length * 2)
      this.nodes.forEach((node, index) => {
        positions[index * 2] = node.x
        positions[index * 2 + 1] = node.y
      })
      this.pointPositions = positions
      return positions
    }

    if (!this.graph) return null
    try { return this.graph.getPointPositions() } catch { return null }
  }

  getVectorIconOverlays(maxItems = 5000): CosmosVectorIconOverlayItem[] {
    if (!this.graph || this.nodes.length === 0) return []

    const positions = this.getPointPositionsForRender()
    if (!positions) return []
    if (!positions.length) return []

    const width = this.container.clientWidth
    const height = this.container.clientHeight
    if (!width || !height) return []

    const profile = this.getAdaptiveNodeProfile()
    const margin = 64
    const focusNodeId = this.hover.lockedNodeId
    const focusNeighborIds = this.hover.hoveredNeighborIds
    const focusLinkIndex = this.hover.lockedLinkIndex
    let linkSourceId: string | undefined
    let linkTargetId: string | undefined
    if (focusLinkIndex !== undefined) {
      const edge = this.edges[this.validEdgeIndices[focusLinkIndex]]
      linkSourceId = edge?.source
      linkTargetId = edge?.target
    }
    const hasFocus = (
      Boolean(focusNodeId) ||
      Boolean(linkSourceId && linkTargetId)
    )

    const overlays: CosmosVectorIconOverlayItem[] = []
    for (const node of this.nodes) {
      if (this.activeFilterKeys && !this.activeFilterKeys.has(node.cluster)) continue
      const screenSize = node.screenSize || profile.pointSize
      if (!node.vectorIconVisible) continue
      if (node.index * 2 + 1 >= positions.length) continue
      const baseOpacity = clamp(node.opacity ?? 1, 0, 1)
      if (baseOpacity <= 0.025) continue

      let screen: [number, number]
      try {
        screen = this.graph.spaceToScreenPosition([positions[node.index * 2], positions[node.index * 2 + 1]])
      } catch {
        continue
      }

      if (!Number.isFinite(screen[0]) || !Number.isFinite(screen[1])) continue
      if (screen[0] < -margin || screen[0] > width + margin || screen[1] < -margin || screen[1] > height + margin) continue

      let opacity = baseOpacity
      let active = false
      let related = false
      if (hasFocus) {
        const isFocusedNode = node.id === focusNodeId
        const isFocusNeighbor = focusNeighborIds?.has(node.id)
        const isFocusedLinkEndpoint = node.id === linkSourceId || node.id === linkTargetId
        active = Boolean(isFocusedNode || isFocusedLinkEndpoint)
        related = Boolean(isFocusNeighbor)
        opacity = baseOpacity * (isFocusedNode || isFocusedLinkEndpoint ? 1 : isFocusNeighbor ? 0.92 : 0.18)
      }

      overlays.push({
        id: node.id,
        x: screen[0],
        y: screen[1],
        size: Math.max(18, Math.round(screenSize)),
        color: node.iconFill || node.color,
        opacity,
        label: node.title,
        typeLabel: node.subTitle || node.cluster,
        domain: node.cluster.split('@')[0] || undefined,
        iconClass: node.iconClass,
        iconUrl: node.iconUrl,
        iconPreset: node.iconPreset,
        lucideIcon: node.lucideIcon,
        ringWidth: node.ringWidth,
        active,
        related,
      })
    }

    if (overlays.length <= maxItems) return overlays
    return overlays
      .sort((left, right) => right.size - left.size || right.opacity - left.opacity || left.id.localeCompare(right.id))
      .slice(0, maxItems)
  }

  getLockedNodeAction(): CosmosNodeActionInfo | null {
    const nodeId = this.hover.lockedNodeId
    if (!nodeId || !this.graph) return null

    const node = this.nodeById.get(nodeId)
    if (!node || !node.vectorIconVisible) return null

    const positions = this.getPointPositionsForRender()
    if (!positions) return null
    if (node.index * 2 + 1 >= positions.length) return null

    let screen: [number, number]
    try {
      screen = this.graph.spaceToScreenPosition([positions[node.index * 2], positions[node.index * 2 + 1]])
    } catch {
      return null
    }

    const width = this.container.clientWidth
    const height = this.container.clientHeight
    const margin = 72
    if (
      !Number.isFinite(screen[0]) ||
      !Number.isFinite(screen[1]) ||
      screen[0] < -margin ||
      screen[0] > width + margin ||
      screen[1] < -margin ||
      screen[1] > height + margin
    ) {
      return null
    }

    return {
      nodeId,
      x: screen[0],
      y: screen[1],
      size: Math.max(18, Math.round(node.screenSize || this.currentPointScreenSize || IMAGE_SIZE)),
      color: node.iconFill || node.color,
    }
  }

  clearHover(): void {
    this.cancelHoverTimer()
    this.cancelHoverFade()
    this.hover.lockedNodeId = undefined
    this.hover.lockedLinkIndex = undefined
    this.hover.hoveredNodeId = undefined
    this.hover.hoveredNeighborIds = undefined
    this.hover.hoveredLinkIndex = undefined
    this.hover.hoverProgress = 0
    this.hover.screenPosition = undefined
    this.resetVisualState()
    this.callbacks.onHoverChange?.(this.hover)
    this.scheduleLabelSync()
  }

  lockNode(nodeId: string): void {
    this.cancelHoverTimer()
    this.cancelHoverFade()
    this.hover.lockedNodeId = nodeId
    this.hover.lockedLinkIndex = undefined
    this.hover.hoveredLinkIndex = undefined
    this.hover.hoveredNodeId = nodeId
    this.hover.hoveredNeighborIds = this.neighbors.get(nodeId) ?? new Set()
    this.hover.hoverProgress = 1
    this.highlightNode(nodeId)
    this.applyHoverColors(1.0)
    this.callbacks.onHoverChange?.(this.hover)
    this.scheduleLabelSync()
  }

  lockLink(linkIndex: number): void {
    this.cancelHoverTimer()
    this.cancelHoverFade()
    this.hover.lockedNodeId = undefined
    this.hover.hoveredNodeId = undefined
    this.hover.hoveredNeighborIds = undefined
    this.hover.lockedLinkIndex = linkIndex
    this.hover.hoveredLinkIndex = linkIndex
    this.hover.hoverProgress = 1
    this.graph?.unselectPoints()
    this.graph?.setConfig({ focusedPointIndex: undefined })
    this.applyLinkFocus(linkIndex, 1)
    this.callbacks.onHoverChange?.(this.hover)
    this.scheduleLabelSync()
  }

  getFocusedEdges(limit = 12): FocusedEdgeRecord[] {
    const focusedLinkIndex = this.hover.lockedLinkIndex ?? this.hover.hoveredLinkIndex
    if (focusedLinkIndex !== undefined) {
      const focused = this.getFocusedEdgeByLinkIndex(focusedLinkIndex)
      return focused ? [focused] : []
    }

    const focusedNodeId = this.hover.hoveredNodeId || this.hover.lockedNodeId
    if (!focusedNodeId || this.hover.hoverProgress <= 0) return []

    const result: FocusedEdgeRecord[] = []
    for (let linkIndex = 0; linkIndex < this.validEdgeIndices.length; linkIndex += 1) {
      const edgeIndex = this.validEdgeIndices[linkIndex]
      const edge = this.edges[edgeIndex]
      if (!edge || (edge.source !== focusedNodeId && edge.target !== focusedNodeId)) continue
      const focused = this.getFocusedEdgeByLinkIndex(linkIndex)
      if (focused) result.push(focused)
      if (result.length >= limit) break
    }
    return result
  }

  destroy(): void {
    if (this.destroyed) return
    this.destroyed = true
    this.cancelHoverTimer()
    this.cancelHoverFade()
    this.clearForceStopTimer()
    this.clearZoomBaselineTimer()
    if (this.labelSyncRafId !== null) cancelAnimationFrame(this.labelSyncRafId)
    if (this.hoverTrackRafId !== null) cancelAnimationFrame(this.hoverTrackRafId)
    this.container.removeEventListener('pointerdown', this.handlePointerDown, true)
    this.container.removeEventListener('pointermove', this.handlePointerTrack)
    this.stopManualDragListeners()
    this.stopLockedBackgroundPressListeners()
    if (this.ignoreNextLockedBackgroundClickTimer !== null) clearTimeout(this.ignoreNextLockedBackgroundClickTimer)
    if (this.visualScaleRafId !== null) cancelAnimationFrame(this.visualScaleRafId)
    this.destroyGraph()
  }

  /* ─────────── Internal: Graph lifecycle ─────────── */

  private destroyGraph(): void {
    this.clearForceStopTimer()
    this.clearZoomBaselineTimer()
    if (this.graph) { this.graph.destroy(); this.graph = null }
  }

  private isForceMode(layout: LayoutOptions): boolean {
    return layout.mode === 'force' || layout.mode === 'clustered'
  }

  private isGraphvizMode(layout: LayoutOptions): boolean {
    return layout.mode === 'graphviz' || layout.mode === 'graphvizFree'
  }

  private isRelationsMode(layout: LayoutOptions): boolean {
    return layout.mode === 'relations'
  }

  private shouldClusterByType(layout: LayoutOptions): boolean {
    if (typeof layout.clusterByType === 'boolean') return layout.clusterByType
    return layout.mode === 'clustered' || layout.mode === 'graphviz'
  }

  private shouldFocusOnHover(): boolean {
    if (this.currentLayout.hoverMode === 'focus') return true
    if (this.currentLayout.hoverMode === 'panel') return false
    return this.nodes.length <= LARGE_HOVER_FOCUS_THRESHOLD
  }

  private startForceSimulation(layout: LayoutOptions): void {
    if (!this.graph) return
    this.clearForceStopTimer()
    const runId = ++this.forceRunId
    this.zoomBaseline = 0
    this.graph.render(0)
    this.syncClusterLabelPositionsFromRenderedPoints()
    this.graph.fitView(220, 0.16)
    this.scheduleZoomBaselineStabilization(220)
    this.graph.start(1)
    this.graph.render(1)
    const duration = Math.max(500, layout.simulationDuration ?? FORCE_SIMULATION_DURATION)
    this.forceStopTimer = setTimeout(() => this.freezeForceSimulation(runId), duration)
  }

  private renderStaticForceLayout(duration = 260, shouldFitView = true): void {
    if (!this.graph) return
    this.clearForceStopTimer()
    ++this.forceRunId
    this.zoomBaseline = 0
    this.graph.stop()
    this.graph.render(0)
    this.syncClusterLabelPositionsFromRenderedPoints()
    if (shouldFitView) this.fitView(duration)
    this.applyAdaptiveNodeSizes(true)
    this.scheduleLabelSync()
    this.callbacks.onSimulationEnd?.()
  }

  private freezeForceSimulation(runId = this.forceRunId): void {
    if (!this.graph || runId !== this.forceRunId) return
    this.clearForceStopTimer()
    this.graph.stop()
    this.syncNodePositionsFromGraph()
    this.buildPointPositions(true)
    this.graph.render(0)
    this.syncClusterLabelPositionsFromRenderedPoints()
    this.scheduleLabelSync()
  }

  private clearForceStopTimer(): void {
    if (this.forceStopTimer !== null) {
      clearTimeout(this.forceStopTimer)
      this.forceStopTimer = null
    }
  }

  private clearZoomBaselineTimer(): void {
    if (this.zoomBaselineTimer !== null) {
      clearTimeout(this.zoomBaselineTimer)
      this.zoomBaselineTimer = null
    }
  }

  private scheduleZoomBaselineStabilization(duration = 0): void {
    this.clearZoomBaselineTimer()
    const delay = Math.max(0, duration) + 80
    this.zoomBaselineTimer = setTimeout(() => {
      this.zoomBaselineTimer = null
      if (this.destroyed || !this.graph) return
      const zoom = this.graph.getZoomLevel()
      if (!Number.isFinite(zoom) || zoom <= 0) return
      this.zoomLevel = zoom
      this.zoomBaseline = this.getAdaptiveZoomBaseline(zoom)
      this.lastVisualScaleKey = ''
      this.applyAdaptiveNodeSizes(true)
      this.scheduleLabelSync()
    }, delay)
  }

  private scheduleRestoredViewVisualSync(duration = 0): void {
    this.clearZoomBaselineTimer()
    const delay = Math.max(0, duration) + 80
    this.zoomBaselineTimer = setTimeout(() => {
      this.zoomBaselineTimer = null
      if (this.destroyed || !this.graph) return
      const zoom = this.graph.getZoomLevel()
      if (Number.isFinite(zoom) && zoom > 0) this.zoomLevel = zoom
      this.lastVisualScaleKey = ''
      this.applyAdaptiveNodeSizes(true)
      this.scheduleLabelSync()
    }, delay)
  }

  private clearTransientStateForDataChange(): void {
    this.cancelHoverTimer()
    this.cancelHoverFade()
    this.hover.lockedNodeId = undefined
    this.hover.lockedLinkIndex = undefined
    this.hover.hoveredNodeId = undefined
    this.hover.hoveredNeighborIds = undefined
    this.hover.hoveredLinkIndex = undefined
    this.hover.hoverProgress = 0
    this.hover.screenPosition = undefined
    this.suppressNextClick = false
    this.ignoreNextLockedBackgroundClick = false
    if (this.ignoreNextLockedBackgroundClickTimer !== null) {
      clearTimeout(this.ignoreNextLockedBackgroundClickTimer)
      this.ignoreNextLockedBackgroundClickTimer = null
    }
    this.stopManualDragListeners()
    this.stopLockedBackgroundPressListeners()
    this.dragState = null
    this.lockedBackgroundPress = null
  }

  private zoomToNearestPoint(position: [number, number], duration: number, zoom: number): void {
    if (!this.graph || !this.nodes.length) return
    let bestIndex = 0
    let bestDistance = Infinity
    if (!this.canUseCachedHitPositions()) {
      let positions: number[]
      try { positions = this.graph.getPointPositions() } catch { return }
      for (let i = 0; i < this.nodes.length; i += 1) {
        const x = positions[i * 2]
        const y = positions[i * 2 + 1]
        if (!Number.isFinite(x) || !Number.isFinite(y)) continue
        const distance = (x - position[0]) ** 2 + (y - position[1]) ** 2
        if (distance < bestDistance) {
          bestDistance = distance
          bestIndex = i
        }
      }
      this.graph.zoomToPointByIndex(bestIndex, duration, zoom)
      return
    }
    for (let i = 0; i < this.nodes.length; i += 1) {
      const nodePosition = this.getNodeSpacePosition(i)
      const x = nodePosition?.[0]
      const y = nodePosition?.[1]
      if (x === undefined || y === undefined || !Number.isFinite(x) || !Number.isFinite(y)) continue
      const distance = (x - position[0]) ** 2 + (y - position[1]) ** 2
      if (distance < bestDistance) {
        bestDistance = distance
        bestIndex = i
      }
    }
    this.graph.zoomToPointByIndex(bestIndex, duration, zoom)
  }

  private syncNodePositionsFromGraph(): void {
    if (!this.graph) return
    let positions: number[]
    try { positions = this.graph.getPointPositions() } catch { return }
    for (let i = 0; i < this.nodes.length; i += 1) {
      const x = positions[i * 2]
      const y = positions[i * 2 + 1]
      if (Number.isFinite(x) && Number.isFinite(y)) {
        this.nodes[i].x = x
        this.nodes[i].y = y
      }
    }
    this.updateClusterCentroidsFromNodes()
  }

  private getZoomBounds(): [number, number] {
    const min = this.currentLayout.minZoomLevel ?? DEFAULT_MIN_ZOOM_LEVEL
    const max = this.currentLayout.maxZoomLevel ?? DEFAULT_MAX_ZOOM_LEVEL
    return min <= max ? [min, max] : [max, min]
  }

  private getAdaptiveZoomBaseline(zoom: number): number {
    if (!Number.isFinite(zoom) || zoom <= 0) return zoom
    if (this.currentLayout.forceDetailedView === true) return zoom
    const count = this.nodes.length
    if (count <= 0) return zoom

    const [minZoom, maxZoom] = this.getZoomBounds()
    let baseline = zoom

    const initialVisualZoomFactor = getInitialVisualZoomFactor(count)
    if (initialVisualZoomFactor > 1) {
      baseline = Math.min(baseline, zoom / initialVisualZoomFactor)
    }

    if (this.currentLayout.focusActive === true && count < 1000) {
      const targetRelativeZoom = count <= 360 ? 8 : 6
      baseline = Math.min(baseline, maxZoom / targetRelativeZoom)
    }

    return clamp(baseline, minZoom, maxZoom)
  }

  private applyZoomBounds(): void {
    if (!this.graph) return
    const [minZoom, maxZoom] = this.getZoomBounds()
    const zoomBehavior = (this.graph as any).zoomInstance?.behavior
    if (zoomBehavior?.scaleExtent) zoomBehavior.scaleExtent([minZoom, maxZoom])
    this.clampZoomLevel()
  }

  private clampZoomLevel(): void {
    if (!this.graph) return
    const [minZoom, maxZoom] = this.getZoomBounds()
    const zoom = this.graph.getZoomLevel()
    const clamped = clamp(zoom, minZoom, maxZoom)
    if (Math.abs(clamped - zoom) > 0.0001) {
      this.graph.setZoomLevel(clamped, 120)
    }
    this.zoomLevel = clamped
  }

  private createGraph(layout: LayoutOptions): void {
    const isForce = this.isForceMode(layout)
    const enableSimulation = false
    const nodeCount = this.nodes.length
    const forceRepulsion = nodeCount >= 2500 ? 8.5 : nodeCount >= 800 ? 6.2 : SIM_DEFAULTS.repulsion
    const forceGravity = nodeCount >= 2500 ? 0.025 : nodeCount >= 800 ? 0.04 : SIM_DEFAULTS.gravity
    const forceLinkDistance = nodeCount >= 2500 ? 96 : nodeCount >= 800 ? 78 : SIM_DEFAULTS.linkDistance

    this.graph = new Graph(this.container, {
      backgroundColor: BG_COLOR,
      spaceSize: COSMOS_SPACE_SIZE,
      enableSimulation,
      fitViewOnInit: true,
      fitViewDelay: 100,
      fitViewPadding: 0.1,
      fitViewDuration: 400,
      rescalePositions: !isForce,

      simulationFriction: layout.simulationFriction ?? SIM_DEFAULTS.friction,
      simulationGravity: layout.simulationGravity ?? forceGravity,
      simulationRepulsion: layout.simulationRepulsion ?? forceRepulsion,
      simulationDecay: SIM_DEFAULTS.decay,
      simulationLinkSpring: SIM_DEFAULTS.linkSpring,
      simulationLinkDistance: forceLinkDistance,
      simulationCluster: 0,
      simulationCenter: SIM_DEFAULTS.center,
      simulationRepulsionTheta: 0.9,
      useClassicQuadtree: true,

      pointDefaultColor: '#6366f1',
      pointDefaultSize: POINT_SIZE_BASE,
      pointSizeScale: 1,
      pointOpacity: 0.98,
      pointGreyoutOpacity: 0.15,

      linkDefaultColor: LINK_DEFAULT_COLOR,
      linkDefaultWidth: LINK_WIDTH * clamp(layout.linkWidthScale ?? 1, 0.6, 3),
      linkWidthScale: 1,
      linkOpacity: clamp(layout.linkOpacity ?? LINK_OPACITY, 0.08, 1),
      linkGreyoutOpacity: LINK_GREYOUT_OPACITY,
      linkDefaultArrows: true,
      linkArrowsSizeScale: 1.15,
      curvedLinks: true,
      curvedLinkWeight: CURVED_LINK_WEIGHT,
      curvedLinkControlPointDistance: CURVED_LINK_CONTROL_POINT_DISTANCE,
      hoveredLinkColor: '#4f46e5',
      hoveredLinkWidthIncrease: 5,
      hoveredLinkCursor: 'pointer',

      renderHoveredPointRing: false,
      hoveredPointRingColor: HOVER_RING_COLOR,
      focusedPointRingColor: FOCUSED_RING_COLOR,
      hoveredPointCursor: 'pointer',

      scalePointsOnZoom: false,
      scaleLinksOnZoom: false,
      enableDrag: layout.enableDrag === true,
      enableZoom: true,
      pixelRatio: Math.max(2, Math.min(window.devicePixelRatio || 2, 3)),
      pointSamplingDistance: 48,
      renderLinks: true,

      onClick: (index: number | undefined, _pos: [number, number] | undefined, ev: MouseEvent) => {
        this.handleCanvasClick(index, ev)
      },

      onMouseMove: (_index: number | undefined, _pos: [number, number] | undefined, ev: MouseEvent) => {
        this.lastPointerPosition = this.getEventOffset(ev)
      },

      onPointMouseOver: (index: number, _pos: [number, number] | undefined, ev: any) => {
        if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
        if (ev && Number.isFinite(ev.clientX) && Number.isFinite(ev.clientY)) this.lastPointerPosition = this.getEventOffset(ev)
        if (ev && Number.isFinite(ev.clientX) && Number.isFinite(ev.clientY) && !this.isNodeIndexAtScreenPosition(index, ev, this.getNodeHitRadius(index, 4, 18))) {
          return
        }
        const nodeId = this.indexToNodeId.get(index)
        if (nodeId) this.scheduleHover(nodeId)
      },

      onPointMouseOut: () => {
        if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
        if (this.lastPointerPosition && this.findNodeAtScreenPoint(this.lastPointerPosition, 4, 18) !== undefined) return
        this.scheduleHoverLeave()
      },

      onLinkMouseOver: (linkIndex: number) => {
        if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
        if (this.lastPointerPosition) {
          const nodeIndex = this.findNodeAtScreenPoint(this.lastPointerPosition, 4, 18)
          const nodeId = nodeIndex === undefined ? undefined : this.indexToNodeId.get(nodeIndex)
          if (nodeId) {
            this.scheduleHover(nodeId)
            return
          }
        }
        this.cancelHoverTimer()
        this.cancelHoverFade()
        if (this.hover.hoveredLinkIndex === linkIndex && this.hover.hoverProgress > 0) return
        this.hover.hoveredLinkIndex = linkIndex
        this.hover.hoveredNodeId = undefined
        this.hover.hoveredNeighborIds = undefined
        this.hover.screenPosition = this.lastPointerPosition
        this.hover.hoverProgress = 1
        if (this.shouldFocusOnHover()) this.applyLinkFocus(linkIndex, 1)
        this.callbacks.onHoverChange?.(this.hover)
        if (this.shouldFocusOnHover()) this.scheduleLabelSync()
      },

      onLinkMouseOut: () => {
        if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
        if (this.shouldFocusOnHover()) this.resetVisualState()
        this.clearTransientHover()
      },

      onZoom: () => {
        this.zoomLevel = this.graph?.getZoomLevel() ?? 1
        this.callbacks.onZoom?.(this.zoomLevel)
        this.scheduleVisualScaleSync()
        this.scheduleLabelSync()
      },

      onZoomEnd: () => {
        this.clampZoomLevel()
        if (this.zoomBaseline <= 0 && this.zoomLevel > 0) {
          this.zoomBaseline = this.getAdaptiveZoomBaseline(this.zoomLevel)
          this.applyAdaptiveNodeSizes(true)
        }
        this.applyAdaptiveNodeSizes()
        this.scheduleLabelSync()
      },

      onSimulationTick: () => {
        this.callbacks.onSimulationTick?.()
        this.scheduleLabelSync()
      },

      onSimulationEnd: () => {
        this.callbacks.onSimulationEnd?.()
        this.scheduleLabelSync()
      },

      onDrag: () => { this.scheduleLabelSync() },
      onDragEnd: () => { this.scheduleLabelSync() },
    })
    this.applyZoomBounds()
  }

  private handleCanvasClick(nativePointIndex: number | undefined, event: MouseEvent): void {
    if (this.suppressNextClick) {
      this.suppressNextClick = false
      return
    }

    if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) {
      if (this.ignoreNextLockedBackgroundClick) {
        this.ignoreNextLockedBackgroundClick = false
        return
      }
      const clickedLockedNativeNode = nativePointIndex !== undefined && this.indexToNodeId.get(nativePointIndex) === this.hover.lockedNodeId
      if (!clickedLockedNativeNode && !this.isLockedItemAtScreenPosition(event)) {
        this.clearHover()
        this.callbacks.onBackgroundClick?.()
      }
      return
    }

    if (nativePointIndex !== undefined) {
      if (this.isNodeIndexAtScreenPosition(nativePointIndex, event, this.getNodeHitRadius(nativePointIndex))) {
        this.dispatchNodeClick(nativePointIndex)
        return
      }
      nativePointIndex = undefined
    }

    if (this.hover.hoveredLinkIndex !== undefined) {
      if (this.isLinkIndexAtScreenPosition(this.hover.hoveredLinkIndex, event, LINK_CLICK_HIT_DISTANCE)) {
        this.dispatchLinkClick(this.hover.hoveredLinkIndex)
        return
      }
    }

    const pointIndex = this.findNodeAtScreenPosition(event)
    if (pointIndex !== undefined) {
      this.dispatchNodeClick(pointIndex)
      return
    }

    const linkIndex = this.findLinkAtScreenPosition(event, LINK_CLICK_HIT_DISTANCE)
    if (linkIndex !== undefined) {
      this.dispatchLinkClick(linkIndex)
      return
    }

    this.clearHover()
    this.callbacks.onBackgroundClick?.()
  }

  private handlePointerDown = (event: PointerEvent): void => {
    if (!this.graph) return
    if (event.button !== 0) return

    if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) {
      if (!this.isLockedItemAtScreenPosition(event)) {
        this.lockedBackgroundPress = {
          pointerId: event.pointerId,
          start: this.getContainerOffset(event),
          moved: false,
        }
        window.addEventListener('pointermove', this.handleLockedBackgroundPointerMove, true)
        window.addEventListener('pointerup', this.handleLockedBackgroundPointerUp, true)
        window.addEventListener('pointercancel', this.handleLockedBackgroundPointerUp, true)
        return
      }
    }

    if (this.currentLayout.enableDrag === false) return

    const index = this.findNodeAtScreenPosition(event, 7, 24)
    if (index === undefined) return

    const point = this.getContainerOffset(event)
    this.dragState = { index, pointerId: event.pointerId, start: point, moved: false }
    window.addEventListener('pointermove', this.handlePointerMove, true)
    window.addEventListener('pointerup', this.handlePointerUp, true)
    window.addEventListener('pointercancel', this.handlePointerUp, true)
    event.preventDefault()
    event.stopPropagation()
  }

  private handlePointerMove = (event: PointerEvent): void => {
    const state = this.dragState
    if (!state || event.pointerId !== state.pointerId || !this.graph) return

    const point = this.getContainerOffset(event)
    const dx = point[0] - state.start[0]
    const dy = point[1] - state.start[1]
    if (!state.moved && Math.hypot(dx, dy) < 3) return

    state.moved = true
    this.moveNodeToScreenPosition(state.index, point)
    event.preventDefault()
    event.stopPropagation()
  }

  private handlePointerUp = (event: PointerEvent): void => {
    const state = this.dragState
    if (!state || event.pointerId !== state.pointerId) return

    this.dragState = null
    this.stopManualDragListeners()
    if (state.moved) {
      this.suppressNextClick = true
      this.scheduleLabelSync()
    } else {
      this.suppressNextClick = true
      this.dispatchNodeClick(state.index)
    }
    event.preventDefault()
    event.stopPropagation()
  }

  private stopManualDragListeners(): void {
    window.removeEventListener('pointermove', this.handlePointerMove, true)
    window.removeEventListener('pointerup', this.handlePointerUp, true)
    window.removeEventListener('pointercancel', this.handlePointerUp, true)
  }

  private handleLockedBackgroundPointerMove = (event: PointerEvent): void => {
    const state = this.lockedBackgroundPress
    if (!state || event.pointerId !== state.pointerId) return
    const point = this.getContainerOffset(event)
    if (Math.hypot(point[0] - state.start[0], point[1] - state.start[1]) > 5) {
      state.moved = true
    }
  }

  private handleLockedBackgroundPointerUp = (event: PointerEvent): void => {
    const state = this.lockedBackgroundPress
    if (!state || event.pointerId !== state.pointerId) return
    this.lockedBackgroundPress = null
    this.stopLockedBackgroundPressListeners()
    if (state.moved) {
      this.ignoreNextLockedBackgroundClick = true
      if (this.ignoreNextLockedBackgroundClickTimer !== null) clearTimeout(this.ignoreNextLockedBackgroundClickTimer)
      this.ignoreNextLockedBackgroundClickTimer = setTimeout(() => {
        this.ignoreNextLockedBackgroundClick = false
        this.ignoreNextLockedBackgroundClickTimer = null
      }, 250)
    }
  }

  private stopLockedBackgroundPressListeners(): void {
    window.removeEventListener('pointermove', this.handleLockedBackgroundPointerMove, true)
    window.removeEventListener('pointerup', this.handleLockedBackgroundPointerUp, true)
    window.removeEventListener('pointercancel', this.handleLockedBackgroundPointerUp, true)
  }

  private getContainerOffset(event: MouseEvent): [number, number] {
    const rect = this.container.getBoundingClientRect()
    return [event.clientX - rect.left, event.clientY - rect.top]
  }

  private handlePointerTrack = (event: PointerEvent): void => {
    this.lastPointerPosition = this.getContainerOffset(event)
    if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
    if (this.hover.hoveredLinkIndex !== undefined) return
    this.pendingHoverTrackPoint = this.lastPointerPosition
    if (this.hoverTrackRafId !== null) return
    this.hoverTrackRafId = requestAnimationFrame(this.flushPointerTrack)
  }

  private flushPointerTrack = (): void => {
    this.hoverTrackRafId = null
    const point = this.pendingHoverTrackPoint
    this.pendingHoverTrackPoint = null
    if (!point || this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
    if (this.hover.hoveredLinkIndex !== undefined) return
    const index = this.findNodeAtScreenPoint(point, 4, 18)
    const nodeId = index === undefined ? undefined : this.indexToNodeId.get(index)
    if (nodeId) {
      this.scheduleHover(nodeId)
    } else if (this.hover.hoveredNodeId) {
      this.scheduleHoverLeave()
    }
  }

  private moveNodeToScreenPosition(index: number, screenPosition: [number, number]): void {
    if (!this.graph) return
    this.syncNodePositionsFromGraph()
    let next: [number, number]
    try { next = this.graph.screenToSpacePosition(screenPosition) } catch { return }
    const node = this.nodes[index]
    if (!node) return
    node.x = next[0]
    node.y = next[1]
    this.buildPointPositions(true)
    this.graph.render(0)
    this.scheduleLabelSync()
  }

  private dispatchNodeClick(index: number): void {
    const nodeId = this.indexToNodeId.get(index)
    if (!nodeId) return
    this.lockNode(nodeId)
    const rawNode = this.rawNodeById.get(nodeId)
    if (rawNode) this.callbacks.onNodeClick?.(rawNode)
  }

  private dispatchLinkClick(linkIndex: number): void {
    const edgeIndex = this.validEdgeIndices[linkIndex]
    const edge = edgeIndex === undefined ? undefined : this.edges[edgeIndex]
    if (!edge) return
    this.lockLink(linkIndex)
    this.callbacks.onEdgeClick?.(edge.source, edge.target, linkIndex)
  }

  private getEventOffset(event: MouseEvent): [number, number] {
    if (Number.isFinite(event.offsetX) && Number.isFinite(event.offsetY)) {
      return [event.offsetX, event.offsetY]
    }
    const target = event.currentTarget instanceof HTMLElement
      ? event.currentTarget
      : event.target instanceof HTMLElement
        ? event.target
        : null
    const rect = target?.getBoundingClientRect()
    if (!rect) return [event.clientX, event.clientY]
    return [event.clientX - rect.left, event.clientY - rect.top]
  }

  private screenPointToSpace(point: [number, number]): [number, number] | undefined {
    if (!this.graph) return undefined
    try { return this.graph.screenToSpacePosition(point) } catch { return undefined }
  }

  private getScreenToSpaceRatio(): number {
    if (!this.graph) return 1
    let zoom = this.zoomLevel
    try { zoom = this.graph.getZoomLevel() } catch { /* Keep the last tracked zoom. */ }
    return Number.isFinite(zoom) && zoom > 0 ? 1 / zoom : 1
  }

  private getNodeSpacePosition(index: number): [number, number] | undefined {
    const node = this.nodes[index]
    if (!node || !Number.isFinite(node.x) || !Number.isFinite(node.y)) return undefined
    return [node.x, node.y]
  }

  private getViewNodePosition(index: number): [number, number] | undefined {
    if (this.canUseCachedHitPositions()) return this.getNodeSpacePosition(index)
    if (!this.graph) return undefined
    let positions: number[]
    try { positions = this.graph.getPointPositions() } catch { return undefined }
    const x = positions[index * 2]
    const y = positions[index * 2 + 1]
    if (!Number.isFinite(x) || !Number.isFinite(y)) return undefined
    return [x, y]
  }

  private canUseCachedHitPositions(): boolean {
    return this.isForceMode(this.currentLayout)
  }

  private findNodeAtScreenPosition(event: MouseEvent, minRadius = 5, maxRadius = 20): number | undefined {
    return this.findNodeAtScreenPoint(this.getEventOffset(event), minRadius, maxRadius)
  }

  private findNodeAtScreenPoint(point: [number, number], minRadius = 5, maxRadius = 20): number | undefined {
    if (!this.canUseCachedHitPositions()) return this.findNodeAtScreenPointFromGraph(point, minRadius, maxRadius)
    const spacePoint = this.screenPointToSpace(point)
    if (!spacePoint) return undefined
    const scale = this.getScreenToSpaceRatio()
    let bestIndex: number | undefined
    let bestDistanceSq = Infinity
    for (const node of this.nodes) {
      const i = node.index
      if (!Number.isFinite(node.x) || !Number.isFinite(node.y)) continue
      const radius = this.getNodeHitRadius(i, minRadius, maxRadius) * scale
      const dx = node.x - spacePoint[0]
      const dy = node.y - spacePoint[1]
      const distSq = dx * dx + dy * dy
      if (distSq <= radius * radius && distSq < bestDistanceSq) {
        bestDistanceSq = distSq
        bestIndex = i
      }
    }
    return bestIndex
  }

  private findNodeAtScreenPointFromGraph(point: [number, number], minRadius = 5, maxRadius = 20): number | undefined {
    if (!this.graph) return undefined
    let positions: number[]
    try { positions = this.graph.getPointPositions() } catch { return undefined }

    const [x, y] = point
    let bestIndex: number | undefined
    let bestDistanceSq = Infinity
    for (const node of this.nodes) {
      const i = node.index
      if (i * 2 + 1 >= positions.length) continue
      let screen: [number, number]
      try { screen = this.graph.spaceToScreenPosition([positions[i * 2], positions[i * 2 + 1]]) } catch { continue }
      const radius = this.getNodeHitRadius(i, minRadius, maxRadius)
      const dx = screen[0] - x
      const dy = screen[1] - y
      const distSq = dx * dx + dy * dy
      if (distSq <= radius * radius && distSq < bestDistanceSq) {
        bestDistanceSq = distSq
        bestIndex = i
      }
    }
    return bestIndex
  }

  private getNodeHitRadius(index: number, minRadius = 5, maxRadius = 20): number {
    const node = this.nodes[index]
    const nodeScreenSize = node?.screenSize || this.currentPointScreenSize || IMAGE_SIZE
    return Math.max(minRadius, Math.min(maxRadius, nodeScreenSize / 2 + 3))
  }

  private isNodeIndexAtScreenPosition(index: number, event: MouseEvent, radius: number): boolean {
    if (!this.canUseCachedHitPositions()) return this.isNodeIndexAtScreenPositionFromGraph(index, event, radius)
    const nodePosition = this.getNodeSpacePosition(index)
    const point = this.screenPointToSpace(this.getEventOffset(event))
    if (!nodePosition || !point) return false
    const spaceRadius = radius * this.getScreenToSpaceRatio()
    return Math.hypot(nodePosition[0] - point[0], nodePosition[1] - point[1]) <= spaceRadius
  }

  private isNodeIndexAtScreenPositionFromGraph(index: number, event: MouseEvent, radius: number): boolean {
    if (!this.graph) return false
    let positions: number[]
    try { positions = this.graph.getPointPositions() } catch { return false }
    if (index * 2 + 1 >= positions.length) return false
    let screen: [number, number]
    try { screen = this.graph.spaceToScreenPosition([positions[index * 2], positions[index * 2 + 1]]) } catch { return false }
    const [x, y] = this.getEventOffset(event)
    return Math.hypot(screen[0] - x, screen[1] - y) <= radius
  }

  private findLinkAtScreenPosition(event: MouseEvent, maxDistancePx = LINK_CLICK_HIT_DISTANCE): number | undefined {
    if (!this.canUseCachedHitPositions()) return this.findLinkAtScreenPositionFromGraph(event, maxDistancePx)
    const point = this.screenPointToSpace(this.getEventOffset(event))
    if (!point) return undefined
    const maxDistance = maxDistancePx * this.getScreenToSpaceRatio()

    let bestLinkIndex: number | undefined
    let bestDistance = Infinity
    for (let linkIndex = 0; linkIndex < this.validEdgeIndices.length; linkIndex += 1) {
      const edgeIndex = this.validEdgeIndices[linkIndex]
      const edge = this.edges[edgeIndex]
      if (!edge) continue
      const sourceIndex = this.nodeIdToIndex.get(edge.source)
      const targetIndex = this.nodeIdToIndex.get(edge.target)
      if (sourceIndex === undefined || targetIndex === undefined) continue
      const source = this.getNodeSpacePosition(sourceIndex)
      const target = this.getNodeSpacePosition(targetIndex)
      if (!source || !target) continue

      const distance = distanceToRenderedLink(point, source, target)
      if (distance < maxDistance && distance < bestDistance) {
        bestDistance = distance
        bestLinkIndex = linkIndex
      }
    }
    return bestLinkIndex
  }

  private findLinkAtScreenPositionFromGraph(event: MouseEvent, maxDistance = LINK_CLICK_HIT_DISTANCE): number | undefined {
    if (!this.graph) return undefined
    const [x, y] = this.getEventOffset(event)
    let positions: number[]
    try { positions = this.graph.getPointPositions() } catch { return undefined }

    let bestLinkIndex: number | undefined
    let bestDistance = Infinity
    for (let linkIndex = 0; linkIndex < this.validEdgeIndices.length; linkIndex += 1) {
      const edgeIndex = this.validEdgeIndices[linkIndex]
      const edge = this.edges[edgeIndex]
      if (!edge) continue
      const sourceIndex = this.nodeIdToIndex.get(edge.source)
      const targetIndex = this.nodeIdToIndex.get(edge.target)
      if (sourceIndex === undefined || targetIndex === undefined) continue
      if (sourceIndex * 2 + 1 >= positions.length || targetIndex * 2 + 1 >= positions.length) continue

      let source: [number, number]
      let target: [number, number]
      try {
        source = this.graph.spaceToScreenPosition([positions[sourceIndex * 2], positions[sourceIndex * 2 + 1]])
        target = this.graph.spaceToScreenPosition([positions[targetIndex * 2], positions[targetIndex * 2 + 1]])
      } catch {
        continue
      }

      const distance = distanceToRenderedLink([x, y], source, target)
      if (distance < maxDistance && distance < bestDistance) {
        bestDistance = distance
        bestLinkIndex = linkIndex
      }
    }
    return bestLinkIndex
  }

  private isLinkIndexAtScreenPosition(linkIndex: number, event: MouseEvent, maxDistance: number): boolean {
    if (!this.canUseCachedHitPositions()) return this.isLinkIndexAtScreenPositionFromGraph(linkIndex, event, maxDistance)
    const edgeIndex = this.validEdgeIndices[linkIndex]
    const edge = this.edges[edgeIndex]
    if (!edge) return false
    const sourceIndex = this.nodeIdToIndex.get(edge.source)
    const targetIndex = this.nodeIdToIndex.get(edge.target)
    if (sourceIndex === undefined || targetIndex === undefined) return false

    const source = this.getNodeSpacePosition(sourceIndex)
    const target = this.getNodeSpacePosition(targetIndex)
    const point = this.screenPointToSpace(this.getEventOffset(event))
    if (!source || !target || !point) return false
    return distanceToRenderedLink(point, source, target) <= maxDistance * this.getScreenToSpaceRatio()
  }

  private isLinkIndexAtScreenPositionFromGraph(linkIndex: number, event: MouseEvent, maxDistance: number): boolean {
    if (!this.graph) return false
    const edgeIndex = this.validEdgeIndices[linkIndex]
    const edge = this.edges[edgeIndex]
    if (!edge) return false
    const sourceIndex = this.nodeIdToIndex.get(edge.source)
    const targetIndex = this.nodeIdToIndex.get(edge.target)
    if (sourceIndex === undefined || targetIndex === undefined) return false

    let positions: number[]
    try { positions = this.graph.getPointPositions() } catch { return false }
    if (sourceIndex * 2 + 1 >= positions.length || targetIndex * 2 + 1 >= positions.length) return false

    let source: [number, number]
    let target: [number, number]
    try {
      source = this.graph.spaceToScreenPosition([positions[sourceIndex * 2], positions[sourceIndex * 2 + 1]])
      target = this.graph.spaceToScreenPosition([positions[targetIndex * 2], positions[targetIndex * 2 + 1]])
    } catch {
      return false
    }

    const point = this.getEventOffset(event)
    return distanceToRenderedLink(point, source, target) <= maxDistance
  }

  private isLockedItemAtScreenPosition(event: MouseEvent): boolean {
    if (this.hover.lockedNodeId) {
      const index = this.nodeIdToIndex.get(this.hover.lockedNodeId)
      return index !== undefined && this.isNodeIndexAtScreenPosition(index, event, LOCKED_NODE_CANCEL_RADIUS)
    }
    if (this.hover.lockedLinkIndex !== undefined) {
      return this.isLinkIndexAtScreenPosition(this.hover.lockedLinkIndex, event, LOCKED_LINK_CANCEL_DISTANCE)
    }
    return false
  }

  private captureNodePositions(): Map<string, [number, number]> {
    const positions = new Map<string, [number, number]>()
    if (!this.nodes.length) return positions

    if (this.graph) {
      try {
        const pointPositions = this.graph.getPointPositions()
        this.nodes.forEach((node) => {
          const x = pointPositions[node.index * 2]
          const y = pointPositions[node.index * 2 + 1]
          if (Number.isFinite(x) && Number.isFinite(y)) positions.set(node.id, [x, y])
        })
        if (positions.size > 0) return positions
      } catch {
        // Fall through to the last deterministic coordinates.
      }
    }

    this.nodes.forEach((node) => {
      if (Number.isFinite(node.x) && Number.isFinite(node.y)) positions.set(node.id, [node.x, node.y])
    })
    return positions
  }

  private restoreStableNodePositions(previousPositions: Map<string, [number, number]>): void {
    if (previousPositions.size === 0 || this.nodes.length === 0) return

    const restored = new Set<string>()
    let sumX = 0
    let sumY = 0
    this.nodes.forEach((node) => {
      const previous = previousPositions.get(node.id)
      if (!previous) return
      node.x = previous[0]
      node.y = previous[1]
      restored.add(node.id)
      sumX += previous[0]
      sumY += previous[1]
    })

    if (restored.size === 0) return
    const fallback: [number, number] = [sumX / restored.size, sumY / restored.size]
    const byId = new Map(this.nodes.map(node => [node.id, node] as const))

    this.nodes.forEach((node) => {
      if (restored.has(node.id)) return
      const neighborPositions: [number, number][] = []
      this.edges.forEach((edge) => {
        const neighborId = edge.source === node.id ? edge.target : edge.target === node.id ? edge.source : ''
        if (!neighborId) return
        const neighbor = byId.get(neighborId)
        if (neighbor && restored.has(neighbor.id)) neighborPositions.push([neighbor.x, neighbor.y])
      })

      if (neighborPositions.length > 0) {
        const cx = neighborPositions.reduce((total, item) => total + item[0], 0) / neighborPositions.length
        const cy = neighborPositions.reduce((total, item) => total + item[1], 0) / neighborPositions.length
        const angle = hashToUnit(`${node.id}:timeline-angle`) * Math.PI * 2
        const radius = 36 + hashToUnit(`${node.id}:timeline-radius`) * 64
        node.x = clamp(cx + Math.cos(angle) * radius, 0, COSMOS_SPACE_SIZE)
        node.y = clamp(cy + Math.sin(angle) * radius, 0, COSMOS_SPACE_SIZE)
      } else if (!Number.isFinite(node.x) || !Number.isFinite(node.y)) {
        const angle = hashToUnit(`${node.id}:timeline-fallback-angle`) * Math.PI * 2
        const radius = 80 + hashToUnit(`${node.id}:timeline-fallback-radius`) * 180
        node.x = clamp(fallback[0] + Math.cos(angle) * radius, 0, COSMOS_SPACE_SIZE)
        node.y = clamp(fallback[1] + Math.sin(angle) * radius, 0, COSMOS_SPACE_SIZE)
      }
    })

    this.updateClusterCentroidsFromNodes()
  }

  /* ─────────── Internal: Index ─────────── */

  private buildIndex(data: TopoData): void {
    this.rawNodes = data.nodes
    this.edges = data.edges
    this.lastVisualScaleKey = ''
    this.pointBaseColors = new Float32Array(0)
    this.pointBaseRgb = new Float32Array(0)
    this.pointPositions = new Float32Array(0)
    this.pointSizes = new Float32Array(0)
    this.pointColors = new Float32Array(0)
    this.pointImageIndices = new Float32Array(0)
    this.pointImageSizes = new Float32Array(0)
    this.linkBaseColors = new Float32Array(0)
    this.linkBaseOpacities = new Float32Array(0)
    this.clusterLabelPositions = new Map()
    this.nodeById.clear()
    this.rawNodeById.clear()
    this.nodeIdToIndex.clear()
    this.indexToNodeId.clear()
    this.neighbors.clear()
    this.clusterByKey.clear()
    this.edgeTimelinePresence = data.edges.map(edge => normalizeTimelinePresence(edge.data?.timeline?.presence))

    const clusterMap = new Map<string, ClusterRecord>()
    let clusterCounter = 0
    const resolveHashColor = createTopoHashColorResolver(data.nodes.map((n) => (
      getTopoTypeColorSeed(
        n.data?.cluster || n.type || 'default',
        n.title || n.label || n.data?.title || n.id,
        n.data,
        (n.data?.style || n.style || {}) as Record<string, any>,
      )
    )))

    this.nodes = data.nodes.map((n, i) => {
      this.rawNodeById.set(n.id, n)
      const cluster = n.data?.cluster || n.type || 'default'
      const rawStyle = (n.data?.style || n.style || {}) as Record<string, any>
      const existingCluster = clusterMap.get(cluster)
      const colorSeed = getTopoTypeColorSeed(cluster, n.title || n.label || n.data?.title || n.id, n.data, rawStyle)
      const fallbackColor = existingCluster?.iconFill || existingCluster?.color || resolveHashColor(colorSeed)
      const visual = resolveTopoVisualIdentity({
        cluster,
        title: n.title || n.label || n.data?.title || n.id,
        data: n.data,
        style: rawStyle,
        fallbackColor,
      })
      if (!clusterMap.has(cluster)) {
        const ci = clusterCounter++
        clusterMap.set(cluster, {
          key: cluster,
          label: n.data?.subTitle || n.type || cluster,
          color: visual.iconFill,
          iconFill: visual.iconFill,
          count: 0,
          nodeIndices: [],
        })
      }
      const cr = clusterMap.get(cluster)!
      cr.count++
      cr.nodeIndices.push(i)
      cr.iconFill = visual.iconFill
      if (n.data?.subTitle) cr.label = n.data.subTitle
      const baseOpacity = readOpacity(rawStyle.opacity, rawStyle.timelineOpacity, (n as any).opacity)
      const baseSizeScale = Number.isFinite(Number(rawStyle.sizeScale)) ? clamp(Number(rawStyle.sizeScale), 0.18, 1.6) : 1
      const baseDisableVectorIcon = Boolean(rawStyle.disableVectorIcon || rawStyle.vectorIconVisible === false)
      const timelinePresence = normalizeTimelinePresence(n.data?.timeline?.presence)
      const timelineOpacity = getTimelineOpacity(n.id, timelinePresence, this.currentLayout.timelinePlayhead, {
        active: TIMELINE_ACTIVE_NODE_OPACITY,
        past: TIMELINE_PAST_NODE_OPACITY,
        future: TIMELINE_FUTURE_NODE_OPACITY,
      })
      const opacity = timelineOpacity ?? baseOpacity
      const timelineInactive = timelineOpacity !== undefined && opacity < 0.62

      const rec: NodeRecord = {
        id: n.id, index: i, cluster,
        clusterIndex: cr.nodeIndices.length - 1,
        title: n.title || n.label || n.data?.title || n.id,
        subTitle: n.data?.subTitle || n.type || '',
        color: cr.color,
        iconFill: visual.iconFill || cr.iconFill,
        iconClass: visual.iconClass,
        iconUrl: visual.iconUrl,
        iconPreset: visual.iconPreset,
        lucideIcon: visual.lucideIcon,
        ringWidth: visual.ringWidth,
        vectorIconVisible: false,
        disableVectorIcon: baseDisableVectorIcon || timelineInactive,
        degree: 0,
        opacity,
        baseOpacity,
        baseDisableVectorIcon,
        baseSizeScale,
        timelinePresence,
        sizeScale: timelineOpacity === undefined ? baseSizeScale : opacity >= 0.62 ? 1 : 0.42,
        x: 0, y: 0, size: POINT_SIZE_BASE, screenSize: POINT_SIZE_BASE,
      }
      this.nodeById.set(n.id, rec)
      this.nodeIdToIndex.set(n.id, i)
      this.indexToNodeId.set(i, n.id)
      this.neighbors.set(n.id, new Set())
      return rec
    })

    data.edges.forEach(e => {
      this.neighbors.get(e.source)?.add(e.target)
      this.neighbors.get(e.target)?.add(e.source)
    })
    this.nodes.forEach(n => { n.degree = this.neighbors.get(n.id)?.size ?? 0 })

    this.clusters = Array.from(clusterMap.values()).sort((a, b) => b.count - a.count)
    this.clusters.forEach(c => this.clusterByKey.set(c.key, c))

    if (this.isForceMode(this.currentLayout)) {
      if (this.shouldClusterByType(this.currentLayout)) this.computeClusteredPositions(data)
      else this.computeForcePositions(data)
    } else {
      this.computeClusteredPositions(data)
    }
    this.computeNodeSizes()
    this.buildPointBaseRgb()
  }

  private computeClusteredPositions(data: TopoData): void {
    const cc = this.clusters.length
    if (cc === 0) return

    const n = data.nodes.length
    const nodeSpacing = n >= 8000 ? 38 : n >= 2500 ? 48 : n >= 1000 ? 56 : 68
    const centers = new Map<string, [number, number]>()
    const radii = new Map<string, number>()
    const nodesByCluster = new Map<string, NodeRecord[]>()

    this.nodes.forEach((node) => {
      const list = nodesByCluster.get(node.cluster)
      if (list) list.push(node)
      else nodesByCluster.set(node.cluster, [node])
    })

    if (cc === 1) {
      centers.set(this.clusters[0].key, [0, 0])
    } else {
      const base = Math.max(420, Math.sqrt(n) * nodeSpacing * 0.42)
      const sorted = [...this.clusters].sort((a, b) => b.count - a.count)
      sorted.forEach((cluster, index) => {
        const radius = Math.max(160, Math.sqrt(cluster.count) * nodeSpacing * 0.88 + nodeSpacing * 2.2)
        const angle = index * GOLDEN_ANGLE
        const ring = Math.sqrt(index)
        const jitterAngle = hashToUnit(cluster.key) * Math.PI * 2
        radii.set(cluster.key, radius)
        centers.set(cluster.key, [
          Math.cos(angle) * base * ring + Math.cos(jitterAngle) * nodeSpacing * 0.7,
          Math.sin(angle) * base * ring + Math.sin(jitterAngle) * nodeSpacing * 0.7,
        ])
      })
      relaxCircleCenters(centers, radii, nodeSpacing * 2.4)
    }

    this.clusterPositions = centers
    nodesByCluster.forEach((nodes, clusterKey) => {
      const center = centers.get(clusterKey) || [0, 0]
      const offsets = buildHexOffsets(nodes.length, nodeSpacing)
      const sortedNodes = [...nodes].sort((a, b) => b.degree - a.degree || a.id.localeCompare(b.id))
      sortedNodes.forEach((node, index) => {
        const offset = offsets[index] ?? [0, 0]
        node.x = center[0] + offset[0]
        node.y = center[1] + offset[1]
      })
    })
  }

  /**
   * Type-agnostic seed layout for live force simulation.
   *
   * Cosmos simulation clamps points to [0, spaceSize], so force seeds must not
   * start around negative coordinates. The seed is based only on connectivity
   * and deterministic node ids; type/cluster is deliberately ignored here.
   */
  private computeForcePositions(data: TopoData): void {
    if (!this.nodes.length) return

    const nodeIds = new Set(this.nodes.map(node => node.id))
    const adjacency = new Map<string, string[]>()
    this.nodes.forEach(node => adjacency.set(node.id, []))
    data.edges.forEach((edge) => {
      if (!nodeIds.has(edge.source) || !nodeIds.has(edge.target)) return
      adjacency.get(edge.source)?.push(edge.target)
      adjacency.get(edge.target)?.push(edge.source)
    })

    adjacency.forEach((neighbors) => {
      neighbors.sort((left, right) => {
        const leftDegree = this.nodeById.get(left)?.degree ?? 0
        const rightDegree = this.nodeById.get(right)?.degree ?? 0
        return rightDegree - leftDegree || left.localeCompare(right)
      })
    })

    const visited = new Set<string>()
    const components: NodeRecord[][] = []
    this.nodes
      .slice()
      .sort((a, b) => b.degree - a.degree || a.id.localeCompare(b.id))
      .forEach((seed) => {
        if (visited.has(seed.id)) return
        const component: NodeRecord[] = []
        const queue = [seed.id]
        visited.add(seed.id)
        for (let head = 0; head < queue.length; head += 1) {
          const id = queue[head]
          const node = this.nodeById.get(id)
          if (node) component.push(node)
          adjacency.get(id)?.forEach((nextId) => {
            if (visited.has(nextId)) return
            visited.add(nextId)
            queue.push(nextId)
          })
        }
        components.push(component)
      })

    components.sort((a, b) => b.length - a.length)

    const total = this.nodes.length
    const spacing = getForceLayoutSpacing(total)
    const componentCenters = new Map<string, [number, number]>()
    const componentRadii = new Map<string, number>()
    const componentLocalPositions = new Map<string, Map<string, [number, number]>>()

    components.forEach((component, componentIndex) => {
      const key = `component-${componentIndex}`
      const positions = this.computeForceComponentOffsets(component, adjacency, spacing)
      let radius = spacing * 2
      positions.forEach(([x, y]) => { radius = Math.max(radius, Math.hypot(x, y) + spacing * 2.2) })
      componentLocalPositions.set(key, positions)
      componentRadii.set(key, radius)
    })

    const primaryRadius = componentRadii.get('component-0') ?? spacing
    const haloGap = spacing * (components.length >= 1000 ? 1.45 : components.length >= 250 ? 1.8 : 2.4)
    components.forEach((_component, index) => {
      const key = `component-${index}`
      if (index === 0) {
        componentCenters.set(key, [0, 0])
        return
      }
      const angle = index * GOLDEN_ANGLE + (hashToUnit(`${key}:halo-angle`) - 0.5) * 0.56
      const componentRadius = componentRadii.get(key) ?? spacing
      const radiusJitter = 0.82 + hashToUnit(`${key}:halo-radius`) * 0.34
      const radius = primaryRadius + componentRadius + haloGap * Math.sqrt(index) * radiusJitter
      componentCenters.set(key, [Math.cos(angle) * radius, Math.sin(angle) * radius])
    })

    components.forEach((component, componentIndex) => {
      const key = `component-${componentIndex}`
      const center = componentCenters.get(key) ?? [0, 0]
      const positions = componentLocalPositions.get(key)
      component.forEach((node) => {
        const offset = positions?.get(node.id) ?? [0, 0]
        node.x = center[0] + offset[0]
        node.y = center[1] + offset[1]
      })
    })

    this.normalizeNodePositionsToCosmosSpace(FORCE_SPACE_MARGIN)
    this.updateClusterCentroidsFromNodes()
  }

  private computeForceComponentOffsets(
    component: NodeRecord[],
    adjacency: Map<string, string[]>,
    spacing: number,
  ): Map<string, [number, number]> {
    const positions = new Map<string, [number, number]>()
    if (component.length === 0) return positions
    if (component.length === 1) {
      positions.set(component[0].id, [0, 0])
      return positions
    }

    const root = component.slice().sort((a, b) => b.degree - a.degree || a.id.localeCompare(b.id))[0]
    const componentIds = new Set(component.map(node => node.id))
    const visited = new Set<string>([root.id])
    const levels = new Map<string, number>([[root.id, 0]])
    const order: NodeRecord[] = []
    const queue = [root.id]

    for (let head = 0; head < queue.length; head += 1) {
      const id = queue[head]
      const node = this.nodeById.get(id)
      if (node) order.push(node)
      const level = (levels.get(id) ?? 0) + 1
      adjacency.get(id)?.forEach((nextId) => {
        if (!componentIds.has(nextId) || visited.has(nextId)) return
        visited.add(nextId)
        levels.set(nextId, level)
        queue.push(nextId)
      })
    }

    component
      .filter(node => !visited.has(node.id))
      .sort((a, b) => b.degree - a.degree || a.id.localeCompare(b.id))
      .forEach((node) => {
        levels.set(node.id, 1)
        order.push(node)
      })

    positions.set(root.id, [0, 0])
    for (let index = 1; index < order.length; index += 1) {
      const node = order[index]
      const level = levels.get(node.id) ?? 1
      const angle = index * GOLDEN_ANGLE + hashToUnit(`${node.id}:force-angle`) * 0.72
      const rankRadius = spacing * Math.sqrt(index) * (component.length >= 2500 ? 0.9 : 1.02)
      const levelRadius = spacing * Math.sqrt(level + 1) * 2.1
      const jitter = (hashToUnit(`${node.id}:force-radius`) - 0.5) * spacing * 1.35
      const radius = Math.max(rankRadius, levelRadius) + jitter
      positions.set(node.id, [Math.cos(angle) * radius, Math.sin(angle) * radius])
    }

    return positions
  }

  private normalizeNodePositionsToCosmosSpace(margin: number): void {
    if (!this.nodes.length) return
    let minX = Infinity
    let maxX = -Infinity
    let minY = Infinity
    let maxY = -Infinity
    this.nodes.forEach((node) => {
      minX = Math.min(minX, node.x)
      maxX = Math.max(maxX, node.x)
      minY = Math.min(minY, node.y)
      maxY = Math.max(maxY, node.y)
    })

    if (!Number.isFinite(minX) || !Number.isFinite(maxX) || !Number.isFinite(minY) || !Number.isFinite(maxY)) {
      this.nodes.forEach((node) => {
        node.x = COSMOS_SPACE_CENTER
        node.y = COSMOS_SPACE_CENTER
      })
      return
    }

    const width = Math.max(1, maxX - minX)
    const height = Math.max(1, maxY - minY)
    const targetSize = Math.max(1, COSMOS_SPACE_SIZE - margin * 2)
    const scale = Math.min(1, targetSize / Math.max(width, height))
    const cx = (minX + maxX) / 2
    const cy = (minY + maxY) / 2
    this.nodes.forEach((node) => {
      node.x = clamp(COSMOS_SPACE_CENTER + (node.x - cx) * scale, margin, COSMOS_SPACE_SIZE - margin)
      node.y = clamp(COSMOS_SPACE_CENTER + (node.y - cy) * scale, margin, COSMOS_SPACE_SIZE - margin)
    })
  }

  /**
   * Static relation layout for large real topology views.
   *
   * This intentionally avoids GraphViz for full node-level graphs because
   * browser-side sfdp can lock the main thread on thousands of nodes. The
   * layout is type-agnostic: components and BFS rings come from links, while
   * colors/icons still come from node type.
   */
  private computeRelationPositions(data: TopoData): void {
    const nodeSet = new Set(this.nodes.map(node => node.id))
    const adjacency = new Map<string, Set<string>>()
    this.nodes.forEach(node => adjacency.set(node.id, new Set()))
    data.edges.forEach((edge) => {
      if (!nodeSet.has(edge.source) || !nodeSet.has(edge.target)) return
      adjacency.get(edge.source)?.add(edge.target)
      adjacency.get(edge.target)?.add(edge.source)
    })

    const visited = new Set<string>()
    const components: NodeRecord[][] = []
    this.nodes
      .slice()
      .sort((a, b) => b.degree - a.degree || a.id.localeCompare(b.id))
      .forEach((seed) => {
        if (visited.has(seed.id)) return
        const queue = [seed.id]
        const component: NodeRecord[] = []
        visited.add(seed.id)

        for (let head = 0; head < queue.length; head += 1) {
          const id = queue[head]
          const node = this.nodeById.get(id)
          if (node) component.push(node)
          adjacency.get(id)?.forEach((nextId) => {
            if (visited.has(nextId)) return
            visited.add(nextId)
            queue.push(nextId)
          })
        }

        components.push(component)
      })

    components.sort((a, b) => b.length - a.length)

    const total = this.nodes.length
    const spacing = total >= 8000 ? 42 : total >= 2500 ? 54 : total >= 1000 ? 62 : 76
    const componentCenters = new Map<string, [number, number]>()
    const componentRadii = new Map<string, number>()
    const componentLocalPositions = new Map<string, Map<string, [number, number]>>()

    components.forEach((component, componentIndex) => {
      const componentKey = `component-${componentIndex}`
      const positions = this.computeComponentRelationOffsets(component, adjacency, spacing)
      let radius = spacing
      positions.forEach(([x, y]) => { radius = Math.max(radius, Math.hypot(x, y) + spacing * 2.2) })
      componentLocalPositions.set(componentKey, positions)
      componentRadii.set(componentKey, radius)
    })

    const base = Math.max(520, Math.sqrt(total) * spacing * 0.38)
    components.forEach((_component, index) => {
      const key = `component-${index}`
      if (index === 0) {
        componentCenters.set(key, [0, 0])
        return
      }
      const angle = index * GOLDEN_ANGLE
      const radius = base * Math.sqrt(index)
      componentCenters.set(key, [Math.cos(angle) * radius, Math.sin(angle) * radius])
    })
    if (components.length > 96) {
      packCircleCentersByGrid(componentCenters, componentRadii, spacing * 3.2)
    } else {
      relaxCircleCenters(componentCenters, componentRadii, spacing * 3.2)
    }

    components.forEach((component, componentIndex) => {
      const key = `component-${componentIndex}`
      const center = componentCenters.get(key) ?? [0, 0]
      const positions = componentLocalPositions.get(key)
      component.forEach((node) => {
        const offset = positions?.get(node.id) ?? [0, 0]
        node.x = center[0] + offset[0]
        node.y = center[1] + offset[1]
      })
    })

    this.updateClusterCentroidsFromNodes()
  }

  private computeComponentRelationOffsets(
    component: NodeRecord[],
    adjacency: Map<string, Set<string>>,
    spacing: number,
  ): Map<string, [number, number]> {
    const positions = new Map<string, [number, number]>()
    if (component.length === 0) return positions
    if (component.length === 1) {
      positions.set(component[0].id, [0, 0])
      return positions
    }

    const root = component.slice().sort((a, b) => b.degree - a.degree || a.id.localeCompare(b.id))[0]
    const componentIds = new Set(component.map(node => node.id))
    const levels = new Map<string, number>([[root.id, 0]])
    const queue = [root.id]

    for (let head = 0; head < queue.length; head += 1) {
      const id = queue[head]
      const nextLevel = (levels.get(id) ?? 0) + 1
      const neighbors = Array.from(adjacency.get(id) ?? [])
        .filter(nextId => componentIds.has(nextId))
        .sort((left, right) => {
          const leftDegree = this.nodeById.get(left)?.degree ?? 0
          const rightDegree = this.nodeById.get(right)?.degree ?? 0
          return rightDegree - leftDegree || left.localeCompare(right)
        })
      neighbors.forEach((nextId) => {
        if (levels.has(nextId)) return
        levels.set(nextId, nextLevel)
        queue.push(nextId)
      })
    }

    const byLevel = new Map<number, NodeRecord[]>()
    component.forEach((node) => {
      const level = levels.get(node.id) ?? 0
      const list = byLevel.get(level)
      if (list) list.push(node)
      else byLevel.set(level, [node])
    })

    positions.set(root.id, [0, 0])
    Array.from(byLevel.entries())
      .filter(([level]) => level > 0)
      .sort(([left], [right]) => left - right)
      .forEach(([level, nodes]) => {
        const sorted = nodes.sort((a, b) => b.degree - a.degree || a.id.localeCompare(b.id))
        const circumferenceRadius = (sorted.length * spacing * 1.05) / (Math.PI * 2)
        const ringRadius = Math.max(level * spacing * 1.85, circumferenceRadius)
        sorted.forEach((node, index) => {
          const slot = index / Math.max(1, sorted.length)
          const angle = slot * Math.PI * 2 + hashToUnit(`${node.id}:${level}`) * 0.42 + level * 0.28
          const jitter = (hashToUnit(`${node.id}:r`) - 0.5) * spacing * 0.42
          positions.set(node.id, [
            Math.cos(angle) * (ringRadius + jitter),
            Math.sin(angle) * (ringRadius + jitter),
          ])
        })
      })

    return positions
  }

  private updateClusterCentroidsFromNodes(): void {
    const sums = new Map<string, { x: number; y: number; count: number }>()
    this.nodes.forEach((node) => {
      const item = sums.get(node.cluster) ?? { x: 0, y: 0, count: 0 }
      item.x += node.x
      item.y += node.y
      item.count += 1
      sums.set(node.cluster, item)
    })

    this.clusterPositions = new Map()
    sums.forEach((item, key) => {
      this.clusterPositions.set(key, [item.x / Math.max(1, item.count), item.y / Math.max(1, item.count)])
    })
  }

  private syncClusterLabelPositionsFromRenderedPoints(): void {
    if (!this.graph || this.clusters.length === 0 || this.nodes.length === 0) return

    let positions: number[]
    try { positions = this.graph.getPointPositions() } catch { return }
    if (!positions || positions.length < this.nodes.length * 2) return

    const next = new Map<string, [number, number]>()
    this.clusters.forEach((cluster) => {
      let sumX = 0
      let sumY = 0
      let count = 0
      cluster.nodeIndices.forEach((nodeIndex) => {
        const x = positions[nodeIndex * 2]
        const y = positions[nodeIndex * 2 + 1]
        if (!Number.isFinite(x) || !Number.isFinite(y)) return
        sumX += x
        sumY += y
        count += 1
      })
      if (count > 0) next.set(cluster.key, [sumX / count, sumY / count])
    })
    if (next.size > 0) this.clusterLabelPositions = next
  }

  private computeNodeSizes(): void {
    let maxDeg = 1
    this.nodes.forEach(n => { if (n.degree > maxDeg) maxDeg = n.degree })
    const logMax = Math.log(1 + maxDeg)
    this.nodes.forEach(n => {
      const t = n.degree === 0 ? 0 : Math.log(1 + n.degree) / logMax
      n.size = Math.max(IMAGE_SIZE, POINT_SIZE_BASE + (POINT_SIZE_MAX - POINT_SIZE_BASE) * t)
    })
  }

  /* ─────────── Internal: GPU buffers ─────────── */

  private pushAllBuffers(dontRescalePositions = false): void {
    if (!this.graph) return
    this.buildPointPositions(dontRescalePositions)
    this.buildPointColors()
    this.buildPointSizes()
    this.buildLinks()
  }

  private applyAsyncGraphvizLayout(data: TopoData, layout: LayoutOptions): void {
    if (!this.isGraphvizMode(layout)) return
    const runId = ++this.graphvizLayoutRunId
    computeGraphvizLayout(data, {
      engine: layout.graphvizEngine ?? 'sfdp',
      rankdir: layout.graphvizRankdir ?? 'LR',
      packClusters: this.shouldClusterByType(layout),
      packMode: this.shouldClusterByType(layout) ? 'cluster' : 'component',
    }).then((result) => {
      if (this.destroyed || runId !== this.graphvizLayoutRunId || !this.graph) return
      let changed = false
      this.nodes.forEach((node) => {
        const position = result.positions.get(node.id)
        if (!position) return
        node.x = position[0]
        node.y = position[1]
        changed = true
      })
      if (!changed) return
      this.clusterPositions = result.clusterPositions
      this.buildPointPositions()
      this.graph.render(0)
      this.syncClusterLabelPositionsFromRenderedPoints()
      if (!layout.skipFitViewOnDataUpdate) this.fitView(360)
      this.scheduleLabelSync()
    }).catch(() => {
      // Keep the deterministic clustered layout if GraphViz is unavailable.
    })
  }

  private buildPointPositions(dontRescale = false): void {
    const buf = new Float32Array(this.nodes.length * 2)
    this.nodes.forEach((n, i) => { buf[i * 2] = n.x; buf[i * 2 + 1] = n.y })
    this.pointPositions = buf
    this.graph!.setPointPositions(buf, dontRescale)
  }

  private hasTimelinePresence(): boolean {
    return (
      this.nodes.some(node => node.timelinePresence && node.timelinePresence.length > 0) ||
      this.edgeTimelinePresence.some(Boolean)
    )
  }

  private getTimelineNodeOpacity(node: NodeRecord, playhead: number): number | undefined {
    return getTimelineOpacity(node.id, node.timelinePresence, playhead, {
      active: TIMELINE_ACTIVE_NODE_OPACITY,
      past: TIMELINE_PAST_NODE_OPACITY,
      future: TIMELINE_FUTURE_NODE_OPACITY,
    })
  }

  private getTimelineEdgeOpacity(edge: TopoEdge, edgeIndex: number, playhead: number | undefined = this.currentLayout.timelinePlayhead): number | undefined {
    const edgeId = String(edge.id || `${edge.source}-${edge.target}-${edge.label || ''}`)
    return getTimelineOpacity(edgeId, this.edgeTimelinePresence[edgeIndex], Number(playhead), {
      active: TIMELINE_ACTIVE_EDGE_OPACITY,
      past: TIMELINE_PAST_EDGE_OPACITY,
      future: TIMELINE_FUTURE_EDGE_OPACITY,
    })
  }

  private applyTimelinePlayhead(playhead: number, syncLabels = true): void {
    if (!this.graph) return

    const timelinePlaying = this.currentLayout.timelinePlaying === true
    const forceDetailedIcons = this.currentLayout.forceDetailedView === true && this.nodes.length <= 120
    this.nodes.forEach((node) => {
      const timelineOpacity = this.getTimelineNodeOpacity(node, playhead)
      if (timelineOpacity === undefined) {
        node.opacity = node.baseOpacity
        node.disableVectorIcon = node.baseDisableVectorIcon || (timelinePlaying && !forceDetailedIcons)
        node.sizeScale = node.baseSizeScale
        return
      }
      node.opacity = timelineOpacity
      node.disableVectorIcon = node.baseDisableVectorIcon || (
        !forceDetailedIcons && (timelinePlaying || timelineOpacity < 0.62)
      )
      node.sizeScale = timelineOpacity >= 0.62 ? 1 : 0.42
    })

    this.writePointBaseColors()
    this.applyAdaptiveNodeSizes(true, false)
    this.buildLinkColorsOnly()
    this.graph.render()
    if (syncLabels) this.scheduleLabelSync()
  }

  private resetTimelineVisualState(syncLabels = true): void {
    if (!this.graph) return

    this.nodes.forEach((node) => {
      node.opacity = node.baseOpacity
      node.disableVectorIcon = node.baseDisableVectorIcon
      node.sizeScale = node.baseSizeScale
    })

    this.writePointBaseColors()
    this.applyAdaptiveNodeSizes(true, false)
    this.buildLinkColorsOnly()
    this.graph.render()
    if (syncLabels) this.scheduleLabelSync()
  }

  private buildPointBaseRgb(): void {
    const rgb = new Float32Array(this.nodes.length * 3)
    this.nodes.forEach((n, i) => {
      const [r, g, b] = ensureVisibleOnLight(...parseColor(n.iconFill || n.color))
      const o = i * 3
      rgb[o] = r
      rgb[o + 1] = g
      rgb[o + 2] = b
    })
    this.pointBaseRgb = rgb
  }

  private writePointBaseColors(): void {
    const length = this.nodes.length * 4
    const buf = this.pointBaseColors.length === length
      ? this.pointBaseColors
      : new Float32Array(length)
    if (this.pointBaseRgb.length !== this.nodes.length * 3) this.buildPointBaseRgb()
    this.nodes.forEach((n, i) => {
      const o = i * 4
      const c = i * 3
      buf[o] = this.pointBaseRgb[c]
      buf[o + 1] = this.pointBaseRgb[c + 1]
      buf[o + 2] = this.pointBaseRgb[c + 2]
      buf[o + 3] = n.opacity
    })
    this.pointBaseColors = buf
  }

  private buildPointColors(): void {
    this.writePointBaseColors()
    this.pointColors = new Float32Array(this.pointBaseColors)
    this.graph!.setPointColors(this.pointColors)
  }

  private buildPointSizes(): void {
    this.applyAdaptiveNodeSizes(true, false)
  }

  private scheduleVisualScaleSync(): void {
    if (this.visualScaleRafId !== null) return
    this.visualScaleRafId = requestAnimationFrame(() => {
      this.visualScaleRafId = null
      this.applyAdaptiveNodeSizes()
    })
  }

  private getAdaptiveNodeProfile(): {
    pointSize: number
    minSize: number
    maxSize: number
    iconThreshold: number
    iconInset: number
    key: string
  } {
    const count = this.nodes.length
    if (this.currentLayout.forceDetailedView === true && count <= 120) {
      let minSize = 21
      let maxSize = 40
      let pointSize = 30
      if (count <= 24) {
        minSize = 28
        maxSize = 48
        pointSize = 38
      } else if (count <= 60) {
        minSize = 24
        maxSize = 44
        pointSize = 34
      }
      return {
        pointSize,
        minSize,
        maxSize,
        iconThreshold: 0,
        iconInset: 0,
        key: `focused-detail:${count}:${pointSize}:${minSize}:${maxSize}`,
      }
    }

    if (this.currentLayout.adaptiveNodeSize === false) {
      return {
        pointSize: IMAGE_SIZE,
        minSize: POINT_SIZE_BASE,
        maxSize: POINT_SIZE_MAX,
        iconThreshold: 0,
        iconInset: 0,
        key: `fixed:${IMAGE_SIZE}`,
      }
    }

    const range = this.currentLayout.nodeSizeRange

    const layoutKind = this.isGraphvizMode(this.currentLayout)
      ? 'graphviz'
      : this.isRelationsMode(this.currentLayout)
        ? 'relations'
        : this.isForceMode(this.currentLayout)
          ? 'force'
          : 'clustered'

    let defaultMin: number
    let defaultMax: number
    let zoomOffset: number
    let zoomSpan: number
    let iconFactor: number
    let iconFloor: number
    let iconInset: number

    if (layoutKind === 'graphviz') {
      defaultMin = count >= 2500 ? 4.2 : count >= 1000 ? 5.6 : count >= 300 ? 8.8 : 12.0
      defaultMax = count >= 2500 ? 20.0 : count >= 1000 ? 24.0 : 34.0
      zoomOffset = -0.05
      zoomSpan = 2.85
      iconFactor = count >= 2500 ? 0.72 : 0.46
      iconFloor = count >= 2500 ? 18.5 : 13.5
      iconInset = count >= 2500 ? 1.75 : 0.75
    } else if (layoutKind === 'relations') {
      defaultMin = count >= 2500 ? 6.0 : count >= 1000 ? 7.5 : count >= 300 ? 10.5 : 14.0
      defaultMax = count >= 2500 ? 30.0 : count >= 1000 ? 34.0 : 40.0
      zoomOffset = 0.46
      zoomSpan = 2.1
      iconFactor = count >= 2500 ? 0.48 : 0.32
      iconFloor = count >= 2500 ? 16.0 : 13.0
      iconInset = count >= 2500 ? 0.6 : 0
    } else if (layoutKind === 'clustered') {
      defaultMin = count >= 2500 ? 7.0 : count >= 1000 ? 8.6 : count >= 300 ? 11.5 : 14.0
      defaultMax = count >= 2500 ? 34.0 : count >= 1000 ? 38.0 : 42.0
      zoomOffset = 0.56
      zoomSpan = 1.9
      iconFactor = count >= 2500 ? 0.46 : 0.30
      iconFloor = count >= 2500 ? 16.0 : 12.5
      iconInset = count >= 2500 ? 0.4 : 0
    } else {
      defaultMin = count >= 2500 ? 3.2 : count >= 1000 ? 5.0 : count >= 300 ? 8.0 : 12.0
      defaultMax = count >= 2500 ? 24.5 : count >= 1000 ? 30.0 : 38.0
      zoomOffset = -0.24
      zoomSpan = 2.9
      iconFactor = count >= 2500 ? 0.68 : 0.46
      iconFloor = count >= 2500 ? 18.5 : 13.5
      iconInset = count >= 2500 ? 0.5 : 0.5
    }

    let minSize = range?.[0] ?? defaultMin
    let maxSize = range?.[1] ?? defaultMax
    if (maxSize < minSize) [minSize, maxSize] = [maxSize, minSize]

    const graphZoom = this.graph?.getZoomLevel()
    const zoom = Math.max(0.015, Number.isFinite(graphZoom) ? graphZoom! : this.zoomLevel || 1)
    const baseline = this.zoomBaseline > 0 ? this.zoomBaseline : zoom
    const relativeZoom = clamp(zoom / Math.max(0.015, baseline), 0.25, 16)
    const zoomProgress = smoothstep((Math.log2(relativeZoom) + zoomOffset) / zoomSpan)
    const pointSize = roundToHalf(minSize + (maxSize - minSize) * zoomProgress)
    const iconThreshold = Math.max(
      IMAGE_VISIBILITY_MIN_SIZE,
      iconFloor,
      minSize + (maxSize - minSize) * iconFactor,
    )

    return {
      pointSize,
      minSize,
      maxSize,
      iconThreshold,
      iconInset,
      key: `${layoutKind}:${count}:${pointSize}:${roundToHalf(minSize)}:${roundToHalf(maxSize)}:${roundToHalf(iconThreshold)}:${roundToHalf(iconInset)}`,
    }
  }

  private getNodeDegreeFactor(node: NodeRecord): number {
    const t = clamp((node.size - POINT_SIZE_BASE) / Math.max(1, POINT_SIZE_MAX - POINT_SIZE_BASE), 0, 1)
    const layoutKind = this.isForceMode(this.currentLayout)
      ? 'force'
      : this.isGraphvizMode(this.currentLayout)
        ? 'graphviz'
        : this.isRelationsMode(this.currentLayout)
          ? 'relations'
          : 'clustered'
    const defaultBoost = layoutKind === 'force' ? 0.08 : 0.22
    const bucketBoost = layoutKind === 'force' ? 0.18 : 0.42
    const boost = this.currentLayout.nodeSizeMode === 'degree-bucket' ? bucketBoost : defaultBoost
    const densityDamp = this.nodes.length >= 2500 ? 0.72 : 1
    return 1 + t * boost * densityDamp
  }

  private applyAdaptiveNodeSizes(force = false, shouldRender = true): void {
    if (!this.graph) return

    const profile = this.getAdaptiveNodeProfile()
    const hasIcons = this.pointImageIndices.length === this.nodes.length && this.nodes.length > 0
    const key = `${profile.key}:${this.currentLayout.nodeSizeMode ?? 'default'}:${hasIcons ? 'icons' : 'points'}`
    if (!force && key === this.lastVisualScaleKey) return

    const colorSource = this.pointBaseColors.length ? this.pointBaseColors : this.pointColors
    const pointSizes = this.pointSizes.length === this.nodes.length
      ? this.pointSizes
      : new Float32Array(this.nodes.length)
    const pointColors = this.pointColors.length === colorSource.length
      ? this.pointColors
      : new Float32Array(colorSource.length)
    pointColors.set(colorSource)
    const imageIndices = hasIcons ? new Float32Array(this.nodes.length) : this.pointImageIndices
    const imageSizes = hasIcons ? new Float32Array(this.nodes.length) : this.pointImageSizes

    this.nodes.forEach((node, index) => {
      const screenSize = roundToHalf(clamp(
        profile.pointSize * this.getNodeDegreeFactor(node) * (node.sizeScale || 1),
        profile.minSize,
        profile.maxSize,
      ))
      pointSizes[index] = screenSize
      node.screenSize = screenSize
      node.vectorIconVisible = !node.disableVectorIcon && screenSize >= profile.iconThreshold

      if (node.vectorIconVisible) {
        pointColors[index * 4 + 3] = 0
      }
      if (this.activeFilterKeys && !this.activeFilterKeys.has(node.cluster)) {
        pointColors[index * 4 + 3] = 0
      }

      if (hasIcons) {
        const showIcon = node.vectorIconVisible
        imageIndices[index] = showIcon ? this.pointImageIndices[index] : -1
        imageSizes[index] = showIcon
          ? Math.max(0, roundToHalf(screenSize - profile.iconInset))
          : screenSize
      }
    })

    this.currentPointScreenSize = profile.pointSize
    this.pointSizes = pointSizes
    this.pointColors = pointColors
    if (hasIcons) this.pointImageSizes = imageSizes
    this.graph.setPointSizes(pointSizes)
    this.graph.setPointColors(pointColors)
    if (hasIcons) {
      this.graph.setPointImageIndices(imageIndices)
      this.graph.setPointImageSizes(imageSizes)
    }
    this.lastVisualScaleKey = key

    if (shouldRender) {
      this.graph.render()
      this.scheduleLabelSync()
    }
  }

  private getBaseLinkWidth(): number {
    return LINK_WIDTH * clamp(this.currentLayout.linkWidthScale ?? 1, 0.6, 3)
  }

  private getBaseLinkOpacity(): number {
    return clamp(this.currentLayout.linkOpacity ?? LINK_OPACITY, 0.08, 1)
  }

  private getHoverLinkWidth(): number {
    return Math.max(LINK_HOVER_WIDTH, this.getBaseLinkWidth() + 3.4)
  }

  private buildLinks(): void {
    this.validEdgeIndices = []
    const pairs: number[] = []
    const colors: number[] = []
    const baseColors: number[] = []
    const baseOpacities: number[] = []
    const linkOpacity = this.getBaseLinkOpacity()
    const linkWidth = this.getBaseLinkWidth()

    this.edges.forEach((e, eIdx) => {
      const si = this.nodeIdToIndex.get(e.source)
      const ti = this.nodeIdToIndex.get(e.target)
      if (si === undefined || ti === undefined) return
      this.validEdgeIndices.push(eIdx)
      pairs.push(si, ti)

      const sn = this.nodeById.get(e.source)
      const [sr, sg, sb] = sn
        ? ensureVisibleOnLight(...parseColor(sn.iconFill || sn.color))
        : parseColor(LINK_DEFAULT_COLOR)
      const baseOpacity = readOpacity(e.style?.opacity, e.style?.timelineOpacity)
      const br = sr * 0.45 + 0.55 * 0.58
      const bg = sg * 0.45 + 0.55 * 0.64
      const bb = sb * 0.45 + 0.55 * 0.72
      baseColors.push(br, bg, bb)
      baseOpacities.push(baseOpacity)
      const edgeOpacity = this.getTimelineEdgeOpacity(e, eIdx) ?? baseOpacity
      colors.push(
        br,
        bg,
        bb,
        linkOpacity * edgeOpacity,
      )
    })

    this.graph!.setLinks(new Float32Array(pairs))
    const lc = new Float32Array(colors)
    this.linkColors = lc
    this.linkBaseColors = new Float32Array(baseColors)
    this.linkBaseOpacities = new Float32Array(baseOpacities)
    this.graph!.setLinkColors(lc)
    const widths = new Float32Array(this.validEdgeIndices.length)
    widths.fill(linkWidth)
    this.linkWidths = widths
    this.graph!.setLinkWidths(widths)
    this.graph!.setLinkArrows(new Array(this.validEdgeIndices.length).fill(true))
  }

  private buildLinkColorsOnly(): void {
    if (!this.graph || this.validEdgeIndices.length === 0) return
    const linkOpacity = this.getBaseLinkOpacity()
    const lc = this.linkColors.length === this.validEdgeIndices.length * 4
      ? this.linkColors
      : new Float32Array(this.validEdgeIndices.length * 4)
    this.validEdgeIndices.forEach((edgeIndex, linkIndex) => {
      const edge = this.edges[edgeIndex]
      const colorOffset = linkIndex * 3
      const o = linkIndex * 4
      const edgeOpacity = this.getTimelineEdgeOpacity(edge, edgeIndex) ?? this.linkBaseOpacities[linkIndex] ?? 1
      lc[o] = this.linkBaseColors[colorOffset] ?? 0.58
      lc[o + 1] = this.linkBaseColors[colorOffset + 1] ?? 0.64
      lc[o + 2] = this.linkBaseColors[colorOffset + 2] ?? 0.72
      lc[o + 3] = linkOpacity * edgeOpacity
    })
    this.linkColors = lc
    this.graph.setLinkColors(lc)
  }

  private buildClusterBuffers(): void {
    if (!this.graph || this.clusters.length === 0) return
    const pointClusters: (number | undefined)[] = new Array(this.nodes.length).fill(undefined)
    const m = new Map<string, number>()
    this.clusters.forEach((c, i) => m.set(c.key, i))
    this.nodes.forEach((n, i) => { const ci = m.get(n.cluster); if (ci !== undefined) pointClusters[i] = ci })
    this.graph.setPointClusters(pointClusters)

    const clusterPositions: Array<number | undefined> = []
    this.clusters.forEach((cluster) => {
      const position = this.clusterPositions.get(cluster.key)
      clusterPositions.push(position?.[0], position?.[1])
    })
    this.graph.setClusterPositions(clusterPositions)

    const strength = new Float32Array(this.nodes.length)
    this.nodes.forEach((node, index) => {
      const cluster = this.clusterByKey.get(node.cluster)
      const count = cluster?.count ?? 1
      strength[index] = Math.max(0.45, Math.min(1, 0.72 + count / 300))
    })
    this.graph.setPointClusterStrength(strength)
  }

  /* ─────────── Internal: Hover with transition ─────────── */

  private cancelHoverTimer(): void {
    if (this.hoverTimer !== null) { clearTimeout(this.hoverTimer); this.hoverTimer = null }
  }

  private cancelHoverFade(): void {
    if (this.hoverFadeRaf !== null) { cancelAnimationFrame(this.hoverFadeRaf); this.hoverFadeRaf = null }
    this.hoverFadeStep = 0
  }

  private scheduleHover(nodeId: string): void {
    if (this.hover.hoveredNodeId === nodeId && this.hover.hoverProgress > 0) return
    this.cancelHoverTimer()
    this.hoverTimer = setTimeout(() => {
      this.hoverTimer = null
      if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
      this.cancelHoverFade()
      this.hover.hoveredLinkIndex = undefined
      this.hover.hoveredNodeId = nodeId
      this.hover.hoveredNeighborIds = this.neighbors.get(nodeId) ?? new Set()
      this.hover.screenPosition = this.lastPointerPosition
      if (this.shouldFocusOnHover()) {
        this.highlightNode(nodeId)
        this.startHoverFadeIn()
      } else {
        this.hover.hoverProgress = 1
      }
      this.callbacks.onHoverChange?.(this.hover)
      if (this.shouldFocusOnHover()) this.scheduleLabelSync()
    }, HOVER_ENTER_DELAY)
  }

  private scheduleHoverLeave(): void {
    this.cancelHoverTimer()
    this.hoverTimer = setTimeout(() => {
      this.hoverTimer = null
      if (this.hover.lockedNodeId || this.hover.lockedLinkIndex !== undefined) return
      if (this.shouldFocusOnHover()) this.startHoverFadeOut()
      else this.clearTransientHover()
    }, HOVER_LEAVE_DELAY)
  }

  private clearTransientHover(): void {
    this.cancelHoverTimer()
    this.cancelHoverFade()
    this.hover.hoveredNodeId = undefined
    this.hover.hoveredNeighborIds = undefined
    this.hover.hoveredLinkIndex = undefined
    this.hover.hoverProgress = 0
    this.hover.screenPosition = undefined
    this.callbacks.onHoverChange?.(this.hover)
    if (this.shouldFocusOnHover()) this.scheduleLabelSync()
  }

  private resetVisualState(): void {
    this.graph?.unselectPoints()
    this.graph?.setConfig({ focusedPointIndex: undefined })
    this.graph?.setPointColors(this.pointColors)
    this.graph?.setLinkColors(this.linkColors)
    this.graph?.setLinkWidths(this.linkWidths)
    this.graph?.render()
  }

  private startHoverFadeIn(): void {
    this.cancelHoverFade()
    this.hoverFadeStep = 0
    const step = () => {
      this.hoverFadeStep++
      const t = Math.min(this.hoverFadeStep / HOVER_FADE_STEPS, 1)
      this.hover.hoverProgress = t
      this.applyHoverColors(t)
      this.callbacks.onHoverChange?.(this.hover)
      this.scheduleLabelSync()
      if (t < 1) this.hoverFadeRaf = requestAnimationFrame(step)
      else this.hoverFadeRaf = null
    }
    this.hoverFadeRaf = requestAnimationFrame(step)
  }

  private startHoverFadeOut(): void {
    this.cancelHoverFade()
    this.hoverFadeStep = HOVER_FADE_STEPS
    const step = () => {
      this.hoverFadeStep--
      const t = Math.max(this.hoverFadeStep / HOVER_FADE_STEPS, 0)
      this.hover.hoverProgress = t
      if (t <= 0) {
        this.hover.hoveredNodeId = undefined
        this.hover.hoveredNeighborIds = undefined
        this.hover.screenPosition = undefined
        this.resetVisualState()
        this.hoverFadeRaf = null
      } else {
        this.applyHoverColors(t)
        this.hoverFadeRaf = requestAnimationFrame(step)
      }
      this.callbacks.onHoverChange?.(this.hover)
      this.scheduleLabelSync()
    }
    this.hoverFadeRaf = requestAnimationFrame(step)
  }

  private applyHoverColors(t: number): void {
    if (!this.graph || !this.hover.hoveredNodeId) return

    const hId = this.hover.hoveredNodeId
    const neighbors = this.hover.hoveredNeighborIds ?? new Set<string>()
    const dimAlpha = 1.0 - t * 0.82

    const pc = new Float32Array(this.pointColors)
    for (let i = 0; i < this.nodes.length; i++) {
      const n = this.nodes[i]
      if (n.id !== hId && !neighbors.has(n.id)) {
        pc[i * 4 + 3] *= dimAlpha
      }
    }
    this.graph.setPointColors(pc)

    const lc = new Float32Array(this.linkColors)
    const lw = new Float32Array(this.linkWidths)
    const baseLinkOpacity = this.getBaseLinkOpacity()
    const baseLinkWidth = this.getBaseLinkWidth()
    const hoverLinkWidth = this.getHoverLinkWidth()
    for (let i = 0; i < this.validEdgeIndices.length; i++) {
      const eIdx = this.validEdgeIndices[i]
      const edge = this.edges[eIdx]
      const isConnected = edge.source === hId || edge.target === hId
      if (isConnected) {
        lc[i * 4 + 3] = Math.min(1.0, baseLinkOpacity + t * 0.45)
        lw[i] = baseLinkWidth + (hoverLinkWidth - baseLinkWidth) * t
      } else {
        lc[i * 4 + 3] = baseLinkOpacity * dimAlpha
        lw[i] = baseLinkWidth
      }
    }
    this.graph.setLinkColors(lc)
    this.graph.setLinkWidths(lw)
    this.graph.render()
  }

  private applyLinkFocus(linkIndex: number, t: number): void {
    if (!this.graph) return
    const edgeIndex = this.validEdgeIndices[linkIndex]
    const edge = edgeIndex === undefined ? undefined : this.edges[edgeIndex]
    if (!edge) return

    const sourceIndex = this.nodeIdToIndex.get(edge.source)
    const targetIndex = this.nodeIdToIndex.get(edge.target)
    const pc = new Float32Array(this.pointColors)
    const lc = new Float32Array(this.linkColors)
    const lw = new Float32Array(this.linkWidths)
    const dimAlpha = 1 - t * 0.82
    const baseLinkOpacity = this.getBaseLinkOpacity()
    const baseLinkWidth = this.getBaseLinkWidth()
    const hoverLinkWidth = this.getHoverLinkWidth()

    for (let i = 0; i < this.nodes.length; i += 1) {
      const node = this.nodes[i]
      if (node.id !== edge.source && node.id !== edge.target) {
        pc[i * 4 + 3] *= dimAlpha
      }
    }

    for (let i = 0; i < this.validEdgeIndices.length; i += 1) {
      if (i === linkIndex) {
        lc[i * 4] = 0.31
        lc[i * 4 + 1] = 0.27
        lc[i * 4 + 2] = 0.9
        lc[i * 4 + 3] = LINK_ACTIVE_OPACITY
        lw[i] = hoverLinkWidth
      } else {
        lc[i * 4 + 3] = baseLinkOpacity * dimAlpha
        lw[i] = baseLinkWidth
      }
    }

    this.graph.setPointColors(pc)
    if (sourceIndex !== undefined && targetIndex !== undefined) {
      this.graph.unselectPoints()
      this.graph.setConfig({ focusedPointIndex: undefined })
    }
    this.graph.setLinkColors(lc)
    this.graph.setLinkWidths(lw)
    this.graph.render()
  }

  private getFocusedEdgeByLinkIndex(linkIndex: number): FocusedEdgeRecord | null {
    const edgeIndex = this.validEdgeIndices[linkIndex]
    const edge = edgeIndex === undefined ? undefined : this.edges[edgeIndex]
    if (!edge) return null
    const source = this.nodeById.get(edge.source)
    const target = this.nodeById.get(edge.target)
    return {
      linkIndex,
      edgeIndex,
      sourceId: edge.source,
      targetId: edge.target,
      sourceTitle: source?.title || edge.source,
      targetTitle: target?.title || edge.target,
      sourceType: source?.subTitle || source?.cluster || 'source',
      targetType: target?.subTitle || target?.cluster || 'target',
      label: edge.label || String(edge.data?.label || edge.data?.type || 'related_to'),
      color: source?.iconFill || LINK_DEFAULT_COLOR,
    }
  }

  /* ─────────── Internal: Label sync ─────────── */

  private scheduleLabelSync(): void {
    if (this.labelSyncRafId !== null) return
    this.labelSyncRafId = requestAnimationFrame(() => {
      this.labelSyncRafId = null
      this.callbacks.onSimulationTick?.()
    })
  }
}
