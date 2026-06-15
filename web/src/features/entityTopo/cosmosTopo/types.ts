import type { Graph } from '@cosmos.gl/graph'
import type React from 'react'

export type TopoEngineType = 'sigma' | 'cosmos'
export type TopoLayoutMode = 'clustered' | 'force' | 'forceStatic' | 'graphviz' | 'relations' | 'graphvizFree'
export type TopoGraphvizEngine = 'dot' | 'sfdp' | 'neato' | 'fdp' | 'twopi' | 'circo' | 'osage'
export type TopoNodeSizeMode = 'default' | 'degree-bucket'
export type TopoHoverMode = 'auto' | 'panel' | 'focus'
export type TopoIconPreset =
  | 'default'
  | 'service'
  | 'application'
  | 'deployment'
  | 'node'
  | 'instance'
  | 'pod'
  | 'database'
  | 'disk'
  | 'redis'
  | 'network'
  | 'loadbalancer'
  | 'endpoint'

export interface TopoNode {
  id: string
  type?: string
  title?: string
  subTitle?: string
  label?: string
  style?: {
    size?: number
    fill?: string
    stroke?: string
    labelFill?: string
    labelFontSize?: number
    iconClass?: string
    iconUrl?: string
    iconPreset?: string
    lucideIcon?: string
    iconFill?: string
    icon?: string | Record<string, any>
    [key: string]: any
  }
  data?: Record<string, any>
  [key: string]: any
}

export interface TopoEdge {
  id: string
  source: string
  target: string
  label?: string
  style?: {
    stroke?: string
    strokeWidth?: number
    opacity?: number
    [key: string]: any
  }
  data?: Record<string, any>
}

export interface TopoData {
  nodes: TopoNode[]
  edges: TopoEdge[]
}

export interface TopoDataUpdateOptions {
  animationAnchorNodeId?: string
  animationType?: string
  disableAnimate?: boolean
}

export interface LayoutOptions {
  mode?: TopoLayoutMode
  clusterGap?: number
  nodeGap?: number
  nodeSizeMode?: TopoNodeSizeMode
  adaptiveNodeSize?: boolean
  nodeSizeRange?: [number, number]
  simulationDuration?: number
  simulationFriction?: number
  simulationGravity?: number
  simulationRepulsion?: number
  enableDrag?: boolean
  showLabels?: boolean
  showClusterLabels?: boolean
  showEdgeLabels?: boolean
  clusterByType?: boolean
  graphvizEngine?: TopoGraphvizEngine
  graphvizRankdir?: 'TB' | 'BT' | 'LR' | 'RL'
  hoverMode?: TopoHoverMode
  minZoomLevel?: number
  maxZoomLevel?: number
  preserveViewOnDataUpdate?: boolean
  skipFitViewOnDataUpdate?: boolean
  timelinePlayhead?: number
  timelinePlaying?: boolean
  focusActive?: boolean
  forceDetailedView?: boolean
  externalFocusMode?: boolean
  showMiniMap?: boolean
  linkWidthScale?: number
  linkOpacity?: number
}

export interface TopoGraphViewState {
  zoomLevel: number
  zoomBaseline?: number
  transform?: {
    x: number
    y: number
    k: number
  }
}

export interface HoverState {
  hoveredNodeId: string | undefined
  hoveredNeighborIds: Set<string> | undefined
  hoveredLinkIndex?: number
  lockedLinkIndex?: number
  lockedNodeId: string | undefined
  hoverProgress: number
  screenPosition?: [number, number]
}

export interface NodeRecord {
  id: string
  index: number
  cluster: string
  clusterIndex: number
  title: string
  subTitle: string
  color: string
  iconFill: string
  iconClass?: string
  iconUrl?: string
  iconPreset: TopoIconPreset
  lucideIcon?: string
  ringWidth: number
  vectorIconVisible: boolean
  disableVectorIcon: boolean
  degree: number
  opacity: number
  baseDisableVectorIcon: boolean
  baseOpacity: number
  baseSizeScale: number
  timelinePresence?: number[]
  sizeScale: number
  x: number
  y: number
  size: number
  screenSize: number
}

export interface CosmosVectorIconOverlayItem {
  id: string
  x: number
  y: number
  size: number
  color: string
  opacity: number
  label: string
  typeLabel: string
  domain?: string
  iconClass?: string
  iconUrl?: string
  iconPreset: TopoIconPreset
  lucideIcon?: string
  ringWidth: number
  active: boolean
  related: boolean
}

export interface CosmosNodeActionInfo {
  nodeId: string
  x: number
  y: number
  size: number
  color: string
}

export interface ClusterRecord {
  key: string
  label: string
  color: string
  iconFill: string
  count: number
  nodeIndices: number[]
}

export interface FocusedEdgeRecord {
  linkIndex: number
  edgeIndex: number
  sourceId: string
  targetId: string
  sourceTitle: string
  targetTitle: string
  sourceType: string
  targetType: string
  label: string
  color: string
}

export interface CosmosEngineCallbacks {
  onNodeClick?: (rawNode: TopoNode) => void
  onEdgeClick?: (sourceId: string, targetId: string, edgeIndex: number) => void
  onBackgroundClick?: () => void
  onHoverChange?: (hover: HoverState) => void
  onZoom?: (zoomLevel: number) => void
  onSimulationTick?: () => void
  onSimulationEnd?: () => void
  onReady?: () => void
}

export interface CosmosEngineConfig {
  callbacks?: CosmosEngineCallbacks
}

export interface TopoGraphProps {
  type: TopoEngineType
  data?: TopoData
  layout?: LayoutOptions
  handleNodeClick?: (context: any) => void
  handleEdgeClick?: (context: any) => void
  handleNodeIsolate?: (nodeId: string | null) => void
  handleCloseInfo?: () => void
  style?: React.CSSProperties
  className?: string
  children?: React.ReactNode
}

export interface ITopoGraph {
  setData: (data: TopoData, options?: TopoDataUpdateOptions) => void
  getData: () => TopoData
  resetData: () => void
  setLayout: (layout: LayoutOptions) => void
  setRuntimeLayout?: (layout: LayoutOptions) => void
  getLayout: () => LayoutOptions
  focusNodeView: (nodeId: string) => void
  resetView: () => void
  clearFocus: () => void
  isolateNode: (nodeId: string) => void
  clearIsolation: () => void
  setIsolationLayout: (mode: string) => void
  getIsolationLayout: () => string
  getFilterState: () => any
  setSelectedFilterKeys: (keys: string[]) => void
  toggleFilterKey: (key: string) => void
  clearFilter: () => void
  render: () => Promise<void>
  fitView: () => Promise<void>
  fitCenter: () => Promise<void>
  getViewState?: () => TopoGraphViewState | null
  restoreViewState?: (state: TopoGraphViewState, options?: { duration?: number }) => void
  setOptions: (options: Partial<TopoGraphProps>) => void
  on: (eventName: string, callback: (...args: any[]) => void) => ITopoGraph
  once: (eventName: string, callback: (...args: any[]) => void) => ITopoGraph
  off: (eventName: string, callback: (...args: any[]) => void) => ITopoGraph
}

export interface TopoGraphRef {
  getGraph: () => ITopoGraph
}

export interface ComputedTopoNode extends TopoNode {
  x: number
  y: number
  cluster: string
  clusterIndex: number
  level: number
  color: string
  size: number
}

export type { Graph as CosmosGraphInstance }
export type CosmosGraphvizEngine = TopoGraphvizEngine
