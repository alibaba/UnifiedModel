export type WorkspaceStatus = 'active' | 'deleted' | 'conflicted'

export interface Page<T> {
  items: T[]
  next_token?: string
}

export interface GraphStoreHealth {
  provider: string
  status: string
  message?: string
}

export interface HealthResponse {
  status: string
  graphstore: GraphStoreHealth
}

export interface WorkspacePaths {
  root: string
  tmp?: string
}

export interface WorkspaceMetadata {
  id: string
  name: string
  description?: string
  labels?: Record<string, string>
  config?: Record<string, Record<string, unknown>>
  paths: WorkspacePaths
  status: WorkspaceStatus
  resource_version: number
  created_at: string
  updated_at: string
  deleted_at?: string
}

export interface CreateWorkspaceRequest {
  id: string
  name?: string
  description?: string
  labels?: Record<string, string>
  config?: Record<string, Record<string, unknown>>
}

export interface UpdateWorkspaceRequest {
  name?: string
  description?: string
  labels?: Record<string, string>
  config?: Record<string, Record<string, unknown>>
  if_match_version?: number
  replace_labels?: boolean
  replace_config?: boolean
}

export interface ErrorDetail {
  field?: string
  reason?: string
  limit?: string
}

export interface ErrorEnvelope {
  error: {
    code: string
    message: string
    retryable: boolean
    details?: Record<string, string>
  }
}

export interface BatchItemResult {
  id?: string
  ok: boolean
  code?: string
  message?: string
  details?: ErrorDetail[]
}

export interface WriteResult {
  accepted: number
  failed: number
  items?: BatchItemResult[]
}

export interface ValidationResult {
  valid: boolean
  errors?: ErrorDetail[]
}

export interface UModelElement {
  kind: string
  domain: string
  name: string
  version?: string
  spec?: Record<string, unknown>
}

export interface UModelImportRequest {
  path: string
  common_schema_packs?: string[]
}

export interface UModelImportResult {
  workspace: string
  source: string
  imported: number
  skipped: number
  elements?: UModelElement[]
  errors?: ErrorDetail[]
}

export interface SampleImportResult {
  workspace: string
  sample: string
  umodel: UModelImportResult
  entities: WriteResult
  relations: WriteResult
  entity_count: number
  relation_count: number
}

export interface QueryRequest {
  query: string
  limit?: number
  timeout_ms?: number
  format?: string
  time_range?: {
    from?: string
    to?: string
  }
  parameters?: Record<string, unknown>
}

export interface QueryExplain {
  source?: '.umodel' | '.entity' | '.topo'
  provider?: string
  storage_provider?: string
  cypher_dialect?: string
  cypher_engine?: string
  pushdown?: string[]
  fallback?: string[]
  operators?: string[]
  depth?: number
  limit?: number
  timeout_ms?: number
  time_range_applied?: boolean
}

export interface QueryResult {
  columns: string[]
  rows: Array<Record<string, unknown>>
  page: {
    limit?: number
    page_token?: string
  }
  explain?: QueryExplain
}

export interface QueryExecuteResponse {
  code: string
  message: string
  success: boolean
  data: {
    data: unknown[][]
    header: string[]
    responseStatus: {
      result: string
      retryPolicy: string
      level: string
      statusItem: unknown[]
    }
  }
}

export interface EntityWriteBatch {
  workspace?: string
  idempotency_key?: string
  partial_success?: boolean
  entities: Array<Record<string, unknown>>
}

export interface RelationWriteBatch {
  workspace?: string
  idempotency_key?: string
  partial_success?: boolean
  relations: Array<Record<string, unknown>>
}

export interface ExpireRequest {
  workspace?: string
  ids: string[]
  reason?: string
}

export interface AgentTool {
  name: string
  description: string
  enabled: boolean
  requires_explicit_write_enable?: boolean
  input_schema?: unknown
  output_schema?: unknown
}

export interface AgentResource {
  uri: string
  name: string
  kind: string
  description: string
  mime_type: string
  read_only: boolean
}

export interface AgentNextAction {
  id: string
  title: string
  description: string
  tool: string
  query_api: {
    method: string
    path: string
    body: QueryRequest
  }
}

export interface AgentDiscovery {
  workspace: string
  tools: AgentTool[]
  resources: AgentResource[]
  next_actions?: AgentNextAction[]
}

export interface AgentResourceReadResult {
  uri: string
  mime_type: string
  content: unknown
}

export interface AgentToolCallResult {
  name: string
  ok: boolean
  output?: unknown
  error?: unknown
}
