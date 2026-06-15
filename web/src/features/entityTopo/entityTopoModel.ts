import type { QueryResult, UModelElement } from '../../api/types'

export const ENTITY_TOPO_LIMIT = 100
export const ENTITY_PROPERTY_LIMIT = ENTITY_TOPO_LIMIT

export type TopoLayoutAlgorithm = 'force' | 'grouped'
export type TopoLayoutDirection = 'LR' | 'TB'
export type TopoZoomLevel = 'mini' | 'compact' | 'full'
export type TopoSelection =
  | { kind: 'node'; node: EntityTopoNode }
  | { kind: 'edge'; edge: EntityTopoEdge }

export interface EntityEndpoint {
  id: string
  domain: string
  entityType: string
  entityId: string
  cluster: string
}

export interface TopoTypeVisual {
  color: string
  bg: string
  text: string
  label: string
  abbrev: string
}

export interface EntityTopoNode {
  id: string
  endpoint: EntityEndpoint
  title: string
  titleSource?: string
  subtitle: string
  searchText: string
  visual: TopoTypeVisual
  properties: Record<string, unknown>
  inDegree: number
  outDegree: number
  relationCount: number
}

export interface EntityTopoEdge {
  id: string
  source: string
  target: string
  relationType: string
  row: Record<string, unknown>
  searchText: string
}

export interface EntityTopoClusterMeta {
  cluster: string
  domain: string
  entityType: string
  displayName: string
  count: number
  visual: TopoTypeVisual
}

export interface AttributeTopoFilter {
  cluster: string
  field: string
  value: string
}

export interface AttributeValueAggregation {
  value: string
  count: number
}

export interface AttributeFieldAggregation {
  field: string
  count: number
  distinctCount: number
  values: AttributeValueAggregation[]
}

export interface AttributeClusterAggregation {
  cluster: string
  totalCount: number
  fields: AttributeFieldAggregation[]
}

export type AttributeAggregationIndex = Record<string, AttributeClusterAggregation>

export interface EntityTopoData {
  nodes: EntityTopoNode[]
  edges: EntityTopoEdge[]
  clusters: EntityTopoClusterMeta[]
  relationTypes: Array<{ type: string; count: number }>
  domains: Array<{ domain: string; count: number }>
  attributeAggregations: AttributeAggregationIndex
  limitInfo: {
    reached: boolean
    limit: number
    rowCount: number
  }
}

export interface EntityTopoFilters {
  domains: string[]
  types: string[]
  relations: string[]
  attributeFilters: AttributeTopoFilter[]
  focusIds: string[]
  searchText: string
  filterStacking: boolean
}

export interface EntityTopoDisplaySettings {
  layoutAlgorithm: TopoLayoutAlgorithm
  layoutDirection: TopoLayoutDirection
  showLabels: boolean
  showClusterLabels: boolean
  showMiniMap: boolean
  enableDrag: boolean
}

export const DEFAULT_ENTITY_TOPO_FILTERS: EntityTopoFilters = {
  domains: [],
  types: [],
  relations: [],
  attributeFilters: [],
  focusIds: [],
  searchText: '',
  filterStacking: false,
}

export const DEFAULT_ENTITY_TOPO_DISPLAY_SETTINGS: EntityTopoDisplaySettings = {
  layoutAlgorithm: 'force',
  layoutDirection: 'LR',
  showLabels: true,
  showClusterLabels: false,
  showMiniMap: true,
  enableDrag: false,
}

const typePalette: TopoTypeVisual[] = [
  { color: '#8b5cf6', bg: '#f3efff', text: '#6d28d9', label: 'Entity', abbrev: 'E' },
  { color: '#f97316', bg: '#fff1e6', text: '#c2410c', label: 'Entity', abbrev: 'E' },
  { color: '#22c55e', bg: '#e8f8ee', text: '#15803d', label: 'Entity', abbrev: 'E' },
  { color: '#ec4899', bg: '#fdf2f8', text: '#be185d', label: 'Entity', abbrev: 'E' },
  { color: '#0891b2', bg: '#e5f8fb', text: '#0e7490', label: 'Entity', abbrev: 'E' },
  { color: '#0ea5e9', bg: '#e0f2fe', text: '#0369a1', label: 'Entity', abbrev: 'E' },
  { color: '#6366f1', bg: '#eef2ff', text: '#4f46e5', label: 'Entity', abbrev: 'E' },
  { color: '#f43f5e', bg: '#fff1f2', text: '#be123c', label: 'Entity', abbrev: 'E' },
  { color: '#d946ef', bg: '#fae8ff', text: '#a21caf', label: 'Entity', abbrev: 'E' },
  { color: '#84cc16', bg: '#f1fadc', text: '#4d7c0f', label: 'Entity', abbrev: 'E' },
  { color: '#eab308', bg: '#fef9c3', text: '#a16207', label: 'Entity', abbrev: 'E' },
  { color: '#14b8a6', bg: '#dff8f5', text: '#0f766e', label: 'Entity', abbrev: 'E' },
  { color: '#64748b', bg: '#f1f5f9', text: '#475569', label: 'Entity', abbrev: 'E' },
  { color: '#06b6d4', bg: '#cffafe', text: '#0e7490', label: 'Entity', abbrev: 'E' },
  { color: '#ef4444', bg: '#fee2e2', text: '#b91c1c', label: 'Entity', abbrev: 'E' },
  { color: '#3b82f6', bg: '#dbeafe', text: '#1d4ed8', label: 'Entity', abbrev: 'E' },
  { color: '#a855f7', bg: '#f3e8ff', text: '#7e22ce', label: 'Entity', abbrev: 'E' },
  { color: '#10b981', bg: '#d1fae5', text: '#047857', label: 'Entity', abbrev: 'E' },
]

export function buildEntityTopoData(
  result: QueryResult,
  umodelElements: UModelElement[],
  entityRows: Array<Record<string, unknown>> = [],
): EntityTopoData {
  const umodelIndex = createUModelEntityIndex(umodelElements)
  const entityIndex = createEntityRowIndex([...inlineEntityRowsFromTopoRows(result.rows), ...entityRows])
  const nodeMap = new Map<string, EntityTopoNode>()
  const edgeMap = new Map<string, EntityTopoEdge>()
  const relationCounts = new Map<string, number>()

  result.rows.forEach((row, index) => {
    const source = endpointFromRow(row, 'src')
    const target = endpointFromRow(row, 'dest')
    if (!source || !target) return

    const sourceNode = ensureNode(nodeMap, source, umodelIndex, entityIndex)
    const targetNode = ensureNode(nodeMap, target, umodelIndex, entityIndex)
    sourceNode.outDegree += 1
    targetNode.inDegree += 1
    sourceNode.relationCount += 1
    targetNode.relationCount += 1

    const relationType = relationTypeFromRow(row)
    const relationProperties = relationPropertiesFromRow(row)
    relationCounts.set(relationType, (relationCounts.get(relationType) || 0) + 1)
    const edgeId = uniqueEdgeId(edgeMap, source.id, target.id, relationType, row, index)
    edgeMap.set(edgeId, {
      id: edgeId,
      source: source.id,
      target: target.id,
      relationType,
      row: relationProperties,
      searchText: compactSearchText([
        relationType,
        source.id,
        target.id,
        sourceNode.title,
        targetNode.title,
        source.cluster,
        target.cluster,
        relationProperties,
      ]),
    })
  })

  const nodeValues = [...nodeMap.values()]
  const clusters = buildClusterMetas(nodeValues, umodelIndex)

  const visualByCluster = new Map(clusters.map((cluster) => [cluster.cluster, cluster.visual]))
  const nodes = nodeValues
    .map((node) => ({
      ...node,
      visual: visualByCluster.get(node.endpoint.cluster) || node.visual,
    }))
    .sort((left, right) => left.endpoint.cluster.localeCompare(right.endpoint.cluster) || left.title.localeCompare(right.title))

  return {
    nodes,
    edges: [...edgeMap.values()],
    clusters,
    relationTypes: [...relationCounts.entries()]
      .map(([type, count]) => ({ type, count }))
      .sort((left, right) => right.count - left.count || left.type.localeCompare(right.type)),
    domains: recomputeDomains(nodes),
    attributeAggregations: buildAttributeAggregations(nodes),
    limitInfo: {
      reached: result.rows.length >= ENTITY_TOPO_LIMIT,
      limit: ENTITY_TOPO_LIMIT,
      rowCount: result.rows.length,
    },
  }
}

export function filterEntityTopoData(data: EntityTopoData, filters: EntityTopoFilters): EntityTopoData {
  const normalizedSearch = normalizeSearch(filters.searchText)
  const searchTerms = normalizedSearch ? normalizedSearch.split(/\s+/).filter(Boolean) : []
  const hasFocus = filters.focusIds.length > 0
  const focusData = hasFocus ? createDataSlice(data, relatedNodeIds(data.edges, filters.focusIds)) : data
  const shouldApplySecondaryFilters = !hasFocus || filters.filterStacking

  if (!shouldApplySecondaryFilters) return focusData

  const nodeMatches = new Set<string>()
  for (const node of focusData.nodes) {
    if (nodePassesScope(node, filters) && textMatches(node.searchText, searchTerms)) {
      nodeMatches.add(node.id)
    }
  }

  const relationMatches = (edge: EntityTopoEdge) => filters.relations.length === 0 || filters.relations.includes(edge.relationType)
  const edges = focusData.edges.filter((edge) => (
    (nodeMatches.has(edge.source) || nodeMatches.has(edge.target)) &&
    relationMatches(edge)
  ))
  const visibleNodeIds = new Set<string>()
  edges.forEach((edge) => {
    visibleNodeIds.add(edge.source)
    visibleNodeIds.add(edge.target)
  })
  if (filters.relations.length === 0) {
    for (const nodeId of nodeMatches) visibleNodeIds.add(nodeId)
  }

  const nodes = focusData.nodes.filter((node) => visibleNodeIds.has(node.id))
  const clusters = recomputeClusters(nodes, data.clusters)
  const relationTypes = recomputeRelationTypes(edges)
  const domains = recomputeDomains(nodes)

  return {
    ...data,
    nodes,
    edges,
    clusters,
    relationTypes,
    domains,
    attributeAggregations: buildAttributeAggregations(nodes),
  }
}

export function hasEntityTopoFilters(filters: EntityTopoFilters) {
  return (
    filters.domains.length > 0 ||
    filters.types.length > 0 ||
    filters.relations.length > 0 ||
    filters.attributeFilters.length > 0 ||
    filters.focusIds.length > 0 ||
    filters.searchText.trim().length > 0
  )
}

export function getAttributeFilterKey(filter: AttributeTopoFilter) {
  return `${filter.cluster}\n${filter.field}\n${filter.value}`
}

export function toggleFilterValue(values: string[], value: string) {
  return values.includes(value) ? values.filter((item) => item !== value) : [...values, value]
}

export function endpointLabel(endpoint: EntityEndpoint) {
  return [endpoint.domain, endpoint.entityType, endpoint.entityId].filter(Boolean).join('/')
}

export function relationTypeFromRow(row: Record<string, unknown>) {
  const relation = recordValue(row.relation)
  return (
    stringValue(row.__relation_type__) ||
    stringValue(relation?.__relation_type__) ||
    stringValue(relation?.relation_type) ||
    stringValue(relation?.type) ||
    (relation ? '' : stringValue(row.relation)) ||
    stringValue(row.type) ||
    'related'
  )
}

export function formatTopoValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function ensureNode(
  nodeMap: Map<string, EntityTopoNode>,
  endpoint: EntityEndpoint,
  umodelIndex: Map<string, { displayName: string }>,
  entityIndex: Map<string, Record<string, unknown>>,
) {
  const existing = nodeMap.get(endpoint.id)
  if (existing) return existing
  const meta = umodelIndex.get(endpoint.cluster)
  const properties = entityIndex.get(endpoint.id) || {}
  const titleMeta = titleFromProperties(properties, endpoint)
  const visual = visualForCluster(endpoint.cluster, meta?.displayName)
  const node: EntityTopoNode = {
    id: endpoint.id,
    endpoint,
    title: titleMeta.title,
    titleSource: titleMeta.source,
    subtitle: meta?.displayName || endpoint.entityType || endpoint.cluster,
    searchText: compactSearchText([endpoint.id, endpoint.domain, endpoint.entityType, endpoint.entityId, meta?.displayName, properties]),
    visual: {
      ...visual,
      label: meta?.displayName || endpoint.entityType || visual.label,
      abbrev: abbreviate(meta?.displayName || endpoint.entityType || endpoint.cluster, endpoint.domain),
    },
    properties,
    inDegree: 0,
    outDegree: 0,
    relationCount: 0,
  }
  nodeMap.set(endpoint.id, node)
  return node
}

function endpointFromRow(row: Record<string, unknown>, side: 'src' | 'dest'): EntityEndpoint | null {
  const prefix = side === 'src' ? '__src' : '__dest'
  const rawDomain = stringValue(row[`${prefix}_domain__`])
  const rawType = stringValue(row[`${prefix}_entity_type__`])
  const rawId = stringValue(row[`${prefix}_entity_id__`])
  if (rawDomain && rawType && rawId) {
    return createEndpoint(rawDomain, rawType, rawId)
  }

  const objectEndpoint = recordValue(row[side])
  if (objectEndpoint) {
    const endpoint = endpointFromEntityRow(objectEndpoint)
    if (endpoint) return endpoint
  }

  const raw = stringValue(row[side])
  if (!raw) return null
  const [domain = 'unknown', entityType = 'unknown', ...idParts] = raw.split('/')
  const entityId = idParts.join('/') || raw
  return createEndpoint(domain, entityType, entityId)
}

function createEndpoint(domain: string, entityType: string, entityId: string): EntityEndpoint {
  const cleanDomain = domain || 'unknown'
  const cleanEntityType = entityType || 'unknown'
  const cleanEntityId = entityId || 'unknown'
  return {
    id: `${cleanDomain}/${cleanEntityType}/${cleanEntityId}`,
    domain: cleanDomain,
    entityType: cleanEntityType,
    entityId: cleanEntityId,
    cluster: `${cleanDomain}@${cleanEntityType}`,
  }
}

function uniqueEdgeId(
  edgeMap: Map<string, EntityTopoEdge>,
  source: string,
  target: string,
  relationType: string,
  row: Record<string, unknown>,
  index: number,
) {
  const relation = recordValue(row.relation)
  const stable =
    stringValue(row.id) ||
    stringValue(row.__relation_id__) ||
    stringValue(relation?.id) ||
    stringValue(relation?.__relation_id__) ||
    `${source}->${relationType}->${target}`
  let candidate = stable
  let suffix = 1
  while (edgeMap.has(candidate)) {
    suffix += 1
    candidate = `${stable}#${suffix}`
  }
  return candidate || `edge-${index}`
}

function inlineEntityRowsFromTopoRows(rows: Array<Record<string, unknown>>) {
  const entityRows: Array<Record<string, unknown>> = []
  for (const row of rows) {
    const src = recordValue(row.src)
    const dest = recordValue(row.dest)
    if (src && endpointFromEntityRow(src)) entityRows.push(src)
    if (dest && endpointFromEntityRow(dest)) entityRows.push(dest)
  }
  return entityRows
}

function createEntityRowIndex(rows: Array<Record<string, unknown>>) {
  const index = new Map<string, Record<string, unknown>>()
  for (const row of rows) {
    const endpoint = endpointFromEntityRow(row)
    if (!endpoint) continue
    const properties = cleanEntityProperties(row)
    const existing = index.get(endpoint.id)
    index.set(endpoint.id, existing ? { ...existing, ...properties } : properties)
  }
  return index
}

function relationPropertiesFromRow(row: Record<string, unknown>) {
  return cleanRelationProperties(recordValue(row.relation) || row)
}

function cleanRelationProperties(row: Record<string, unknown>) {
  const properties: Record<string, unknown> = {}
  Object.entries(row).forEach(([key, value]) => {
    if (key === 'src' || key === 'dest') return
    if (key === 'relation' && recordValue(value)) return
    if (!isDisplayableProperty(key, value)) return
    properties[key] = value
  })
  return properties
}

function endpointFromEntityRow(row: Record<string, unknown>) {
  const domain = stringValue(row.__domain__) || stringValue(row.domain)
  const entityType = stringValue(row.__entity_type__) || stringValue(row.entity_type) || stringValue(row.type)
  const entityId = stringValue(row.__entity_id__) || stringValue(row.entity_id)
  if (!domain || !entityType || !entityId) return null
  return createEndpoint(domain, entityType, entityId)
}

function cleanEntityProperties(row: Record<string, unknown>) {
  const properties: Record<string, unknown> = {}
  Object.entries(row).forEach(([key, value]) => {
    if (!isDisplayableProperty(key, value)) return
    properties[key] = value
  })
  return properties
}

function isDisplayableProperty(key: string, value: unknown) {
  if (key.startsWith('__')) return false
  if (value === null || value === undefined) return false
  if (typeof value === 'string') return value.trim().length > 0
  if (typeof value === 'number' || typeof value === 'boolean') return true
  if (Array.isArray(value)) return value.length > 0
  return typeof value === 'object'
}

const titleFieldPriority = [
  'display_name',
  'displayName',
  'name',
  'title',
  'service_name',
  'serviceName',
  'app',
  'application',
  'operation_name',
  'operation',
  'endpoint',
  'database',
  'db_name',
  'topic',
  'model',
  'provider',
  'url',
  'host',
  'hostname',
  'pod_name',
  'instance',
  'instance_name',
]

const bareTitleFields = new Set(['display_name', 'displayName', 'name', 'title', 'service_name', 'serviceName'])

function titleFromProperties(properties: Record<string, unknown>, endpoint: EntityEndpoint) {
  for (const field of titleFieldPriority) {
    const raw = propertyStringValue(properties[field])
    if (!raw) continue
    return {
      title: truncateTitle(bareTitleFields.has(field) ? raw : `${field}: ${raw}`),
      source: field,
    }
  }

  const fallback = Object.entries(properties).find(([key, value]) => {
    if (/^(id|entity_id|uid|uuid)$/i.test(key)) return false
    if (/_id$/i.test(key)) return false
    return Boolean(propertyStringValue(value))
  })
  if (fallback) {
    return {
      title: truncateTitle(`${fallback[0]}: ${propertyStringValue(fallback[1])}`),
      source: fallback[0],
    }
  }

  return {
    title: shortenEntityId(endpoint.entityId || endpoint.id),
    source: 'entity_id',
  }
}

function propertyStringValue(value: unknown) {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value.trim()
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return ''
}

function truncateTitle(value: string) {
  const compact = value.replace(/\s+/g, ' ').trim()
  return compact.length > 52 ? `${compact.slice(0, 49)}...` : compact
}

function shortenEntityId(value: string) {
  if (value.length <= 22) return value
  return `${value.slice(0, 8)}...${value.slice(-6)}`
}

function buildClusterMetas(nodes: EntityTopoNode[], umodelIndex: Map<string, { displayName: string }>) {
  const counts = new Map<string, number>()
  for (const node of nodes) counts.set(node.endpoint.cluster, (counts.get(node.endpoint.cluster) || 0) + 1)
  return [...counts.keys()]
    .sort((left, right) => left.localeCompare(right))
    .map((cluster, index) => {
      const count = counts.get(cluster) || 0
      const [domain, entityType] = splitCluster(cluster)
      const meta = umodelIndex.get(cluster)
      const visual = visualForCluster(cluster, meta?.displayName, index)
      return {
        cluster,
        domain,
        entityType,
        displayName: meta?.displayName || entityType || cluster,
        count,
        visual: {
          ...visual,
          label: meta?.displayName || entityType || visual.label,
          abbrev: abbreviate(meta?.displayName || entityType || cluster, domain),
        },
      }
    })
    .sort((left, right) => right.count - left.count || left.displayName.localeCompare(right.displayName))
}

function createUModelEntityIndex(elements: UModelElement[]) {
  const index = new Map<string, { displayName: string }>()
  for (const element of elements) {
    if (element.kind !== 'entity_set') continue
    const displayName = displayNameForUModel(element)
    const domain = element.domain || 'unknown'
    const name = element.name || ''
    const keys = [
      umodelElementKey(element),
      `${domain}@${name}`,
      `${domain}/${name}`,
      name,
    ].filter(Boolean)
    for (const key of keys) index.set(key, { displayName })
  }
  return index
}

function umodelElementKey(element: UModelElement) {
  return [element.domain, element.name, element.kind].filter(Boolean).join('/')
}

function displayNameForUModel(element: UModelElement) {
  const metadata = (element as unknown as Record<string, unknown>).metadata as Record<string, unknown> | undefined
  const spec = element.spec || {}
  const displayName =
    localizedText(metadata?.display_name) ||
    localizedText((spec as Record<string, unknown>).display_name) ||
    localizedText(metadata?.description) ||
    localizedText((spec as Record<string, unknown>).description) ||
    element.name ||
    umodelElementKey(element)
  return String(displayName)
}

function localizedText(value: unknown): string {
  if (!value) return ''
  if (typeof value === 'string') return value
  if (typeof value !== 'object') return ''
  const record = value as Record<string, unknown>
  return stringValue(record.zh_cn) || stringValue(record.en_us) || stringValue(record.en) || ''
}

function visualForCluster(cluster: string, displayName?: string, paletteIndex?: number): TopoTypeVisual {
  const visual = typeof paletteIndex === 'number'
    ? typePalette[paletteIndex % typePalette.length]
    : typePalette[Math.abs(hashString(cluster)) % typePalette.length]
  return {
    ...visual,
    label: displayName || splitCluster(cluster)[1] || visual.label,
    abbrev: abbreviate(displayName || splitCluster(cluster)[1] || cluster, splitCluster(cluster)[0]),
  }
}

function splitCluster(cluster: string) {
  const [domain = '', entityType = ''] = cluster.split('@')
  return [domain, entityType] as const
}

function recomputeClusters(nodes: EntityTopoNode[], allClusters: EntityTopoClusterMeta[]) {
  const counts = new Map<string, number>()
  for (const node of nodes) counts.set(node.endpoint.cluster, (counts.get(node.endpoint.cluster) || 0) + 1)
  const metaByCluster = new Map(allClusters.map((meta) => [meta.cluster, meta]))
  return [...counts.entries()]
    .map(([cluster, count]) => {
      const existing = metaByCluster.get(cluster)
      if (existing) return { ...existing, count }
      const [domain, entityType] = splitCluster(cluster)
      const visual = visualForCluster(cluster, entityType)
      return {
        cluster,
        domain,
        entityType,
        displayName: entityType || cluster,
        count,
        visual,
      }
    })
    .filter((meta) => meta.cluster)
    .sort((left, right) => right.count - left.count || left.displayName.localeCompare(right.displayName))
}

function recomputeRelationTypes(edges: EntityTopoEdge[]) {
  const counts = new Map<string, number>()
  for (const edge of edges) counts.set(edge.relationType, (counts.get(edge.relationType) || 0) + 1)
  return [...counts.entries()]
    .map(([type, count]) => ({ type, count }))
    .sort((left, right) => right.count - left.count || left.type.localeCompare(right.type))
}

function recomputeDomains(nodes: EntityTopoNode[]) {
  const counts = new Map<string, number>()
  for (const node of nodes) counts.set(node.endpoint.domain, (counts.get(node.endpoint.domain) || 0) + 1)
  return [...counts.entries()]
    .map(([domain, count]) => ({ domain, count }))
    .sort((left, right) => right.count - left.count || left.domain.localeCompare(right.domain))
}

function createDataSlice(data: EntityTopoData, nodeIds: Set<string>): EntityTopoData {
  const nodes = data.nodes.filter((node) => nodeIds.has(node.id))
  const edgeIds = new Set(nodes.map((node) => node.id))
  const edges = data.edges.filter((edge) => edgeIds.has(edge.source) && edgeIds.has(edge.target))
  return {
    ...data,
    nodes,
    edges,
    clusters: recomputeClusters(nodes, data.clusters),
    relationTypes: recomputeRelationTypes(edges),
    domains: recomputeDomains(nodes),
    attributeAggregations: buildAttributeAggregations(nodes),
  }
}

function buildAttributeAggregations(nodes: EntityTopoNode[]): AttributeAggregationIndex {
  const clusterMaps = new Map<string, {
    totalCount: number
    fields: Map<string, { count: number; values: Map<string, number> }>
  }>()

  for (const node of nodes) {
    let cluster = clusterMaps.get(node.endpoint.cluster)
    if (!cluster) {
      cluster = { totalCount: 0, fields: new Map() }
      clusterMaps.set(node.endpoint.cluster, cluster)
    }
    cluster.totalCount += 1

    Object.entries(node.properties).forEach(([field, rawValue]) => {
      if (!isAttributeField(field, rawValue)) return
      const value = attributeValue(rawValue)
      if (!value) return
      let fieldMap = cluster!.fields.get(field)
      if (!fieldMap) {
        fieldMap = { count: 0, values: new Map() }
        cluster!.fields.set(field, fieldMap)
      }
      fieldMap.count += 1
      fieldMap.values.set(value, (fieldMap.values.get(value) || 0) + 1)
    })
  }

  const result: AttributeAggregationIndex = {}
  for (const [cluster, aggregation] of clusterMaps.entries()) {
    const fields = [...aggregation.fields.entries()]
      .map(([field, detail]) => {
        const values = [...detail.values.entries()]
          .map(([value, count]) => ({ value, count }))
          .sort((left, right) => right.count - left.count || left.value.localeCompare(right.value))
        return {
          field,
          count: detail.count,
          distinctCount: values.length,
          values: values.slice(0, 60),
        }
      })
      .filter((field) => {
        if (field.distinctCount === 0) return false
        if (field.distinctCount > 80 && field.distinctCount / Math.max(1, aggregation.totalCount) > 0.65) return false
        return true
      })
      .sort((left, right) => (
        right.count - left.count ||
        left.distinctCount - right.distinctCount ||
        left.field.localeCompare(right.field)
      ))
    result[cluster] = {
      cluster,
      totalCount: aggregation.totalCount,
      fields,
    }
  }
  return result
}

function isAttributeField(field: string, value: unknown) {
  if (/^(__|id$|entity_id$|uid$|uuid$)/i.test(field)) return false
  if (/_id$/i.test(field) && field !== 'trace_id') return false
  if (value === null || value === undefined) return false
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean'
}

function attributeValue(value: unknown) {
  const text = propertyStringValue(value)
  if (!text) return ''
  return text.length > 96 ? `${text.slice(0, 93)}...` : text
}

function nodePassesScope(node: EntityTopoNode, filters: EntityTopoFilters) {
  const hasTypeLikeFilter = filters.types.length > 0 || filters.attributeFilters.length > 0
  if (
    hasTypeLikeFilter &&
    !filters.types.includes(node.endpoint.cluster) &&
    !filters.attributeFilters.some((filter) => nodeMatchesAttributeFilter(node, filter))
  ) return false
  if (filters.domains.length > 0 && !filters.domains.includes(node.endpoint.domain)) return false
  return true
}

function nodeMatchesAttributeFilter(node: EntityTopoNode, filter: AttributeTopoFilter) {
  if (node.endpoint.cluster !== filter.cluster) return false
  return attributeValue(node.properties[filter.field]) === filter.value
}

function textMatches(searchText: string, terms: string[]) {
  if (terms.length === 0) return true
  const haystack = normalizeSearch(searchText)
  return terms.every((term) => haystack.includes(term))
}

function relatedNodeIds(edges: EntityTopoEdge[], focusIds: string[]) {
  const related = new Set(focusIds)
  const focus = new Set(focusIds)
  for (const edge of edges) {
    if (focus.has(edge.source) || focus.has(edge.target)) {
      related.add(edge.source)
      related.add(edge.target)
    }
  }
  return related
}

function compactSearchText(values: unknown[]) {
  const parts: string[] = []
  const visit = (value: unknown, depth: number) => {
    if (parts.length >= 80 || value == null || depth > 2) return
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
      const text = String(value).trim()
      if (text) parts.push(text)
      return
    }
    if (Array.isArray(value)) {
      value.slice(0, 10).forEach((item) => visit(item, depth + 1))
      return
    }
    if (typeof value === 'object') {
      Object.entries(value as Record<string, unknown>).slice(0, 24).forEach(([key, item]) => {
        if (!key.startsWith('__')) parts.push(key)
        visit(item, depth + 1)
      })
    }
  }
  values.forEach((value) => visit(value, 0))
  return parts.join(' ')
}

function normalizeSearch(value: string) {
  return value.trim().toLowerCase()
}

function stringValue(value: unknown) {
  if (value === null || value === undefined) return ''
  return String(value)
}

function recordValue(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function stripDomainPrefix(value: string, domain?: string) {
  if (domain) {
    const prefix = domain + '.'
    if (value.startsWith(prefix) && value.length > prefix.length) return value.slice(prefix.length)
  }
  return value
}

function abbreviate(value: string, domain?: string) {
  const effective = stripDomainPrefix(value, domain)
  const parts = effective.split(/[^a-zA-Z0-9]+/).filter(Boolean)
  if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase()
  const compact = effective.replace(/[^a-zA-Z0-9]/g, '')
  return (compact.slice(0, 2) || 'E').toUpperCase()
}

function hashString(value: string) {
  let hash = 0
  for (let index = 0; index < value.length; index += 1) {
    hash = ((hash << 5) - hash + value.charCodeAt(index)) | 0
  }
  return hash
}
