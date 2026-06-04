import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import Editor from '@monaco-editor/react'
import { ArrowRight, BarChart3, CalendarClock, ChevronLeft, ChevronRight, Database, Network, Play, Rows3, Search, SearchCode, SlidersHorizontal, Table2, Wand2, X } from 'lucide-react'
import type { editor as MonacoEditor } from 'monaco-editor'
import type { QueryExplain, QueryRequest, QueryResult } from '../../api/types'
import { UModelApi } from '../../api/client'
import { Badge, Button, EmptyState, IconButton, SegmentedControl } from '../../design/components'
import { useI18n, type MessageKey, type TFunction } from '../../i18n'
import { formatError, stringify } from '../../lib/json'
import { disableMonacoEditContext } from '../../lib/preloadMonaco'
import { EntityTopoGraphView } from '../entityTopo/EntityTopoGraphView'
import {
  DEFAULT_ENTITY_TOPO_DISPLAY_SETTINGS,
  buildEntityTopoData,
  endpointLabel,
  filterEntityTopoData,
  formatTopoValue,
  toggleFilterValue,
  type EntityTopoData,
  type EntityTopoDisplaySettings,
  type EntityTopoEdge,
  type EntityTopoNode,
  type TopoSelection,
  type TopoZoomLevel,
} from '../entityTopo/entityTopoModel'
import '../entityTopo/entityTopo.css'
import './query.css'

disableMonacoEditContext()

type QueryAction = 'execute' | 'explain'
type ResultView = 'table' | 'chart'

const resultPageSizes = [25, 50, 100]
const splMinEditorHeight = 41
const splMaxEditorHeight = 117

const examples = [
  { labelKey: 'query.examples.umodel', query: ".umodel with(kind='entity_set') | project domain,name,kind | sort domain,name | limit 20" },
  {
    labelKey: 'query.examples.entity',
    query:
      ".entity with(domain='devops', name='devops.service', query='checkout', mode='vector', topk=20) | project __category__,__domain__,__entity_type__,__entity_id__,__method__,__first_observed_time__,__last_observed_time__,__keep_alive_seconds__,display_name,status,owner",
  },
  { labelKey: 'query.examples.topo', query: '.topo | limit 20' },
  {
    labelKey: 'query.examples.direct',
    query:
      ".topo | graph-call getDirectRelations([(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | project src,relation,dest | limit 20",
  },
  {
    labelKey: 'query.examples.neighbors',
    query:
      ".topo | graph-call getNeighborNodes('full', 2, [(:\"devops@devops.service\" {__entity_id__: '10000000000000000000000000000101'})]) | limit 20",
  },
  {
    labelKey: 'query.examples.cypher',
    query:
      ".topo | graph-call cypher(`MATCH (src:``devops@devops.service`` {__entity_id__: '10000000000000000000000000000101'})-[r]->(dest) RETURN properties(src) AS src, properties(r) AS relation, properties(dest) AS dest LIMIT 20`) | limit 20",
  },
  {
    labelKey: 'query.examples.entitySetMethods',
    query: ".entity_set with(domain='devops', name='devops.service') | entity-call __list_method__()",
  },
  {
    labelKey: 'query.examples.entitySetDatasets',
    query: ".entity_set with(domain='devops', name='devops.service') | entity-call list_data_set(['metric_set', 'log_set', 'event_set'], true)",
  },
] as const satisfies ReadonlyArray<{ labelKey: MessageKey; query: string }>

export function QueryPage({ api, workspaceId }: { api: UModelApi; workspaceId: string }) {
  const { t } = useI18n()
  const [query, setQuery] = useState<string>(examples[0].query)
  const [timeRange, setTimeRange] = useState({ from: '', to: '' })
  const [result, setResult] = useState<QueryResult | null>(null)
  const [topoEntityRows, setTopoEntityRows] = useState<Array<Record<string, unknown>>>([])
  const [explain, setExplain] = useState<QueryExplain | null>(null)
  const [explainOpen, setExplainOpen] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [resultView, setResultView] = useState<ResultView>('table')
  const [splEditorHeight, setSplEditorHeight] = useState(splMinEditorHeight)

  const resultColumns = result?.columns.length ? result.columns : result?.rows[0] ? Object.keys(result.rows[0]) : []
  const topoData = useMemo(() => (result ? buildEntityTopoData(result, [], topoEntityRows) : null), [result, topoEntityRows])
  const canChart = Boolean(topoData && topoData.nodes.length > 0 && topoData.edges.length > 0)

  async function run(kind: QueryAction) {
    setBusy(true)
    setError('')
    try {
      const request: QueryRequest = {
        query,
      }
      const from = toIsoOrUndefined(timeRange.from)
      const to = toIsoOrUndefined(timeRange.to)
      if (from || to) request.time_range = { from, to }

      if (kind === 'execute') {
        const [next, nextExplain] = await Promise.all([api.query(workspaceId, request), api.explain(workspaceId, request)])
        const nextTopoEntityRows = hasInlineTopoEntityProperties(next)
          ? []
          : await loadTopoEntityRows(api, workspaceId, next, request.time_range).catch(() => [])
        setResult(next)
        setTopoEntityRows(nextTopoEntityRows)
        setExplain(nextExplain)
        setExplainOpen(false)
        setResultView('table')
      } else {
        const next = await api.explain(workspaceId, request)
        setExplain(next)
        setExplainOpen(true)
      }
    } catch (nextError) {
      setError(formatError(nextError))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="query-workbench">
      <header className="query-head">
        <ExamplePicker onPick={(item) => {
          setQuery(item.query)
          setResultView('table')
        }} />
        <div className="query-head-actions">
          <TimeRangeControl value={timeRange} onChange={setTimeRange} />
          <Button variant="secondary" onClick={() => void run('explain')} disabled={busy}>
            <SearchCode size={15} />
            {t('query.action.explain')}
          </Button>
          <Button className="query-execute-button" variant="primary" onClick={() => void run('execute')} disabled={busy}>
            <Play size={14} />
            {t('query.action.execute')}
          </Button>
        </div>
      </header>

      {error && <div className="query-error">{error}</div>}

      <div className="query-layout">
        <section className="query-compose">
          <MonacoBlock
            value={query}
            language="sql"
            height={splEditorHeight}
            maxAutoHeight={splMaxEditorHeight}
            minAutoHeight={splMinEditorHeight}
            wordWrap="on"
            onChange={setQuery}
            onContentHeightChange={setSplEditorHeight}
            onSubmit={() => void run('execute')}
          />
        </section>

        <section className="query-results">
          <div className="query-result-header">
            <div>
              <strong>{t('query.result.title')}</strong>
              <span>{result ? formatResultSummary(t, result.rows.length, resultColumns.length) : t('query.result.empty.title')}</span>
            </div>
            <SegmentedControl<ResultView>
              size="sm"
              value={resultView}
              onChange={setResultView}
              items={[
                { value: 'table', label: t('query.view.table'), icon: <Table2 size={13} /> },
                { value: 'chart', label: t('query.view.chart'), icon: <BarChart3 size={13} /> },
              ]}
            />
          </div>

          <div className="query-result-body">
            {resultView === 'table' ? (
              result ? <ResultTable result={result} /> : <QueryEmpty title={t('query.result.empty.title')} detail={t('query.result.empty.detail')} icon={<Rows3 size={22} />} />
            ) : (
              <QueryTopoChart data={topoData} canChart={canChart} />
            )}
          </div>
        </section>
      </div>

      {explainOpen && (
        <div className="query-explain-drawer" role="presentation" onClick={() => setExplainOpen(false)}>
          <section className="query-explain-panel" aria-label={t('query.explain.panelLabel')} onClick={(event) => event.stopPropagation()}>
            <div className="query-explain-title">
              <span><SearchCode size={13} /> {t('query.action.explain')}</span>
              <div className="query-explain-actions">
                {explain?.provider && <Badge tone="indigo">{explain.provider}</Badge>}
                <IconButton label={t('query.action.close')} size="sm" onClick={() => setExplainOpen(false)}>
                  <X size={14} />
                </IconButton>
              </div>
            </div>
            <MonacoBlock
              value={explain ? stringify(explain) : '{}'}
              language="json"
              height={260}
              readOnly
            />
          </section>
        </div>
      )}
    </div>
  )
}

function ExamplePicker({ onPick }: { onPick: (item: (typeof examples)[number]) => void }) {
  const { t } = useI18n()

  return (
    <div className="query-examples">
      <div className="query-example-title">{t('query.examples.title')}</div>
      <div className="query-example-grid">
        {examples.map((item) => (
          <button key={item.labelKey} type="button" onClick={() => onPick(item)}>
            <Wand2 size={13} />
            {t(item.labelKey)}
          </button>
        ))}
      </div>
    </div>
  )
}

function TimeRangeControl({
  value,
  onChange,
}: {
  value: { from: string; to: string }
  onChange: (value: { from: string; to: string }) => void
}) {
  const { t } = useI18n()

  return (
    <div className="query-timebar">
      <CalendarClock size={14} />
      <input
        aria-label={t('query.time.from')}
        type="datetime-local"
        value={value.from}
        onChange={(event) => onChange({ ...value, from: event.target.value })}
      />
      <span>{t('query.time.toSeparator')}</span>
      <input
        aria-label={t('query.time.to')}
        type="datetime-local"
        value={value.to}
        onChange={(event) => onChange({ ...value, to: event.target.value })}
      />
      <button type="button" onClick={() => onChange({ from: '', to: '' })}>{t('query.time.all')}</button>
    </div>
  )
}

function MonacoBlock({
  value,
  language,
  height,
  readOnly = false,
  lineNumbers = 'on',
  wordWrap = 'on',
  maxAutoHeight,
  minAutoHeight,
  onChange,
  onContentHeightChange,
  onSubmit,
}: {
  value: string
  language: string
  height: number
  readOnly?: boolean
  lineNumbers?: 'on' | 'off'
  wordWrap?: 'on' | 'off'
  maxAutoHeight?: number
  minAutoHeight?: number
  onChange?: (value: string) => void
  onContentHeightChange?: (height: number) => void
  onSubmit?: () => void
}) {
  const autoHeightRef = useRef({ min: minAutoHeight, max: maxAutoHeight, onChange: onContentHeightChange, onSubmit })
  autoHeightRef.current = { min: minAutoHeight, max: maxAutoHeight, onChange: onContentHeightChange, onSubmit }

  const updateAutoHeight = (editor: MonacoEditor.IStandaloneCodeEditor) => {
    const { min, max, onChange: emitHeight } = autoHeightRef.current
    if (!emitHeight) return
    const contentHeight = editor.getContentHeight()
    const nextHeight = Math.max(min ?? contentHeight, Math.min(max ?? contentHeight, contentHeight))
    emitHeight(Math.round(nextHeight))
  }

  return (
    <div className="query-monaco" style={{ height }}>
      <Editor
        value={value}
        language={language}
        theme="vs"
        onMount={(editor, monaco) => {
          updateAutoHeight(editor)
          editor.onDidContentSizeChange(() => updateAutoHeight(editor))
          if (!readOnly) {
            editor.addCommand(monaco.KeyCode.Enter, () => autoHeightRef.current.onSubmit?.())
            const insertLineBreak = () => editor.trigger('keyboard', 'type', { text: '\n' })
            editor.addCommand(monaco.KeyMod.Shift | monaco.KeyCode.Enter, insertLineBreak)
            editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, insertLineBreak)
            editor.addCommand(monaco.KeyMod.WinCtrl | monaco.KeyCode.Enter, insertLineBreak)
          }
        }}
        onChange={(nextValue) => {
          if (!readOnly) onChange?.(nextValue || '')
        }}
        options={{
          accessibilitySupport: 'off',
          automaticLayout: true,
          domReadOnly: readOnly,
          fontFamily: 'var(--om-mono)',
          fontSize: 12,
          lineHeight: 19,
          lineNumbers,
          lineNumbersMinChars: lineNumbers === 'off' ? 0 : 3,
          minimap: { enabled: false },
          padding: { top: 10, bottom: 10 },
          readOnly,
          renderLineHighlight: 'none',
          scrollBeyondLastLine: false,
          tabSize: 2,
          wordWrap,
        }}
      />
    </div>
  )
}

function QueryTopoChart({ data, canChart }: { data: EntityTopoData | null; canChart: boolean }) {
  const { t } = useI18n()
  const [selected, setSelected] = useState<TopoSelection | null>(null)
  const [searchText, setSearchText] = useState('')
  const [selectedTypes, setSelectedTypes] = useState<string[]>([])
  const [filterOpen, setFilterOpen] = useState(false)
  const [zoomLevel, setZoomLevel] = useState<TopoZoomLevel>('full')
  const settings = useMemo<EntityTopoDisplaySettings>(() => ({
    ...DEFAULT_ENTITY_TOPO_DISPLAY_SETTINGS,
    layoutAlgorithm: 'force',
    showLabels: true,
    showMiniMap: true,
  }), [])
  const filteredData = useMemo(() => {
    if (!data) return null
    return filterEntityTopoData(data, {
      domains: [],
      types: selectedTypes,
      relations: [],
      attributeFilters: [],
      focusIds: [],
      searchText,
      filterStacking: false,
    })
  }, [data, searchText, selectedTypes])

  useEffect(() => {
    setSelected(null)
  }, [data])

  useEffect(() => {
    if (!data) return
    const validTypes = new Set(data.clusters.map((cluster) => cluster.cluster))
    setSelectedTypes((current) => current.filter((item) => validTypes.has(item)))
  }, [data])

  useEffect(() => {
    if (!filteredData || !selected) return
    if (selected.kind === 'node' && !filteredData.nodes.some((node) => node.id === selected.node.id)) setSelected(null)
    if (selected.kind === 'edge' && !filteredData.edges.some((edge) => edge.id === selected.edge.id)) setSelected(null)
  }, [filteredData, selected])

  if (!data) {
    return <QueryEmpty title={t('query.chart.empty.title')} detail={t('query.chart.empty.detail')} icon={<Network size={22} />} />
  }

  if (!canChart) {
    return (
      <QueryEmpty
        title={t('query.chart.unavailable.title')}
        detail={t('query.chart.unavailable.detail')}
        icon={<Network size={22} />}
      />
    )
  }

  const visibleData = filteredData || data
  const activeFilterCount = selectedTypes.length + (searchText.trim() ? 1 : 0)

  return (
    <div className="query-chart-shell">
      <div className="query-chart-controls">
        <label className="eto-search-wrap query-chart-search">
          <Search size={14} />
          <input value={searchText} onChange={(event) => setSearchText(event.target.value)} placeholder={t('query.chart.search.placeholder')} />
          {searchText && (
            <button className="eto-icon-button subtle query-chart-clear-search" type="button" onClick={() => setSearchText('')} title={t('query.chart.search.clear')}>
              <X size={13} />
            </button>
          )}
        </label>
        <div className="query-chart-filter-wrap">
          <button
            className={activeFilterCount > 0 ? 'eto-filter-chip query-chart-filter-trigger active' : 'eto-filter-chip query-chart-filter-trigger'}
            onClick={() => setFilterOpen((value) => !value)}
            type="button"
          >
            <SlidersHorizontal size={14} />
            <span>{t('query.chart.filter')}</span>
            {selectedTypes.length > 0 && <strong>{selectedTypes.length}</strong>}
          </button>
          {filterOpen && (
            <div className="query-chart-filter-panel">
              <div className="query-chart-filter-title">
                <strong>{t('query.chart.type')}</strong>
                {selectedTypes.length > 0 && <button type="button" onClick={() => setSelectedTypes([])}>{t('query.action.clear')}</button>}
              </div>
              <div className="query-chart-type-list">
                {data.clusters.map((cluster) => (
                  <button
                    key={cluster.cluster}
                    className={selectedTypes.includes(cluster.cluster) ? 'eto-filter-row active' : 'eto-filter-row'}
                    onClick={() => setSelectedTypes((current) => toggleFilterValue(current, cluster.cluster))}
                    type="button"
                  >
                    <span className="eto-type-dot" style={{ background: cluster.visual.color }} />
                    <span className="eto-filter-label">
                      <b>{cluster.displayName}</b>
                      <small>{cluster.cluster}</small>
                    </span>
                    <span className="eto-filter-row-meta">
                      <span className="eto-filter-count">{cluster.count.toLocaleString()}</span>
                    </span>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {visibleData.nodes.length > 0 ? (
        <EntityTopoGraphView
          data={visibleData}
          enableFocusActions={false}
          showViewportToolbar={false}
          focusIds={[]}
          selected={selected}
          settings={settings}
          zoomLevel={zoomLevel}
          onSelect={setSelected}
          onFocusNode={() => undefined}
          onZoomLevelChange={setZoomLevel}
        />
      ) : (
        <QueryEmpty title={t('query.chart.noMatching.title')} detail={t('query.chart.noMatching.detail')} icon={<Network size={22} />} />
      )}
      <QueryTopoDetailPanel selection={selected} data={visibleData} onClose={() => setSelected(null)} />
    </div>
  )
}

function QueryTopoDetailPanel({
  selection,
  data,
  onClose,
}: {
  selection: TopoSelection | null
  data: EntityTopoData
  onClose: () => void
}) {
  const { t } = useI18n()

  if (!selection) return null
  const nodeById = new Map(data.nodes.map((node) => [node.id, node]))

  if (selection.kind === 'edge') {
    const edge = selection.edge
    return (
      <aside className="eto-detail-panel open query-chart-detail-panel">
        <QueryDetailHeader title={edge.relationType} subtitle={t('query.detail.relation')} icon={<ArrowRight size={16} />} onClose={onClose} />
        <div className="eto-detail-body">
          <QueryEdgeRoute edge={edge} source={nodeById.get(edge.source)} target={nodeById.get(edge.target)} />
          <QueryDetailTable rows={queryDetailRows(edge.row)} />
        </div>
      </aside>
    )
  }

  const node = selection.node
  return (
    <aside className="eto-detail-panel open query-chart-detail-panel">
      <QueryDetailHeader
        title={node.title}
        subtitle={node.endpoint.entityType}
        icon={<Database size={16} />}
        color={node.visual.color}
        onClose={onClose}
      />
      <div className="eto-detail-body">
        <div className="eto-detail-summary">
          <span><strong>{node.inDegree}</strong> {t('query.detail.inbound')}</span>
          <span><strong>{node.outDegree}</strong> {t('query.detail.outbound')}</span>
          <span><strong>{node.relationCount}</strong> {t('query.detail.total')}</span>
        </div>
        <QueryDetailTable
          rows={[
            ...(node.titleSource ? [['label_source', node.titleSource] as [string, unknown]] : []),
            ...queryDetailRows(node.properties),
            ['domain', node.endpoint.domain],
            ['entity_type', node.endpoint.entityType],
            ['entity_id', node.endpoint.entityId],
            ['cluster', node.endpoint.cluster],
          ]}
        />
      </div>
    </aside>
  )
}

function QueryDetailHeader({
  title,
  subtitle,
  icon,
  color = '#64748b',
  onClose,
}: {
  title: string
  subtitle: string
  icon: ReactNode
  color?: string
  onClose: () => void
}) {
  const { t } = useI18n()

  return (
    <header className="eto-detail-header">
      <div className="eto-detail-icon" style={{ color, background: `${color}14`, borderColor: `${color}33` }}>
        {icon}
      </div>
      <div className="eto-detail-title">
        <strong>{title}</strong>
        <code>{subtitle}</code>
      </div>
      <button className="eto-icon-button subtle" onClick={onClose} type="button" title={t('query.action.close')}>
        <X size={15} />
      </button>
    </header>
  )
}

function QueryEdgeRoute({
  edge,
  source,
  target,
}: {
  edge: EntityTopoEdge
  source?: EntityTopoNode
  target?: EntityTopoNode
}) {
  const { t } = useI18n()

  if (!source || !target) return null
  return (
    <div className="eto-edge-route">
      <div className="query-route-node">
        <span className="eto-route-icon" style={{ background: source.visual.bg, color: source.visual.text }}>
          <Database size={14} />
        </span>
        <span>
          <b>{source.title}</b>
          <small>{t('query.detail.source')} · {endpointLabel(source.endpoint)}</small>
        </span>
      </div>
      <div className="eto-route-line"><span>{edge.relationType}</span></div>
      <div className="query-route-node">
        <span className="eto-route-icon" style={{ background: target.visual.bg, color: target.visual.text }}>
          <Database size={14} />
        </span>
        <span>
          <b>{target.title}</b>
          <small>{t('query.detail.target')} · {endpointLabel(target.endpoint)}</small>
        </span>
      </div>
    </div>
  )
}

function QueryDetailTable({ rows }: { rows: Array<[string, unknown]> }) {
  const { t } = useI18n()

  if (rows.length === 0) return <div className="eto-detail-empty">{t('query.detail.noProperties')}</div>
  return (
    <table className="eto-detail-table">
      <tbody>
        {rows.map(([key, value], index) => (
          <tr key={`${key}-${index}`}>
            <td>{key}</td>
            <td>{formatTopoValue(value)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function queryDetailRows(row: Record<string, unknown>) {
  return Object.entries(row)
    .filter(([, value]) => value !== undefined && value !== null && formatTopoValue(value).trim() !== '')
    .slice(0, 28)
}

function QueryEmpty({ title, detail, icon }: { title: string; detail: string; icon: ReactNode }) {
  return (
    <div className="query-empty">
      <div>{icon}</div>
      <strong>{title}</strong>
      <span>{detail}</span>
    </div>
  )
}

export function ResultTable({ result }: { result: QueryResult }) {
  const { t } = useI18n()
  const columns = result.columns.length > 0 ? result.columns : result.rows[0] ? Object.keys(result.rows[0]) : []
  const [pageSize, setPageSize] = useState(50)
  const [page, setPage] = useState(1)
  const pageCount = Math.max(1, Math.ceil(result.rows.length / pageSize))
  const safePage = Math.min(page, pageCount)
  const pageNumbers = useMemo(() => paginationItems(safePage, pageCount), [pageCount, safePage])
  const pageRows = useMemo(
    () => result.rows.slice((safePage - 1) * pageSize, safePage * pageSize),
    [pageSize, result.rows, safePage],
  )

  useEffect(() => {
    setPage(1)
  }, [result])

  useEffect(() => {
    if (page !== safePage) setPage(safePage)
  }, [page, safePage])

  if (result.rows.length === 0) return <EmptyState title={t('query.table.noRows.title')} detail={t('query.table.noRows.detail')} />

  return (
    <div className="query-table-region">
      <div className="query-table-wrap">
        <table className="om-table query-table">
          <thead>
            <tr>
              {columns.map((column) => (
                <th key={column}>{column}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {pageRows.map((row, index) => (
              <tr key={`${safePage}-${index}`}>
                {columns.map((column) => {
                  const cellValue = formatCell(row[column])
                  return (
                    <td key={column} className={typeof row[column] === 'object' ? 'mono small' : undefined}>
                      <span className="query-cell-value">{cellValue}</span>
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <footer className="query-table-footer">
        <div className="query-table-pages">
          <button disabled={safePage <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))} type="button" title={t('query.table.previousPage')}>
            <ChevronLeft size={14} />
          </button>
          {pageNumbers.map((item, index) => item === '...'
            ? <span className="query-table-page-gap" key={`gap-${index}`}>...</span>
            : (
              <button key={item} className={item === safePage ? 'active' : ''} onClick={() => setPage(item)} type="button">
                {item}
              </button>
            ))}
          <button disabled={safePage >= pageCount} onClick={() => setPage((value) => Math.min(pageCount, value + 1))} type="button" title={t('query.table.nextPage')}>
            <ChevronRight size={14} />
          </button>
        </div>
        <select value={pageSize} onChange={(event) => setPageSize(Number(event.target.value))}>
          {resultPageSizes.map((size) => <option key={size} value={size}>{t('query.table.pageSize', { size })}</option>)}
        </select>
        <span>{t.rich('query.table.totalRows', { strong: (chunks) => <strong>{chunks}</strong> }, { count: result.rows.length.toLocaleString() })}</span>
      </footer>
    </div>
  )
}

function formatResultSummary(t: TFunction, rows: number, columns: number) {
  return `${formatCountUnit(t, rows, 'query.unit.row', 'query.unit.rows')}, ${formatCountUnit(t, columns, 'query.unit.column', 'query.unit.columns')}`
}

function formatCountUnit(t: TFunction, count: number, oneKey: MessageKey, otherKey: MessageKey) {
  const key = count === 1 ? oneKey : otherKey
  return `${count.toLocaleString()} ${t(key)}`
}

function formatCell(value: unknown): string {
  if (value === null || value === undefined) return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function toIsoOrUndefined(value: string) {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function hasInlineTopoEntityProperties(result: QueryResult) {
  return result.rows.some((row) => isEntityPropertyRecord(row.src) && isEntityPropertyRecord(row.dest))
}

function isEntityPropertyRecord(value: unknown) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  const record = value as Record<string, unknown>
  return Boolean(record.__domain__ && record.__entity_type__ && record.__entity_id__)
}

async function loadTopoEntityRows(
  api: UModelApi,
  workspaceId: string,
  result: QueryResult,
  timeRange?: QueryRequest['time_range'],
) {
  const topo = buildEntityTopoData(result, [], [])
  if (topo.nodes.length === 0 || topo.edges.length === 0) return []

  const idsByType = new Map<string, { domain: string; entityType: string; ids: string[] }>()
  topo.nodes.slice(0, 120).forEach((node) => {
    const key = `${node.endpoint.domain}\n${node.endpoint.entityType}`
    const group = idsByType.get(key) || { domain: node.endpoint.domain, entityType: node.endpoint.entityType, ids: [] }
    if (!group.ids.includes(node.endpoint.entityId)) group.ids.push(node.endpoint.entityId)
    idsByType.set(key, group)
  })

  const results = await Promise.all([...idsByType.values()].map(async (group) => {
    const ids = group.ids.map((id) => `'${escapeSPL(id)}'`).join(',')
    const limit = Math.max(20, group.ids.length)
    const query = `.entity with(domain='${escapeSPL(group.domain)}', name='${escapeSPL(group.entityType)}', ids=[${ids}], topk=${limit}) | limit ${limit}`
    const rows = await api.query(workspaceId, { query, limit, time_range: timeRange })
    return rows.rows
  }))
  return results.flat()
}

function escapeSPL(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/'/g, "\\'")
}

function paginationItems(page: number, totalPages: number): Array<number | '...'> {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, index) => index + 1)
  const items: Array<number | '...'> = [1]
  if (page > 3) items.push('...')
  for (let next = Math.max(2, page - 1); next <= Math.min(totalPages - 1, page + 1); next += 1) items.push(next)
  if (page < totalPages - 2) items.push('...')
  items.push(totalPages)
  return items
}
