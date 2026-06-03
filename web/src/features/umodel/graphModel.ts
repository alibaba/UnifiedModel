import type { Graphviz } from '@hpcc-js/wasm-graphviz'
import { Position, type Edge, type Node } from '@xyflow/react'
import type { UModelElement } from '../../api/types'
import {
  aliasForElements,
  colorForKind,
  columnForKind,
  elementKey,
  endpointId,
  entityLinkTypeForEdge,
  isEntitySetLinkElement,
  isLinkElement,
  nodeWidth,
  tagCountForElement,
  tagsForElement,
  titleForElement,
  type DraftStatus,
  type EntitySetLinkDisplay,
  type KindColor,
} from './model'

export interface GraphActions {
  onSelect: (element: UModelElement) => void
  onFocus: (element: UModelElement) => void
  onConnect: (element: UModelElement) => void
  onCopy: (element: UModelElement) => void
  onCopyCascade: (element: UModelElement) => void
  onDelete: (element: UModelElement, cascade: boolean) => void
}

export interface UModelNodeData extends Record<string, unknown> {
  element: UModelElement
  title: string
  name: string
  domain: string
  kind: string
  color: KindColor
  tags: string[]
  totalTagCount: number
  actions: GraphActions
  draftStatus?: DraftStatus
}

export interface UModelEdgeData extends Record<string, unknown> {
  element: UModelElement
  title: string
  kind: string
  sourceTitle: string
  targetTitle: string
  sourceKind: string
  targetKind: string
  sourceColor: string
  targetColor: string
  draftStatus?: DraftStatus
}

export interface GraphModel {
  nodes: Array<Node<UModelNodeData>>
  edges: Array<Edge<UModelEdgeData>>
}

let graphvizPromise: Promise<Graphviz> | null = null

export function buildGraph(
  elements: UModelElement[],
  actions: GraphActions,
  draftStatusById: Map<string, DraftStatus>,
  entitySetLinkDisplay: EntitySetLinkDisplay,
): GraphModel {
  const nodeElements = elements.filter(
    (element) => !isLinkElement(element) || (entitySetLinkDisplay === 'relative_link' && isEntitySetLinkElement(element)),
  )
  const linkElements = elements.filter(isLinkElement)
  const alias = aliasForElements(nodeElements)
  const nodeIds = new Set(nodeElements.map(elementKey))
  const nodeById = new Map(nodeElements.map((element) => [elementKey(element), element]))
  const validEdges: Array<{
    id: string
    element: UModelElement
    source: string
    target: string
    sourceElement: UModelElement
    targetElement: UModelElement
    kind?: string
  }> = []

  for (const element of linkElements) {
    const src = endpointId((element.spec || {}).src)
    const dest = endpointId((element.spec || {}).dest)
    const source = src ? alias.get(src) || src : ''
    const target = dest ? alias.get(dest) || dest : ''
    const sourceElement = nodeById.get(source)
    const targetElement = nodeById.get(target)
    if (!source || !target || !nodeIds.has(source) || !nodeIds.has(target) || !sourceElement || !targetElement) continue
    const key = elementKey(element)
    if (isEntitySetLinkElement(element) && entitySetLinkDisplay === 'relative_link' && nodeById.has(key)) {
      validEdges.push({
        id: `${key}:source`,
        element,
        source,
        target: key,
        sourceElement,
        targetElement: element,
        kind: '__temp__',
      })
      validEdges.push({
        id: `${key}:target`,
        element,
        source: key,
        target,
        sourceElement: element,
        targetElement,
        kind: '__temp__',
      })
    } else {
      validEdges.push({ id: key, element, source, target, sourceElement, targetElement })
    }
  }

  const nodes = fallbackLayoutNodes(nodeElements).map(({ element, position }) => {
    const key = elementKey(element)
    const color = colorForKind(element.kind)
    const entityLinkNode = entitySetLinkDisplay === 'relative_link' && isEntitySetLinkElement(element)
    const width = entityLinkNode ? Math.min(190, Math.max(68, entityLinkTypeForEdge(element).length * 8 + 38)) : nodeWidth
    const height = entityLinkNode ? 30 : 84
    return {
      id: key,
      type: 'umodel',
      position,
      draggable: true,
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      width,
      height,
      initialWidth: width,
      initialHeight: height,
      measured: { width, height },
      handles: [
        { id: 'source', nodeId: key, type: 'source' as const, position: Position.Right, x: width, y: height / 2, width: 1, height: 1 },
        { id: 'target', nodeId: key, type: 'target' as const, position: Position.Left, x: 0, y: height / 2, width: 1, height: 1 },
      ],
      data: {
        element,
        title: titleForElement(element),
        name: element.name || key,
        domain: element.domain || 'unknown',
        kind: element.kind,
        color,
        tags: tagsForElement(element),
        totalTagCount: tagCountForElement(element),
        actions,
        draftStatus: draftStatusById.get(key),
      },
    }
  })

  const edges: Array<Edge<UModelEdgeData>> = validEdges.map(({ id, element, source, target, sourceElement, targetElement, kind }) => ({
    id,
    type: 'umodel',
    source,
    target,
    sourceHandle: 'source',
    targetHandle: 'target',
    data: {
      element,
      title: titleForElement(element),
      kind: kind || element.kind,
      sourceTitle: source,
      targetTitle: target,
      sourceKind: sourceElement.kind,
      targetKind: targetElement.kind,
      sourceColor: colorForKind(sourceElement.kind).color,
      targetColor: colorForKind(targetElement.kind).color,
      draftStatus: draftStatusById.get(elementKey(element)),
    },
  }))

  return { nodes, edges }
}

function fallbackLayoutNodes(elements: UModelElement[]) {
  const grouped = new Map<number, UModelElement[]>()
  for (const element of elements) {
    const column = columnForKind(element.kind)
    if (!grouped.has(column)) grouped.set(column, [])
    grouped.get(column)!.push(element)
  }

  const result: Array<{ element: UModelElement; position: { x: number; y: number } }> = []
  let lane = 0
  const maxPerLane = 12
  for (const [, items] of [...grouped.entries()].sort((left, right) => left[0] - right[0])) {
    const sorted = [...items].sort((left, right) => titleForElement(left).localeCompare(titleForElement(right)))
    for (let start = 0; start < sorted.length; start += maxPerLane) {
      const chunk = sorted.slice(start, start + maxPerLane)
      chunk.forEach((element, index) => {
        const total = chunk.length
        result.push({ element, position: { x: lane * 330, y: (index - (total - 1) / 2) * 96 } })
      })
      lane += 1
    }
  }
  return result
}

export async function layoutGraphWithGraphviz(model: GraphModel): Promise<GraphModel> {
  if (model.nodes.length === 0) return model
  const graphviz = await getGraphviz()
  const dot = graphToDot(model)
  const raw = graphviz.layout(dot, 'json')
  const parsed = JSON.parse(raw) as { objects?: Array<{ name?: string; pos?: string }> }
  const positions = new Map<string, { x: number; y: number }>()

  for (const item of parsed.objects || []) {
    if (!item.name || !item.pos) continue
    const [xRaw, yRaw] = item.pos.split(',').map(Number)
    if (Number.isFinite(xRaw) && Number.isFinite(yRaw)) positions.set(item.name, { x: xRaw, y: yRaw })
  }
  if (positions.size === 0) return model

  return {
    ...model,
    nodes: model.nodes.map((node) => {
      const position = positions.get(node.id)
      if (!position) return node
      return { ...node, position }
    }),
  }
}

function getGraphviz(): Promise<Graphviz> {
  graphvizPromise ||= import('@hpcc-js/wasm-graphviz').then(({ Graphviz }) => Graphviz.load())
  return graphvizPromise
}

function graphToDot(model: GraphModel): string {
  const nodes = model.nodes
    .map((node) => {
      const width = node.data.kind === 'entity_set_link' ? 2 : 3
      const height = node.data.kind === 'entity_set_link' ? 0.5 : 1
      return `"${dotEscape(node.id)}" [label="${dotEscape(node.data.name)}", fixedsize=true, width=${width}, height=${height}];`
    })
    .join('\n')
  const edges = model.edges
    .map((edge) => `"${dotEscape(edge.source)}" -> "${dotEscape(edge.target)}" [minlen="1"];`)
    .join('\n')
  return `
digraph G {
  rankdir="LR";
  splines=true;
  graph[sep="1.5", mindist="2.5", oneblock=true, beautify=true, margin=0.5, nodesep=0.5, ranksep=2.5, rankdir="LR"];
  node [shape=box, style=filled, color=blue, penwidth=5, fontsize=12];
  edge [arrowhead=vee, color=black, penwidth=5];
  ${nodes}
  ${edges}
}`
}

function dotEscape(value: string) {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}
