import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import Editor from '@monaco-editor/react'
import { ArrowRight, BarChart3, CalendarClock, Check, ChevronDown, ChevronLeft, ChevronRight, Database, Eye, Network, Play, Rows3, Search, SearchCode, SlidersHorizontal, Table2, Wand2, X } from 'lucide-react'
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
type CellPreview = {
  title: string
  subtitle: string
  value: string
  language: string
}
type CellPresentation = {
  display: string
  full: string
  language: string
  multiline: boolean
  complex: boolean
  previewable: boolean
  variant: 'plain' | 'code' | 'long-text'
  showFullHover: boolean
}
type ColumnProfile = {
  key: string
  minWidth: number
  weight: number
  variant: 'plain' | 'code' | 'long-text'
}
type TableLayout = {
  columnWidths: number[]
  mode: 'fill' | 'overflow'
  tableWidth: number
}

const resultPageSizes = [25, 50, 100]
const splMinEditorHeight = 41
const splMaxEditorHeight = 117
const largeTextCodeblockLength = 1000
const wrappedTextLineClamp = 4

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
  {
    labelKey: 'query.examples.entitySetLogs',
    query:
      ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_logs('devops', 'devops.log.service', query='level = \"ERROR\"')",
  },
  {
    labelKey: 'query.examples.entitySetMetrics',
    query:
      ".entity_set with(domain='devops', name='devops.service', ids=['10000000000000000000000000000101']) | entity-call get_metrics('devops', 'devops.metric.service', 'request_count', step='30s')",
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

  const tableResult = useMemo(() => result ? normalizeResultForTable(result) : null, [result])
  const resultColumns = tableResult?.columns.length ? tableResult.columns : tableResult?.rows[0] ? Object.keys(tableResult.rows[0]) : []
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
        <ExamplePicker value={query} onPick={(item) => {
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
              <span>{tableResult ? formatResultSummary(t, tableResult.rows.length, resultColumns.length) : t('query.result.empty.title')}</span>
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
              tableResult ? <ResultTable result={tableResult} /> : <QueryEmpty title={t('query.result.empty.title')} detail={t('query.result.empty.detail')} icon={<Rows3 size={22} />} />
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

function ExamplePicker({
  value,
  onPick,
}: {
  value: string
  onPick: (item: (typeof examples)[number]) => void
}) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const pickerRef = useRef<HTMLDivElement | null>(null)
  const selected = examples.find((item) => item.query === value)

  useEffect(() => {
    if (!open) return
    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!pickerRef.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', closeOnOutsidePointer)
    return () => document.removeEventListener('pointerdown', closeOnOutsidePointer)
  }, [open])

  return (
    <div className="query-examples" ref={pickerRef}>
      <div className="query-example-title">{t('query.examples.title')}</div>
      <div className="query-example-control">
        <button
          className="query-example-trigger"
          type="button"
          aria-expanded={open}
          onClick={() => setOpen((value) => !value)}
        >
          <Wand2 size={14} />
          <span>{selected ? t(selected.labelKey) : t('query.examples.placeholder')}</span>
          <ChevronDown className="query-example-chevron" size={14} />
        </button>
        {open && (
          <div className="query-example-menu" role="menu">
            {examples.map((item) => {
              const active = selected?.labelKey === item.labelKey
              return (
                <button
                  key={item.labelKey}
                  className={active ? 'active' : ''}
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    onPick(item)
                    setOpen(false)
                  }}
                >
                  <span>{t(item.labelKey)}</span>
                  {active && <Check size={14} />}
                </button>
              )
            })}
          </div>
        )}
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
  height: number | string
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
          scrollbar: {
            alwaysConsumeMouseWheel: false,
            horizontal: 'auto',
            horizontalScrollbarSize: 8,
            useShadows: false,
            vertical: 'auto',
            verticalScrollbarSize: 8,
          },
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
  const [tableWrapRef, tableWrapWidth] = useElementWidth<HTMLDivElement>()
  const [pageSize, setPageSize] = useState(50)
  const [page, setPage] = useState(1)
  const [preview, setPreview] = useState<CellPreview | null>(null)
  const pageCount = Math.max(1, Math.ceil(result.rows.length / pageSize))
  const safePage = Math.min(page, pageCount)
  const pageNumbers = useMemo(() => paginationItems(safePage, pageCount), [pageCount, safePage])
  const pageRows = useMemo(
    () => result.rows.slice((safePage - 1) * pageSize, safePage * pageSize),
    [pageSize, result.rows, safePage],
  )
  const columnProfiles = useMemo(() => getColumnProfiles(columns, pageRows), [columns, pageRows])
  const tableLayout = useMemo(() => getTableLayout(columnProfiles, tableWrapWidth), [columnProfiles, tableWrapWidth])

  useEffect(() => {
    setPage(1)
    setPreview(null)
  }, [result])

  useEffect(() => {
    if (page !== safePage) setPage(safePage)
  }, [page, safePage])

  if (result.rows.length === 0) return <EmptyState title={t('query.table.noRows.title')} detail={t('query.table.noRows.detail')} />

  return (
    <div className={preview ? 'query-table-region with-preview' : 'query-table-region'}>
      <div className="query-table-main">
        <div className="query-table-wrap" ref={tableWrapRef}>
          <table
            className={`om-table query-table ${tableLayout.mode === 'overflow' ? 'overflowing' : 'filling'}`}
            style={{ minWidth: tableLayout.tableWidth, width: tableLayout.tableWidth }}
          >
            <colgroup>
              {columnProfiles.map((profile, index) => (
                <col
                  key={profile.key}
                  className={`query-col-${profile.variant}`}
                  style={{ width: tableLayout.columnWidths[index] }}
                />
              ))}
            </colgroup>
            <thead>
              <tr>
                {columns.map((column) => (
                  <th key={column}>{column}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {pageRows.map((row, index) => {
                const rowPresentations = columns.map((column, columnIndex) => getCellPresentation(row[column], tableLayout.columnWidths[columnIndex]))
                const rowHasPreviewable = rowPresentations.some((presentation) => presentation.previewable)

                return (
                  <tr key={`${safePage}-${index}`} className={rowHasPreviewable ? 'query-row-has-code' : undefined}>
                    {columns.map((column, columnIndex) => {
                      const presentation = rowPresentations[columnIndex]
                      const wrapLongText = rowHasPreviewable && presentation.variant === 'long-text'
                      const showFullHover = presentation.showFullHover && (
                        !wrapLongText || !canWrappedTextShowFull(presentation.full, tableLayout.columnWidths[columnIndex])
                      )
                      const rowNumber = (safePage - 1) * pageSize + index + 1
                      const cellClassName = [
                        presentation.complex ? 'mono small complex' : '',
                        presentation.previewable ? 'query-cell-td-code' : '',
                        presentation.variant === 'long-text' ? 'query-cell-td-long-text' : '',
                      ].filter(Boolean).join(' ')

                      return (
                        <td key={column} className={cellClassName || undefined}>
                          <div className={presentation.previewable ? 'query-cell-content previewable' : 'query-cell-content'}>
                            {presentation.previewable ? (
                              <span className="query-cell-code-shell">
                                <span className="query-cell-actionbar">
                                  <button
                                    type="button"
                                    aria-label={t('query.table.viewCell')}
                                    onClick={() => setPreview({
                                      title: column,
                                      subtitle: t('query.preview.row', { row: rowNumber.toLocaleString() }),
                                      value: presentation.full,
                                      language: presentation.language,
                                    })}
                                  >
                                    <Eye size={13} />
                                    <span>{t('query.table.viewCell')}</span>
                                  </button>
                                </span>
                                <span className="query-cell-value multiline complex code">
                                  {presentation.display}
                                </span>
                              </span>
                            ) : (
                              <>
                                <span
                                  className={[
                                    'query-cell-value',
                                    presentation.multiline ? 'multiline' : '',
                                    presentation.complex ? 'complex' : '',
                                    wrapLongText ? 'wrapped' : '',
                                    presentation.variant,
                                  ].filter(Boolean).join(' ')}
                                  tabIndex={showFullHover ? 0 : undefined}
                                >
                                  {wrapLongText ? presentation.full : presentation.display}
                                </span>
                                {showFullHover && (
                                  <span className="query-cell-full-popover" role="tooltip">
                                    {presentation.full}
                                  </span>
                                )}
                              </>
                            )}
                          </div>
                        </td>
                      )
                    })}
                  </tr>
                )
              })}
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
      {preview && (
        <aside className="query-preview-panel">
          <div className="query-preview-header">
            <div>
              <strong>{preview.title}</strong>
              <span>{preview.subtitle}</span>
            </div>
            <button type="button" onClick={() => setPreview(null)} title={t('query.preview.close')} aria-label={t('query.preview.close')}>
              <X size={15} />
            </button>
          </div>
          <div className="query-preview-body">
            <MonacoBlock
              value={preview.value}
              language={preview.language}
              height="100%"
              readOnly
              wordWrap="on"
            />
          </div>
        </aside>
      )}
    </div>
  )
}

function getCellPresentation(value: unknown, columnWidth?: number): CellPresentation {
  if (value === null || value === undefined) {
    return { display: '-', full: '-', language: 'plaintext', multiline: false, complex: false, previewable: false, variant: 'plain', showFullHover: false }
  }

  const normalized = normalizeStructuredValue(value)
  if (normalized.structured) {
    const full = JSON.stringify(normalized.value, null, 2)
    const compact = JSON.stringify(normalized.value)
    if (compact.length <= 36) {
      return {
        display: compact,
        full: compact,
        language: 'json',
        multiline: false,
        complex: false,
        previewable: false,
        variant: 'plain',
        showFullHover: false,
      }
    }
    return {
      display: full,
      full,
      language: 'json',
      multiline: true,
      complex: true,
      previewable: true,
      variant: 'code',
      showFullHover: false,
    }
  }

  const text = String(value)
  if (text.length > largeTextCodeblockLength) {
    return {
      display: text,
      full: text,
      language: 'plaintext',
      multiline: text.includes('\n'),
      complex: true,
      previewable: true,
      variant: 'code',
      showFullHover: false,
    }
  }

  const long = text.length > 36 || text.includes('\n') || isTextVisuallyOverflowed(text, columnWidth)
  return {
    display: long ? middleEllipsis(text, inlineTextLimit(columnWidth)) : text,
    full: text,
    language: 'plaintext',
    multiline: text.includes('\n'),
    complex: long,
    previewable: false,
    variant: long ? 'long-text' : 'plain',
    showFullHover: long,
  }
}

function canWrappedTextShowFull(text: string, columnWidth?: number) {
  if (!columnWidth) return false
  return estimateWrappedLineCount(text, inlineTextContentWidth(columnWidth)) <= wrappedTextLineClamp
}

function estimateWrappedLineCount(text: string, lineWidth: number) {
  if (lineWidth <= 0) return Number.POSITIVE_INFINITY
  const hardLines = text.split('\n')
  return hardLines.reduce((lineCount, line) => (
    lineCount + Math.max(1, Math.ceil(estimateInlineTextWidth(line) / lineWidth))
  ), 0)
}

function isTextVisuallyOverflowed(text: string, columnWidth?: number) {
  if (!columnWidth || text.includes('\n')) return false
  return estimateInlineTextWidth(text) > inlineTextContentWidth(columnWidth)
}

function inlineTextLimit(columnWidth?: number) {
  if (!columnWidth) return 36
  return clamp(Math.floor(inlineTextContentWidth(columnWidth) / 7.2), 12, 36)
}

function inlineTextContentWidth(columnWidth: number) {
  return Math.max(40, columnWidth - 34)
}

function estimateInlineTextWidth(text: string) {
  return Array.from(text).reduce((width, char) => width + estimateCharacterWidth(char), 0)
}

function estimateCharacterWidth(char: string) {
  if (/[\u2e80-\u9fff\uac00-\ud7af\uff00-\uffef]/.test(char)) return 13
  if (/\s/.test(char)) return 4.2
  if (/[A-Z]/.test(char)) return 7.8
  return 7.2
}

function useElementWidth<T extends HTMLElement>() {
  const ref = useRef<T | null>(null)
  const [width, setWidth] = useState(0)

  useEffect(() => {
    const element = ref.current
    if (!element) return undefined

    const updateWidth = () => setWidth(element.clientWidth)
    updateWidth()

    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', updateWidth)
      return () => window.removeEventListener('resize', updateWidth)
    }

    const observer = new ResizeObserver(updateWidth)
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  return [ref, width] as const
}

function getColumnProfiles(columns: string[], rows: Array<Record<string, unknown>>): ColumnProfile[] {
  return columns.map((column) => {
    let variant: ColumnProfile['variant'] = 'plain'
    let maxTextLength = column.length

    for (const row of rows) {
      const presentation = getCellPresentation(row[column])
      if (presentation.previewable) {
        variant = 'code'
      } else if (variant !== 'code' && presentation.variant === 'long-text') {
        variant = 'long-text'
      }
      maxTextLength = Math.max(maxTextLength, Math.min(presentation.display.length, 30))
    }

    if (variant === 'code') return { key: column, minWidth: 320, weight: 2.2, variant }
    if (variant === 'long-text') return { key: column, minWidth: 220, weight: 1.25, variant }

    return {
      key: column,
      minWidth: clamp(Math.round(maxTextLength * 7.2 + 54), 128, 260),
      weight: 1,
      variant,
    }
  })
}

function getTableLayout(columns: ColumnProfile[], availableWidth: number): TableLayout {
  if (columns.length === 0) return { columnWidths: [], mode: 'fill', tableWidth: Math.max(availableWidth, 0) }

  const minWidths = columns.map((column) => column.minWidth)
  const totalMinWidth = minWidths.reduce((sum, width) => sum + width, 0)
  const safeAvailableWidth = Math.max(availableWidth, totalMinWidth)

  if (availableWidth <= 0 || totalMinWidth >= availableWidth) {
    return {
      columnWidths: minWidths,
      mode: totalMinWidth > availableWidth ? 'overflow' : 'fill',
      tableWidth: totalMinWidth,
    }
  }

  const remainingWidth = safeAvailableWidth - totalMinWidth
  const totalWeight = columns.reduce((sum, column) => sum + column.weight, 0)
  const columnWidths = minWidths.map((width, index) => (
    Math.floor(width + (remainingWidth * columns[index].weight) / totalWeight)
  ))
  const roundingGap = safeAvailableWidth - columnWidths.reduce((sum, width) => sum + width, 0)
  if (roundingGap > 0) columnWidths[columnWidths.length - 1] += roundingGap

  return {
    columnWidths,
    mode: 'fill',
    tableWidth: safeAvailableWidth,
  }
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function middleEllipsis(value: string, maxLength = 36): string {
  if (value.length <= maxLength) return value
  const head = Math.ceil((maxLength - 1) * 0.6)
  const tail = Math.floor((maxLength - 1) * 0.4)
  return `${value.slice(0, head)}...${value.slice(value.length - tail)}`
}

function normalizeStructuredValue(value: unknown): { structured: boolean; value: unknown } {
  if (typeof value === 'object') return { structured: true, value }
  if (typeof value !== 'string') return { structured: false, value }

  const trimmed = value.trim()
  if (!looksLikeJsonValue(trimmed)) return { structured: false, value }
  const parsed = parseJsonLike(trimmed)
  return parsed !== trimmed && typeof parsed === 'object' && parsed !== null
    ? { structured: true, value: parsed }
    : { structured: false, value }
}

function formatResultSummary(t: TFunction, rows: number, columns: number) {
  return `${formatCountUnit(t, rows, 'query.unit.row', 'query.unit.rows')}, ${formatCountUnit(t, columns, 'query.unit.column', 'query.unit.columns')}`
}

function formatCountUnit(t: TFunction, count: number, oneKey: MessageKey, otherKey: MessageKey) {
  const key = count === 1 ? oneKey : otherKey
  return `${count.toLocaleString()} ${t(key)}`
}

function normalizeResultForTable(result: QueryResult): QueryResult {
  const expanded = expandHeaderDataResult(result)
  return expanded || result
}

function expandHeaderDataResult(result: QueryResult): QueryResult | null {
  const nestedRows = result.rows.map((row) => extractHeaderDataRows(row))
  if (nestedRows.length === 0 || nestedRows.some((item) => !item)) return null

  const columns = uniqueStrings(nestedRows.flatMap((item) => item?.columns || []))
  if (columns.length === 0) return null

  return {
    ...result,
    columns,
    rows: nestedRows.flatMap((item) => item?.rows || []),
  }
}

function extractHeaderDataRows(row: Record<string, unknown>): { columns: string[]; rows: Array<Record<string, unknown>> } | null {
  if (!('header' in row) || !('data' in row)) return null

  const columns = readStringArray(row.header)
  const rawData = parseJsonLike(row.data)
  if (!columns || !Array.isArray(rawData)) return null

  const rows = rawData.map((item) => dataItemToRow(columns, item)).filter((item): item is Record<string, unknown> => Boolean(item))
  return { columns, rows }
}

function dataItemToRow(columns: string[], item: unknown): Record<string, unknown> | null {
  const rawItem = parseJsonLike(item)
  if (Array.isArray(rawItem)) return valuesToRow(columns, rawItem)
  if (!rawItem || typeof rawItem !== 'object') return null

  const record = rawItem as Record<string, unknown>
  if (Array.isArray(record.values)) return valuesToRow(columns, record.values)
  return record
}

function valuesToRow(columns: string[], values: unknown[]): Record<string, unknown> {
  return Object.fromEntries(columns.map((column, index) => [column, parseCellValue(values[index])]))
}

function parseCellValue(value: unknown): unknown {
  if (typeof value !== 'string') return value
  const trimmed = value.trim()
  if (!trimmed) return value
  if (!looksLikeJsonValue(trimmed)) return value
  return parseJsonLike(trimmed)
}

function parseJsonLike(value: unknown): unknown {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch {
    return value
  }
}

function readStringArray(value: unknown): string[] | null {
  const parsed = parseJsonLike(value)
  if (!Array.isArray(parsed) || parsed.some((item) => typeof item !== 'string')) return null
  return parsed
}

function looksLikeJsonValue(value: string): boolean {
  return (
    value === 'null' ||
    value === 'true' ||
    value === 'false' ||
    value.startsWith('{') ||
    value.startsWith('[') ||
    (value.startsWith('"') && value.endsWith('"'))
  )
}

function uniqueStrings(values: string[]) {
  const seen = new Set<string>()
  return values.filter((value) => {
    if (seen.has(value)) return false
    seen.add(value)
    return true
  })
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
