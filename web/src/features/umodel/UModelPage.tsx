import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import ReactDOM from 'react-dom'
import Editor, { DiffEditor, useMonaco } from '@monaco-editor/react'
import * as YAML from 'js-yaml'
import {
  Box,
  Cable,
  Crosshair,
  Database,
  FileUp,
  GitBranch,
  Layers,
  Plus,
  RefreshCcw,
  Redo2,
  Save,
  Search,
  Settings2,
  Table2,
  Trash2,
  Undo2,
  X,
} from 'lucide-react'
import {
  ReactFlowProvider,
  applyNodeChanges,
  type Node,
  type NodeChange,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { BatchItemResult, QueryResult, UModelElement, WriteResult } from '../../api/types'
import { UModelApi } from '../../api/client'
import { Button, EmptyState, IconButton, SegmentedControl } from '../../design/components'
import { useI18n, type TFunction } from '../../i18n'
import { asArray, formatError, parseJson, stringify } from '../../lib/json'
import { buildGraph, layoutGraphWithGraphviz, type UModelNodeData, type GraphModel } from './graphModel'
import { SearchPanel } from './UModelSearchPanel'
import { FilterBar, SettingsSidebar, SummarySidebar } from './UModelSidebar'
import { GraphView } from './UModelGraphView'
import {
  aliasForElements,
  asUnknownArray,
  buildSearchIndex,
  cloneElementForDraft,
  cloneElements,
  cloneLinkForDraft,
  colorForKind,
  countEntries,
  createDataLink,
  defaultNewNode,
  descriptionForElement,
  detailShort,
  diffElements,
  elementKey,
  endpointId,
  filterElements,
  focusIdsForElements,
  isEntitySetLinkElement,
  isLinkElement,
  isObject,
  kindRank,
  labelForKind,
  linkTouchesElement,
  nodeKindOptions,
  optionalString,
  paginationItems,
  summarize,
  tableSortValue,
  titleForElement,
  toggleValue,
  upsertById,
  type BackgroundStyle,
  type DraftDiff,
  type DraftStatus,
  type EntitySetLinkDisplay,
  type ViewMode,
  type ZoomLevel,
} from './model'
import './umodel.css'

type SidebarTab = 'summary' | 'settings'

interface SubmitFailure {
  summary: string
  details: string[]
}

export function UModelPage({
  api,
  workspaceId,
  refreshToken,
}: {
  api: UModelApi
  workspaceId: string
  refreshToken: number
}) {
  const { t } = useI18n()
  const [serverElements, setServerElements] = useState<UModelElement[]>([])
  const [draftElements, setDraftElements] = useState<UModelElement[]>([])
  const [loading, setLoading] = useState(false)
  const [committing, setCommitting] = useState(false)
  const [layouting, setLayouting] = useState(false)
  const [error, setError] = useState('')
  const [, setMessage] = useState('')
  const [submitFailure, setSubmitFailure] = useState<SubmitFailure | null>(null)
  const [queryResult, setQueryResult] = useState<QueryResult | null>(null)
  const [mode, setMode] = useState<ViewMode>('graph')
  const [sidebarTab, setSidebarTab] = useState<SidebarTab>('summary')
  const [searchText, setSearchText] = useState('')
  const [searchPanelOpen, setSearchPanelOpen] = useState(false)
  const [fullTextFilters, setFullTextFilters] = useState<string[]>([])
  const [kindFilters, setKindFilters] = useState<string[]>([])
  const [domainFilters, setDomainFilters] = useState<string[]>([])
  const [focusIds, setFocusIds] = useState<string[]>([])
  const [filterStacking, setFilterStacking] = useState(false)
  const [backgroundStyle, setBackgroundStyle] = useState<BackgroundStyle>('dots')
  const [entitySetLinkDisplay, setEntitySetLinkDisplay] = useState<EntitySetLinkDisplay>('absolute_node')
  const [forceFullMode, setForceFullMode] = useState(false)
  const [selected, setSelected] = useState<UModelElement | null>(null)
  const [detailPanelWidth, setDetailPanelWidth] = useState(380)
  const [zoomLevel, setZoomLevel] = useState<ZoomLevel>('full')
  const [graph, setGraph] = useState<GraphModel>({ nodes: [], edges: [] })
  const [createOpen, setCreateOpen] = useState(false)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [addMenuOpen, setAddMenuOpen] = useState(false)
  const [connectOpen, setConnectOpen] = useState(false)
  const [connectSource, setConnectSource] = useState<UModelElement | null>(null)
  const [diffOpen, setDiffOpen] = useState(false)
  const [undoStack, setUndoStack] = useState<UModelElement[][]>([])
  const [redoStack, setRedoStack] = useState<UModelElement[][]>([])
  const searchBlurRef = useRef<number | null>(null)
  const searchWrapRef = useRef<HTMLDivElement | null>(null)
  const [searchPanelStyle, setSearchPanelStyle] = useState<CSSProperties>()

  const updateSearchPanelGeometry = useCallback(() => {
    const wrap = searchWrapRef.current
    const main = wrap?.closest('.ume-main') as HTMLElement | null
    if (!wrap || !main) return
    const wrapRect = wrap.getBoundingClientRect()
    const mainRect = main.getBoundingClientRect()
    const contentRect = (main.querySelector('.ume-content-main') as HTMLElement | null)?.getBoundingClientRect()
    const viewportPadding = 12
    const boundaryLeft = Math.max((contentRect?.left ?? mainRect.left) + viewportPadding, viewportPadding)
    const boundaryRight = Math.min(mainRect.right - viewportPadding, window.innerWidth - viewportPadding)
    const availableWidth = Math.max(0, boundaryRight - boundaryLeft)
    const width = availableWidth < 220 ? availableWidth : Math.min(680, availableWidth)
    const left = availableWidth <= width
      ? boundaryLeft
      : Math.min(Math.max(boundaryLeft, wrapRect.right - width), boundaryRight - width)
    setSearchPanelStyle({
      left,
      top: wrapRect.bottom + 8,
      width,
      maxWidth: width,
    })
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const result = await api.listUModel(workspaceId, 100)
      const elements = result.rows.map(rowToElement).filter((element) => elementKey(element))
      setQueryResult(result)
      setServerElements(elements)
      setDraftElements(cloneElements(elements))
      setUndoStack([])
      setRedoStack([])
      setSelected((current) => {
        if (!current) return null
        return elements.find((element) => elementKey(element) === elementKey(current)) || null
      })
      setMessage('')
    } catch (nextError) {
      setError(formatError(nextError))
    } finally {
      setLoading(false)
    }
  }, [api, workspaceId])

  useEffect(() => {
    void load()
  }, [load, refreshToken])

  useLayoutEffect(() => {
    if (!searchPanelOpen) return
    updateSearchPanelGeometry()
    const update = () => updateSearchPanelGeometry()
    const resizeObserver = typeof ResizeObserver === 'undefined'
      ? null
      : new ResizeObserver(update)
    if (resizeObserver && searchWrapRef.current) {
      resizeObserver.observe(searchWrapRef.current)
      const main = searchWrapRef.current.closest('.ume-main') as HTMLElement | null
      if (main) resizeObserver.observe(main)
    }
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => {
      resizeObserver?.disconnect()
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
    }
  }, [searchPanelOpen, updateSearchPanelGeometry])

  const stats = useMemo(() => summarize(draftElements), [draftElements])
  const searchIndex = useMemo(() => buildSearchIndex(draftElements), [draftElements])
  const resultLimit = queryResult?.page.limit
  const resultLimitReached = Boolean(resultLimit && serverElements.length >= resultLimit)
  const filtered = useMemo(
    () => filterElements(draftElements, fullTextFilters, kindFilters, domainFilters, focusIds, filterStacking, mode, entitySetLinkDisplay),
    [domainFilters, draftElements, entitySetLinkDisplay, filterStacking, focusIds, fullTextFilters, kindFilters, mode],
  )
  const filteredStats = useMemo(() => summarize(filtered), [filtered])

  const focusElement = useCallback((element: UModelElement) => {
    setMode('graph')
    if (isLinkElement(element)) {
      const alias = aliasForElements(draftElements.filter((item) => !isLinkElement(item)))
      const source = alias.get(endpointId((element.spec || {}).src)) || endpointId((element.spec || {}).src)
      const target = alias.get(endpointId((element.spec || {}).dest)) || endpointId((element.spec || {}).dest)
      const ids = [source, target].filter(Boolean)
      setFocusIds(ids.length > 0 ? ids : [elementKey(element)])
    } else {
      setFocusIds([elementKey(element)])
    }
    setSelected(element)
  }, [draftElements])

  const focusSingleElement = useCallback((element: UModelElement) => {
    setMode('graph')
    setFilterStacking(false)
    setFocusIds([elementKey(element)])
    setSelected(element)
  }, [])

  const updateDraft = useCallback((updater: (items: UModelElement[]) => UModelElement[], nextMessage?: string) => {
    setDraftElements((items) => {
      const before = cloneElements(items)
      const after = updater(cloneElements(items))
      setUndoStack((stack) => [...stack, before].slice(-10))
      setRedoStack([])
      return after
    })
    if (nextMessage) setMessage(nextMessage)
  }, [])

  const copyDraftElement = useCallback((element: UModelElement) => {
    const copy = cloneElementForDraft(element)
    updateDraft((items) => [...items, copy], t('umodelExplorer.message.createdDraftElement', { name: titleForElement(copy) }))
    setSelected(copy)
    setMode('graph')
    setFocusIds([elementKey(element), elementKey(copy)])
  }, [t, updateDraft])

  const copyDraftCascadeElement = useCallback((element: UModelElement) => {
    const copy = cloneElementForDraft(element)
    updateDraft((items) => {
      if (isLinkElement(element)) return [...items, copy]
      const relatedLinks = items.filter((item) => isLinkElement(item) && linkTouchesElement(item, element, items))
      const linkCopies = relatedLinks.map((link) => cloneLinkForDraft(link, element, copy))
      return [...items, copy, ...linkCopies]
    }, t('umodelExplorer.message.createdDraftElement', { name: titleForElement(copy) }))
    setSelected(copy)
    setMode('graph')
    setFocusIds([elementKey(element), elementKey(copy)])
  }, [t, updateDraft])

  const deleteDraftElement = useCallback((element: UModelElement, cascade: boolean) => {
    updateDraft((items) => {
      const ids = new Set<string>([elementKey(element)])
      if (cascade && !isLinkElement(element)) {
        for (const item of items) {
          if (isLinkElement(item) && linkTouchesElement(item, element, items)) ids.add(elementKey(item))
        }
      }
      return items.filter((item) => !ids.has(elementKey(item)))
    }, cascade ? t('umodelExplorer.message.deletedDraftNodeLinks') : t('umodelExplorer.message.deletedDraftElement'))
    setSelected((current) => (current && elementKey(current) === elementKey(element) ? null : current))
  }, [t, updateDraft])

  useEffect(() => {
    function handleKeyboardDelete(event: KeyboardEvent) {
      if (!selected || diffOpen || createOpen || uploadOpen || connectOpen) return
      if (event.key !== 'Backspace' && event.key !== 'Delete') return
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, select, [contenteditable="true"], .monaco-editor')) return
      event.preventDefault()
      deleteDraftElement(selected, true)
    }
    window.addEventListener('keydown', handleKeyboardDelete)
    return () => window.removeEventListener('keydown', handleKeyboardDelete)
  }, [connectOpen, createOpen, deleteDraftElement, diffOpen, selected, uploadOpen])

  const replaceDraftElement = useCallback((next: UModelElement) => {
    updateDraft((items) => upsertById(items, next))
    setSelected(next)
  }, [updateDraft])

  const createDraftElement = useCallback((next: UModelElement) => {
    updateDraft((items) => upsertById(items, next), t('umodelExplorer.message.createdDraftElement', { name: titleForElement(next) }))
    setSelected(next)
    setMode('graph')
    setFocusIds([elementKey(next)])
    setCreateOpen(false)
  }, [t, updateDraft])

  const uploadDraftElements = useCallback((items: UModelElement[]) => {
    updateDraft((current) => {
      let next = current
      for (const item of items) next = upsertById(next, item)
      return next
    }, t(items.length === 1 ? 'umodelExplorer.message.uploadedDraftElements' : 'umodelExplorer.message.uploadedDraftElementsOther', { count: items.length }))
    setSelected(items[0] || null)
    setMode('graph')
    setFocusIds(focusIdsForElements(items, [...draftElements, ...items]))
    setUploadOpen(false)
  }, [draftElements, t, updateDraft])

  const createDraftLinks = useCallback((source: UModelElement, targets: UModelElement[]) => {
    if (targets.length === 0) return
    const links = targets.map((target) => createDataLink(source, target))
    updateDraft((items) => {
      let next = items
      for (const link of links) next = upsertById(next, link)
      return next
    }, t(links.length === 1 ? 'umodelExplorer.message.createdDraftLinks' : 'umodelExplorer.message.createdDraftLinksOther', {
      count: links.length,
      name: titleForElement(source),
    }))
    setSelected(links[0])
    setMode('graph')
    setFocusIds([...new Set([elementKey(source), ...targets.map(elementKey)])])
    setConnectOpen(false)
    setConnectSource(null)
  }, [t, updateDraft])

  const openConnectFromNode = useCallback((element: UModelElement) => {
    setConnectSource(element)
    setConnectOpen(true)
  }, [])

  const diff = useMemo(() => diffElements(serverElements, draftElements), [draftElements, serverElements])
  const focusDraftChanges = useCallback(() => {
    const focusable = [...diff.added, ...diff.modified]
    if (focusable.length === 0) return
    setMode('graph')
    setFilterStacking(false)
    setFocusIds(focusIdsForElements(focusable, draftElements))
    setSelected(focusable[0] || null)
  }, [diff.added, diff.modified, draftElements])
  const draftStatusById = useMemo(() => {
    const status = new Map<string, DraftStatus>()
    for (const element of diff.added) status.set(elementKey(element), 'added')
    for (const element of diff.modified) status.set(elementKey(element), 'modified')
    return status
  }, [diff])

  const graphSource = useMemo(
    () =>
      buildGraph(filtered, {
        onSelect: setSelected,
        onFocus: focusElement,
        onConnect: openConnectFromNode,
        onCopy: copyDraftElement,
        onCopyCascade: copyDraftCascadeElement,
        onDelete: deleteDraftElement,
      }, draftStatusById, entitySetLinkDisplay),
    [
      copyDraftCascadeElement,
      copyDraftElement,
      deleteDraftElement,
      draftStatusById,
      entitySetLinkDisplay,
      filtered,
      focusElement,
      openConnectFromNode,
    ],
  )

  useEffect(() => {
    let cancelled = false
    setGraph(graphSource)
    if (mode !== 'graph' || graphSource.nodes.length === 0) return
    setLayouting(true)
    layoutGraphWithGraphviz(graphSource)
      .then((nextGraph) => {
        if (!cancelled) setGraph(nextGraph)
      })
      .catch(() => {
        if (!cancelled) setGraph(graphSource)
      })
      .finally(() => {
        if (!cancelled) setLayouting(false)
      })
    return () => {
      cancelled = true
    }
  }, [graphSource, mode])

  const handleGraphNodesChange = useCallback((changes: NodeChange<Node<UModelNodeData>>[]) => {
    setGraph((current) => ({
      ...current,
      nodes: applyNodeChanges(changes, current.nodes),
    }))
  }, [])

  const hasChanges = diff.added.length + diff.modified.length + diff.deleted.length > 0
  const activeCount =
    kindFilters.length + domainFilters.length + focusIds.length + fullTextFilters.length

  async function importSample() {
    setLoading(true)
    setMessage('')
    setError('')
    try {
      const result = await api.importSampleData(workspaceId)
      await load()
      setMessage(t('umodelExplorer.message.importedSample', {
        umodel: result.umodel.imported,
        entities: result.entities.accepted,
        relations: result.relations.accepted,
      }))
    } catch (nextError) {
      setError(formatError(nextError))
    } finally {
      setLoading(false)
    }
  }

  async function commitDraft() {
    if (!hasChanges) return
    setCommitting(true)
    setSubmitFailure(null)
    setError('')
    setMessage('')
    try {
      const upserts = [...diff.added, ...diff.modified]
      if (upserts.length > 0) {
        const validation = await api.validateUModel(workspaceId, upserts)
        if (!validation.valid) {
          const failure = buildSubmitFailure(
            t('umodelExplorer.submitFailure.validation'),
            formatValidationErrorLines(validation.errors, t),
          )
          setSubmitFailure(failure)
          setError(failure.summary)
          return
        }
        const result = await api.putUModel(workspaceId, upserts)
        const failure = formatWriteFailure(result, t('umodelExplorer.submitFailure.operation.upsert'), t)
        if (failure) {
          setSubmitFailure(failure)
          setError(failure.summary)
          return
        }
      }
      if (diff.deleted.length > 0) {
        const result = await api.deleteUModel(workspaceId, diff.deleted.map(elementKey))
        const failure = formatWriteFailure(result, t('umodelExplorer.submitFailure.operation.delete'), t)
        if (failure) {
          setSubmitFailure(failure)
          setError(failure.summary)
          return
        }
      }
      setDiffOpen(false)
      setSubmitFailure(null)
      await load()
      setMessage(t('umodelExplorer.message.submitted', { upserts: upserts.length, deletes: diff.deleted.length }))
    } catch (nextError) {
      const failure = buildSubmitFailure(t('umodelExplorer.submitFailure.unexpected'), [formatError(nextError)])
      setSubmitFailure(failure)
      setError(failure.summary)
    } finally {
      setCommitting(false)
    }
  }

  function clearFilters() {
    setKindFilters([])
    setDomainFilters([])
    setFocusIds([])
    setFullTextFilters([])
    setFilterStacking(false)
    setSearchText('')
  }

  function applyFullTextSearch(text = searchText) {
    const value = text.trim()
    if (!value) return
    setFullTextFilters((items) => (items.includes(value) ? items : [...items, value]))
    setSearchText('')
    setSearchPanelOpen(false)
  }

  function undoDraft() {
    if (undoStack.length === 0) return
    setDraftElements((current) => {
      const previous = undoStack[undoStack.length - 1]
      setUndoStack((items) => items.slice(0, -1))
      setRedoStack((items) => [cloneElements(current), ...items].slice(0, 10))
      return cloneElements(previous)
    })
    setSelected(null)
    setMessage(t('umodelExplorer.message.undoDraftChange'))
  }

  function redoDraft() {
    if (redoStack.length === 0) return
    setDraftElements((current) => {
      const next = redoStack[0]
      setRedoStack((items) => items.slice(1))
      setUndoStack((items) => [...items, cloneElements(current)].slice(-10))
      return cloneElements(next)
    })
    setSelected(null)
    setMessage(t('umodelExplorer.message.redoDraftChange'))
  }

  return (
    <div className="ume-v2 umodel-page">
      <aside className="ume-sidebar">
        <div className="ume-sidebar-tabs">
          <button className={sidebarTab === 'summary' ? 'active' : ''} onClick={() => setSidebarTab('summary')} type="button" title={t('umodelExplorer.tabs.summary')}>
            <Layers size={15} />
          </button>
          <button className={sidebarTab === 'settings' ? 'active' : ''} onClick={() => setSidebarTab('settings')} type="button" title={t('umodelExplorer.tabs.settings')}>
            <Settings2 size={15} />
          </button>
        </div>

        {sidebarTab === 'summary' ? (
          <SummarySidebar
            stats={stats}
            diff={diff}
            kindFilters={kindFilters}
            domainFilters={domainFilters}
            currentView={mode}
            entitySetLinkDisplay={entitySetLinkDisplay}
            onFocusDraftChanges={focusDraftChanges}
            onToggleKind={(kind) => setKindFilters((items) => toggleValue(items, kind))}
            onToggleDomain={(domain) => setDomainFilters((items) => toggleValue(items, domain))}
          />
        ) : (
          <SettingsSidebar
            backgroundStyle={backgroundStyle}
            entitySetLinkDisplay={entitySetLinkDisplay}
            forceFullMode={forceFullMode}
            onBackgroundStyleChange={setBackgroundStyle}
            onEntitySetLinkDisplayChange={setEntitySetLinkDisplay}
            onForceFullModeChange={setForceFullMode}
          />
        )}
      </aside>

      <section className="ume-main">
        <header className="ume-topbar">
          <div className="ume-topbar-left">
            <SegmentedControl
              className="ume-view-segmented"
              value={mode}
              onChange={setMode}
              items={[
                { value: 'graph', label: t('umodelExplorer.view.graph'), icon: <GitBranch size={14} /> },
                { value: 'table', label: t('umodelExplorer.view.table'), icon: <Table2 size={14} /> },
              ]}
            />
          </div>

          <div className="ume-search-wrap" ref={searchWrapRef}>
            <Search size={14} />
            <input
              value={searchText}
              onBlur={() => {
                searchBlurRef.current = window.setTimeout(() => setSearchPanelOpen(false), 180)
              }}
              onChange={(event) => {
                setSearchText(event.target.value)
                setAddMenuOpen(false)
                setSearchPanelOpen(true)
              }}
              onFocus={() => {
                if (searchBlurRef.current) window.clearTimeout(searchBlurRef.current)
                setAddMenuOpen(false)
                setSearchPanelOpen(true)
              }}
              onMouseDown={() => {
                setAddMenuOpen(false)
                setSearchPanelOpen(true)
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  applyFullTextSearch()
                  setSearchPanelOpen(false)
                }
                if (event.key === 'Escape') setSearchPanelOpen(false)
              }}
              placeholder={t('umodelExplorer.search.placeholder')}
            />
            {searchPanelOpen && (
              <SearchPanel
                index={searchIndex}
                query={searchText}
                currentView={mode}
                entitySetLinkDisplay={entitySetLinkDisplay}
                style={searchPanelStyle}
                onApplyDomain={(domain) => setDomainFilters((items) => toggleValue(items, domain))}
                onApplyFullText={applyFullTextSearch}
                onApplyKind={(kind) => setKindFilters((items) => toggleValue(items, kind))}
                onClose={() => setSearchPanelOpen(false)}
                onFocusElement={focusElement}
              />
            )}
          </div>

          <div className="ume-topbar-actions">
            {error && <span className="ume-toast danger">{error}</span>}
            <span className="ume-action-divider" />
            <div className="ume-add-menu-wrap">
              <IconButton className="ume-icon-button" label={t('umodelExplorer.action.add')} onClick={() => setAddMenuOpen((value) => !value)} type="button">
                <Plus size={15} />
              </IconButton>
              {addMenuOpen && (
                <div className="ume-add-menu">
                  <button onClick={() => { setAddMenuOpen(false); setUploadOpen(true) }} type="button">
                    <FileUp size={14} />
                    {t('umodelExplorer.action.uploadYamlJson')}
                  </button>
                  <button onClick={() => { setAddMenuOpen(false); setCreateOpen(true) }} type="button">
                    <Box size={14} />
                    {t('umodelExplorer.action.createNode')}
                  </button>
                  <button onClick={() => { setAddMenuOpen(false); setConnectSource(null); setConnectOpen(true) }} type="button">
                    <Cable size={14} />
                    {t('umodelExplorer.action.createLink')}
                  </button>
                </div>
              )}
            </div>
            <IconButton className="ume-icon-button" label={t('umodelExplorer.action.refreshResetDraft')} onClick={() => void load()} type="button">
              <RefreshCcw size={15} />
            </IconButton>
            <IconButton className="ume-icon-button" disabled={undoStack.length === 0} label={t('umodelExplorer.action.undo')} onClick={undoDraft} type="button">
              <Undo2 size={15} />
            </IconButton>
            <IconButton className="ume-icon-button" disabled={redoStack.length === 0} label={t('umodelExplorer.action.redo')} onClick={redoDraft} type="button">
              <Redo2 size={15} />
            </IconButton>
            <button
              className="ume-submit-button"
              disabled={!hasChanges}
              onClick={() => {
                setSubmitFailure(null)
                setDiffOpen(true)
              }}
              type="button"
              title={t('umodelExplorer.action.reviewDiffSubmit')}
            >
              <Save size={14} />
              {t('umodelExplorer.action.submit')}
              {hasChanges && <span>{diff.added.length + diff.modified.length + diff.deleted.length}</span>}
            </button>
          </div>
        </header>

        <FilterBar
          activeCount={activeCount}
          focusIds={focusIds}
          fullTextFilters={fullTextFilters}
          kindFilters={kindFilters}
          domainFilters={domainFilters}
          filterStacking={filterStacking}
          draftElements={draftElements}
          currentView={mode}
          entitySetLinkDisplay={entitySetLinkDisplay}
          onClear={clearFilters}
          onRemoveFullText={(value) => setFullTextFilters((items) => items.filter((item) => item !== value))}
          onRemoveKind={(value) => setKindFilters((items) => items.filter((item) => item !== value))}
          onRemoveDomain={(value) => setDomainFilters((items) => items.filter((item) => item !== value))}
          onRemoveFocus={(value) => setFocusIds((items) => items.filter((item) => item !== value))}
          onClearFocus={() => setFocusIds([])}
          onToggleStacking={() => setFilterStacking((value) => !value)}
        />
        <div className="ume-separator horizontal" />

        <div className="ume-content-area">
          <main className="ume-content-main">
            {loading && draftElements.length === 0 && <div className="ume-loading">{t('umodelExplorer.loading.graph')}</div>}
            {loading && draftElements.length > 0 && <div className="ume-layout-badge">{t('umodelExplorer.loading.refreshingApi')}</div>}
            {!loading && draftElements.length === 0 && (
              <div className="ume-empty-wrap">
                <EmptyState
                  title={t('umodelExplorer.empty.elements.title')}
                  detail={t('umodelExplorer.empty.elements.detail')}
                  action={
                    <Button variant="primary" onClick={() => void importSample()}>
                      <Database size={16} />
                      {t('umodelExplorer.action.importQuickstartSample')}
                    </Button>
                  }
                />
              </div>
            )}
            {draftElements.length > 0 && mode === 'graph' && (
              <ReactFlowProvider>
                <GraphView
                  graph={graph}
                  focusIds={focusIds}
                  zoomLevel={zoomLevel}
                  layouting={layouting}
                  backgroundStyle={backgroundStyle}
                  forceFullMode={forceFullMode}
                  selectedId={selected ? elementKey(selected) : null}
                  onZoomLevelChange={setZoomLevel}
                  onNodesChange={handleGraphNodesChange}
                  onSelect={setSelected}
                />
              </ReactFlowProvider>
            )}
            {draftElements.length > 0 && mode === 'table' && (
              <TableView elements={filtered} selected={selected} onDelete={deleteDraftElement} onSelect={setSelected} />
            )}
          </main>

          <DetailPanel
            api={api}
            workspaceId={workspaceId}
            element={selected}
            width={detailPanelWidth}
            onApply={replaceDraftElement}
            onClose={() => setSelected(null)}
            onWidthChange={setDetailPanelWidth}
          />
        </div>

        <footer className="ume-statusbar">
          <span><strong>{filteredStats.nodes}</strong> {t('umodelExplorer.status.nodes')}</span>
          <span className="ume-status-sep" />
          <span><strong>{filteredStats.links}</strong> {t('umodelExplorer.status.links')}</span>
          {resultLimitReached && resultLimit && (
            <>
              <span className="ume-status-sep" />
              <span>{t('umodelExplorer.status.limit', { limit: resultLimit.toLocaleString() })}</span>
            </>
          )}
        </footer>
      </section>

      {createOpen && (
        <CreateNodeDialog
          api={api}
          workspaceId={workspaceId}
          elements={draftElements}
          onClose={() => setCreateOpen(false)}
          onCreate={createDraftElement}
        />
      )}
      {uploadOpen && (
        <UploadDialog
          api={api}
          workspaceId={workspaceId}
          elements={draftElements}
          onClose={() => setUploadOpen(false)}
          onUpload={uploadDraftElements}
        />
      )}
      {connectOpen && (
        <ConnectDialog
          source={connectSource}
          elements={draftElements}
          onClose={() => {
            setConnectOpen(false)
            setConnectSource(null)
          }}
          onConnect={createDraftLinks}
        />
      )}
      {diffOpen && (
        <DiffDialog
          diff={diff}
          serverElements={serverElements}
          committing={committing}
          submitFailure={submitFailure}
          onClose={() => {
            setDiffOpen(false)
            setSubmitFailure(null)
          }}
          onFocusElement={focusSingleElement}
          onSubmit={() => void commitDraft()}
        />
      )}
    </div>
  )
}

function TableView({
  elements,
  selected,
  onDelete,
  onSelect,
}: {
  elements: UModelElement[]
  selected: UModelElement | null
  onDelete: (element: UModelElement, cascade: boolean) => void
  onSelect: (element: UModelElement) => void
}) {
  const { t } = useI18n()
  type SortKey = 'name' | 'domain' | 'kind' | 'description'
  const [sortKey, setSortKey] = useState<SortKey>('name')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)
  const rows = useMemo(() => {
    const sorted = [...elements].sort((left, right) => {
      const leftValue = tableSortValue(left, sortKey)
      const rightValue = tableSortValue(right, sortKey)
      const result = leftValue.localeCompare(rightValue)
      return sortDir === 'asc' ? result : -result
    })
    return sorted
  }, [elements, sortDir, sortKey])
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize))
  const pageNumbers = useMemo(() => paginationItems(page, totalPages), [page, totalPages])
  const pagedRows = rows.slice((page - 1) * pageSize, page * pageSize)

  useEffect(() => {
    setPage(1)
  }, [elements.length, pageSize, sortDir, sortKey])

  function changeSort(nextKey: SortKey) {
    if (sortKey === nextKey) setSortDir((value) => (value === 'asc' ? 'desc' : 'asc'))
    else {
      setSortKey(nextKey)
      setSortDir('asc')
    }
  }

  function sortArrow(key: SortKey) {
    if (sortKey !== key) return <span className="ume-sort-arrow muted">↕</span>
    return <span className="ume-sort-arrow">{sortDir === 'asc' ? '↑' : '↓'}</span>
  }

  return (
    <div className="ume-table-view">
      <div className="ume-table-scroll">
        <table className="ume-data-table">
          <thead>
            <tr>
              <th className="col-icon" />
              <th className="col-name" onClick={() => changeSort('name')} role="button">{t('umodelExplorer.table.name')} {sortArrow('name')}</th>
              <th className="col-domain" onClick={() => changeSort('domain')} role="button">{t('umodelExplorer.table.domain')} {sortArrow('domain')}</th>
              <th className="col-type" onClick={() => changeSort('kind')} role="button">{t('umodelExplorer.table.type')} {sortArrow('kind')}</th>
              <th className="col-description" onClick={() => changeSort('description')} role="button">{t('umodelExplorer.table.description')} {sortArrow('description')}</th>
              <th className="col-detail">{t('umodelExplorer.table.detail')}</th>
              <th className="col-action">{t('umodelExplorer.table.action')}</th>
            </tr>
          </thead>
          <tbody>
            {pagedRows.map((element) => {
              const key = elementKey(element)
              const color = colorForKind(element.kind)
              const detail = detailShort(element, {
                fields: t('umodelExplorer.unit.fields'),
                metrics: t('umodelExplorer.unit.metrics'),
              })
              return (
                <tr key={key} className={selected && elementKey(selected) === key ? 'selected' : ''} onClick={() => onSelect(element)}>
                  <td className="cell-icon">
                    <span className={isLinkElement(element) ? 'ume-kind-line table' : 'ume-kind-dot table'} style={{ background: color.dot }} />
                  </td>
                  <td>
                    <div className="cell-name-text">
                      <strong>{titleForElement(element)}</strong>
                      <code>{key}</code>
                    </div>
                  </td>
                  <td>{element.domain || '-'}</td>
                  <td>
                    <span className="ume-kind-badge" style={{ background: color.bg, color: color.text }}>
                      {color.label}
                    </span>
                  </td>
                  <td className="cell-desc">{descriptionForElement(element) || '-'}</td>
                  <td><code>{detail || '-'}</code></td>
                  <td>
                    <TableDeleteButton element={element} onDelete={onDelete} />
                  </td>
                </tr>
              )
            })}
            {pagedRows.length === 0 && (
              <tr>
                <td colSpan={7} className="ume-empty-cell">{t('umodelExplorer.empty.matchingModels')}</td>
              </tr>
            )
            }
          </tbody>
        </table>
      </div>
      <div className="ume-table-footer">
        <div className="ume-table-pages">
          <button disabled={page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))} type="button">‹</button>
          {pageNumbers.map((item, index) => item === '...'
            ? <span key={`gap-${index}`}>…</span>
            : (
              <button key={item} className={item === page ? 'active' : ''} onClick={() => setPage(item)} type="button">
                {item}
              </button>
            ))}
          <button disabled={page >= totalPages} onClick={() => setPage((value) => Math.min(totalPages, value + 1))} type="button">›</button>
        </div>
        <select value={pageSize} onChange={(event) => setPageSize(Number(event.target.value))}>
          {[20, 50, 100].map((size) => <option key={size} value={size}>{t('umodelExplorer.table.pageSize', { size })}</option>)}
        </select>
        <span>{t.rich('umodelExplorer.status.totalItemsRich', {
          strong: (chunks) => <strong>{chunks}</strong>,
        }, { count: rows.length })}</span>
      </div>
    </div>
  )
}

function TableDeleteButton({
  element,
  onDelete,
}: {
  element: UModelElement
  onDelete: (element: UModelElement, cascade: boolean) => void
}) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [rect, setRect] = useState<DOMRect | null>(null)
  const buttonRef = useRef<HTMLButtonElement | null>(null)
  const isLink = isLinkElement(element)

  useEffect(() => {
    if (!open) return
    const close = () => setOpen(false)
    const updateRect = () => {
      if (buttonRef.current) setRect(buttonRef.current.getBoundingClientRect())
    }
    updateRect()
    window.addEventListener('click', close)
    window.addEventListener('resize', updateRect)
    window.addEventListener('scroll', updateRect, true)
    return () => {
      window.removeEventListener('click', close)
      window.removeEventListener('resize', updateRect)
      window.removeEventListener('scroll', updateRect, true)
    }
  }, [open])

  function handleClick(event: React.MouseEvent) {
    event.stopPropagation()
    if (isLink) {
      onDelete(element, false)
      return
    }
    setOpen((value) => !value)
  }

  return (
    <>
      <button ref={buttonRef} className="ume-table-delete-button" onClick={handleClick} type="button" title={isLink ? t('umodelExplorer.action.deleteLink') : t('umodelExplorer.action.delete')}>
        <Trash2 size={13} />
      </button>
      {open && rect && ReactDOM.createPortal(
        <div
          className="ume-node-menu floating table-delete"
          style={{
            left: Math.min(Math.max(8, rect.right - 180), window.innerWidth - 188),
            top: rect.bottom + 6,
          } as CSSProperties}
          onMouseDown={(event) => event.stopPropagation()}
          onClick={(event) => event.stopPropagation()}
        >
          <button className="danger" onClick={() => { onDelete(element, true); setOpen(false) }} type="button">
            {t('umodelExplorer.action.deleteNode')}
          </button>
          <button className="danger" onClick={() => { onDelete(element, true); setOpen(false) }} type="button">
            {t('umodelExplorer.action.deleteWithEdges')}
          </button>
        </div>,
        document.body,
      )}
    </>
  )
}

function DetailPanel({
  api,
  workspaceId,
  element,
  width,
  onClose,
  onApply,
  onWidthChange,
}: {
  api: UModelApi
  workspaceId: string
  element: UModelElement | null
  width: number
  onClose: () => void
  onApply: (element: UModelElement) => void
  onWidthChange: (width: number) => void
}) {
  const { t } = useI18n()
  const [json, setJson] = useState('')
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)
  const [resizing, setResizing] = useState(false)

  useEffect(() => {
    setJson(element ? stringify(element) : '')
    setStatus('')
  }, [element])

  useEffect(() => {
    return () => document.body.classList.remove('ume-resizing-detail')
  }, [])

  if (!element) return null

  const color = colorForKind(element.kind)

  function startResize(event: React.PointerEvent<HTMLDivElement>) {
    event.preventDefault()
    const startX = event.clientX
    const startWidth = width
    const maxWidth = Math.min(760, Math.floor(window.innerWidth * 0.58))
    setResizing(true)
    document.body.classList.add('ume-resizing-detail')
    const handleMove = (moveEvent: PointerEvent) => {
      const nextWidth = startWidth - (moveEvent.clientX - startX)
      onWidthChange(Math.min(maxWidth, Math.max(320, nextWidth)))
    }
    const stopResize = () => {
      setResizing(false)
      document.body.classList.remove('ume-resizing-detail')
      window.removeEventListener('pointermove', handleMove)
      window.removeEventListener('pointerup', stopResize)
      window.removeEventListener('pointercancel', stopResize)
    }
    window.addEventListener('pointermove', handleMove)
    window.addEventListener('pointerup', stopResize)
    window.addEventListener('pointercancel', stopResize)
  }

  async function apply() {
    if (!element) return
    setBusy(true)
    setStatus('')
    try {
      const next = parseJson<UModelElement>(json, t('umodelExplorer.validation.elementLabel'))
      const validation = await api.validateUModel(workspaceId, [next])
      if (!validation.valid) {
        setStatus(formatValidationErrors(validation.errors, t))
        return
      }
      onApply(next)
      setStatus('')
    } catch (nextError) {
      setStatus(formatError(nextError))
    } finally {
      setBusy(false)
    }
  }

  return (
    <aside className={`ume-detail-panel open${resizing ? ' resizing' : ''}`} style={{ '--ume-detail-width': `${width}px` } as CSSProperties}>
      <div className="ume-detail-resizer" onPointerDown={startResize} role="separator" aria-orientation="vertical" aria-label={t('umodelExplorer.aria.resizeDetailPanel')} />
      <header className="ume-detail-header">
        <span className="ume-detail-stripe" style={{ background: color.color }} />
        <div className="ume-detail-title">
          <strong>{titleForElement(element)}</strong>
          <code>{element.domain || t('umodelExplorer.misc.unknown')}@{element.name || elementKey(element)}</code>
        </div>
        <span className="ume-kind-badge" style={{ background: color.bg, color: color.text }}>{color.label}</span>
        <button className="ume-icon-button subtle" onClick={onClose} type="button" title={t('umodelExplorer.action.close')}>
          <X size={15} />
        </button>
      </header>
      <div className="ume-separator horizontal" />
      <div className="ume-detail-body">
        <MonacoJsonEditor value={json} onChange={setJson} />
        {status && <pre className="ume-result-box">{status}</pre>}
      </div>
      <footer className="ume-detail-footer">
        <button className="ume-primary-button" disabled={busy} onClick={() => void apply()} type="button">
          <Save size={14} />
          {t('umodelExplorer.action.applyToDraft')}
        </button>
      </footer>
    </aside>
  )
}

function MonacoJsonEditor({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <div className="ume-monaco-wrap">
      <Editor
        value={value}
        defaultLanguage="json"
        theme="vs"
        onChange={(next) => onChange(next || '')}
        options={{
          automaticLayout: true,
          fontFamily: 'SF Mono, Fira Code, Consolas, monospace',
          fontSize: 12,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          tabSize: 2,
          wordWrap: 'on',
        }}
      />
    </div>
  )
}

function CreateNodeDialog({
  api,
  workspaceId,
  elements,
  onClose,
  onCreate,
}: {
  api: UModelApi
  workspaceId: string
  elements: UModelElement[]
  onClose: () => void
  onCreate: (element: UModelElement) => void
}) {
  const { t } = useI18n()
  const [selectedKind, setSelectedKind] = useState('entity_set')
  const [json, setJson] = useState(() => stringify(defaultNewNode('entity_set')))
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setJson(stringify(defaultNewNode(selectedKind)))
    setStatus('')
  }, [selectedKind])

  async function create() {
    setBusy(true)
    setStatus('')
    try {
      const next = parseJson<UModelElement>(json, t('umodelExplorer.validation.elementLabel'))
      const key = elementKey(next)
      if (elements.some((element) => elementKey(element) === key)) {
        setStatus(t('umodelExplorer.validation.duplicateElement', { id: key }))
        return
      }
      const validation = await api.validateUModel(workspaceId, [next])
      if (!validation.valid) {
        setStatus(formatValidationErrors(validation.errors, t))
        return
      }
      onCreate(next)
    } catch (nextError) {
      setStatus(formatError(nextError))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="ume-dialog-backdrop">
      <section className="ume-dialog ume-dialog-wide ume-create-node-dialog">
        <header>
          <div>
            <strong>{t('umodelExplorer.dialog.create.title')}</strong>
            <span>{t('umodelExplorer.dialog.create.description')}</span>
          </div>
          <button className="ume-icon-button subtle" onClick={onClose} type="button">
            <X size={15} />
          </button>
        </header>
        <div className="ume-create-node-body">
          <aside className="ume-create-kind-list">
            <span>{t('umodelExplorer.dialog.create.type')}</span>
            {nodeKindOptions.map((option) => {
              const color = colorForKind(option.value)
              const active = option.value === selectedKind
              return (
                <button
                  key={option.value}
                  className={active ? 'active' : ''}
                  onClick={() => setSelectedKind(option.value)}
                  style={{ '--kind-bg': color.bg, '--kind-text': color.text, '--kind-dot': color.dot } as CSSProperties}
                  type="button"
                >
                  <i />
                  {option.label}
                </button>
              )
            })}
          </aside>
          <section className="ume-create-json-pane">
            <div className="ume-create-json-title">
              <span>{t('umodelExplorer.dialog.create.json')}</span>
              <strong>{labelForKind(selectedKind)}</strong>
            </div>
            <MonacoJsonEditor value={json} onChange={setJson} />
            {status && <pre className="ume-result-box">{status}</pre>}
          </section>
        </div>
        <footer>
          <button className="ume-secondary-inline" onClick={onClose} type="button">{t('common.cancel')}</button>
          <button className="ume-primary-button" disabled={busy} onClick={() => void create()} type="button">
            <Plus size={14} />
            {t('common.create')}
          </button>
        </footer>
      </section>
    </div>
  )
}

function UploadDialog({
  api,
  workspaceId,
  elements,
  onClose,
  onUpload,
}: {
  api: UModelApi
  workspaceId: string
  elements: UModelElement[]
  onClose: () => void
  onUpload: (elements: UModelElement[]) => void
}) {
  const { t } = useI18n()
  const [text, setText] = useState(() => stringify([defaultNewNode()]))
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)
  const duplicateIds = useMemo(() => {
    const ids = new Set(elements.map(elementKey))
    return parseUploadPreview(text).filter((element) => ids.has(elementKey(element))).map(elementKey)
  }, [elements, text])

  async function upload() {
    setBusy(true)
    setStatus('')
    try {
      const next = parseUModelElementsFromYamlOrJson(text, t)
      if (next.length === 0) {
        setStatus(t('umodelExplorer.validation.noElementsFound'))
        return
      }
      const validation = await api.validateUModel(workspaceId, next)
      if (!validation.valid) {
        setStatus(formatValidationErrors(validation.errors, t))
        return
      }
      onUpload(next)
    } catch (nextError) {
      setStatus(formatError(nextError))
    } finally {
      setBusy(false)
    }
  }

  async function readFile(file: File) {
    setStatus('')
    setText(await file.text())
  }

  return (
    <div className="ume-dialog-backdrop">
      <section className="ume-dialog ume-dialog-wide">
        <header>
          <div>
            <strong>{t('umodelExplorer.dialog.upload.title')}</strong>
            <span>{t('umodelExplorer.dialog.upload.description')}</span>
          </div>
          <button className="ume-icon-button subtle" onClick={onClose} type="button">
            <X size={15} />
          </button>
        </header>
        <div className="ume-upload-toolbar">
          <label>
            <FileUp size={14} />
            {t('umodelExplorer.action.chooseFile')}
            <input
              accept=".json,.yaml,.yml,application/json,text/yaml,text/x-yaml"
              onChange={(event) => {
                const file = event.target.files?.[0]
                if (file) void readFile(file)
              }}
              type="file"
            />
          </label>
          {duplicateIds.length > 0 && (
            <span>
              {t(duplicateIds.length === 1 ? 'umodelExplorer.dialog.upload.duplicateIds' : 'umodelExplorer.dialog.upload.duplicateIdsOther', {
                count: duplicateIds.length,
              })}
            </span>
          )}
        </div>
        <div className="ume-dialog-body">
          <div className="ume-monaco-wrap">
            <Editor
              value={text}
              defaultLanguage="yaml"
              theme="vs"
              onChange={(next) => setText(next || '')}
              options={{
                automaticLayout: true,
                fontFamily: 'SF Mono, Fira Code, Consolas, monospace',
                fontSize: 12,
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                tabSize: 2,
                wordWrap: 'on',
              }}
            />
          </div>
          {status && <pre className="ume-result-box">{status}</pre>}
        </div>
        <footer>
          <button className="ume-secondary-inline" onClick={onClose} type="button">{t('common.cancel')}</button>
          <button className="ume-primary-button" disabled={busy} onClick={() => void upload()} type="button">
            <FileUp size={14} />
            {t('umodelExplorer.action.addToDraft')}
          </button>
        </footer>
      </section>
    </div>
  )
}

interface NodePickerRow {
  id: string
  element: UModelElement
  title: string
  domain: string
  kind: string
  isTemp: boolean
}

function buildNodePickerRows(elements: UModelElement[], serverElements?: UModelElement[], unknownLabel = 'unknown'): NodePickerRow[] {
  const serverIds = new Set((serverElements || []).map(elementKey))
  return elements
    .filter((element) => !isLinkElement(element))
    .map((element) => ({
      id: elementKey(element),
      element,
      title: titleForElement(element),
      domain: element.domain || unknownLabel,
      kind: element.kind,
      isTemp: serverElements ? !serverIds.has(elementKey(element)) : false,
    }))
    .sort((left, right) => kindRank(left.kind) - kindRank(right.kind) || left.title.localeCompare(right.title))
}

function NodePickerTable({
  rows,
  selectedIds,
  singleSelect,
  onToggle,
}: {
  rows: NodePickerRow[]
  selectedIds: Set<string>
  singleSelect?: boolean
  onToggle: (id: string) => void
}) {
  const { t } = useI18n()
  const [kindFilter, setKindFilter] = useState('')
  const [domainFilter, setDomainFilter] = useState('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const pageSize = 20
  const kindOptions = useMemo(() => [...new Set(rows.map((row) => row.kind))].sort((left, right) => kindRank(left) - kindRank(right)), [rows])
  const domainOptions = useMemo(() => [...new Set(rows.map((row) => row.domain))].sort(), [rows])
  const filteredRows = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return rows.filter((row) => {
      if (kindFilter && row.kind !== kindFilter) return false
      if (domainFilter && row.domain !== domainFilter) return false
      if (!needle) return true
      return `${row.title} ${row.domain} ${row.kind} ${row.id}`.toLowerCase().includes(needle)
    })
  }, [domainFilter, kindFilter, rows, search])
  const totalPages = Math.max(1, Math.ceil(filteredRows.length / pageSize))
  const pagedRows = filteredRows.slice((page - 1) * pageSize, page * pageSize)

  useEffect(() => {
    setPage(1)
  }, [domainFilter, kindFilter, search, rows.length])

  return (
    <div className="ume-node-picker">
      <div className="ume-node-picker-toolbar">
        <select value={kindFilter} onChange={(event) => setKindFilter(event.target.value)}>
          <option value="">{t('umodelExplorer.search.allTypes')}</option>
          {kindOptions.map((kind) => (
            <option key={kind} value={kind}>{labelForKind(kind)}</option>
          ))}
        </select>
        <select value={domainFilter} onChange={(event) => setDomainFilter(event.target.value)}>
          <option value="">{t('umodelExplorer.search.allDomains')}</option>
          {domainOptions.map((domain) => (
            <option key={domain} value={domain}>{domain}</option>
          ))}
        </select>
        <label>
          <Search size={13} />
          <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t('umodelExplorer.search.placeholder')} />
        </label>
      </div>
      <div className="ume-node-picker-table">
        <table>
          <thead>
            <tr>
              <th />
              <th>{t('umodelExplorer.table.type')}</th>
              <th>{t('umodelExplorer.table.domain')}</th>
              <th>{t('umodelExplorer.table.name')}</th>
            </tr>
          </thead>
          <tbody>
            {pagedRows.map((row) => {
              const checked = selectedIds.has(row.id)
              const color = colorForKind(row.kind)
              return (
                <tr key={row.id} className={checked ? 'selected' : ''} onClick={() => onToggle(row.id)}>
                  <td>
                    <input type={singleSelect ? 'radio' : 'checkbox'} checked={checked} readOnly />
                  </td>
                  <td>
                    <span className="ume-node-picker-kind" style={{ background: color.bg, color: color.text }}>
                      {color.label}
                    </span>
                  </td>
                  <td>{row.domain}</td>
                  <td>
                    <div className="ume-node-picker-name">
                      <strong>{row.title}</strong>
                      <code>{row.id}</code>
                    </div>
                  </td>
                </tr>
              )
            })}
            {pagedRows.length === 0 && (
              <tr>
                <td colSpan={4} className="ume-empty-cell">{t('umodelExplorer.empty.matchingNodes')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <div className="ume-node-picker-footer">
        <span>
          {t('umodelExplorer.status.items', { count: filteredRows.length })} · {t('umodelExplorer.status.page', { page, total: totalPages })}
        </span>
        <button disabled={page <= 1} onClick={() => setPage((value) => Math.max(1, value - 1))} type="button">‹</button>
        <button disabled={page >= totalPages} onClick={() => setPage((value) => Math.min(totalPages, value + 1))} type="button">›</button>
      </div>
    </div>
  )
}

function ConnectDialog({
  source,
  elements,
  onClose,
  onConnect,
}: {
  source: UModelElement | null
  elements: UModelElement[]
  onClose: () => void
  onConnect: (source: UModelElement, targets: UModelElement[]) => void
}) {
  const { t } = useI18n()
  const allRows = useMemo(() => buildNodePickerRows(elements, undefined, t('umodelExplorer.misc.unknown')), [elements, t])
  const [step, setStep] = useState<'source' | 'target'>(source ? 'target' : 'source')
  const [selectedSourceId, setSelectedSourceId] = useState(source ? elementKey(source) : '')
  const [selectedTargetIds, setSelectedTargetIds] = useState<Set<string>>(new Set())
  const selectedSource = source || allRows.find((row) => row.id === selectedSourceId)?.element || null
  const sourceRows = allRows
  const targetRows = allRows.filter((row) => row.id !== (selectedSource ? elementKey(selectedSource) : ''))
  const targetCount = selectedTargetIds.size

  function toggleSource(id: string) {
    setSelectedSourceId((current) => (current === id ? '' : id))
    setSelectedTargetIds(new Set())
    if (id !== selectedSourceId) setStep('target')
  }

  function toggleTarget(id: string) {
    setSelectedTargetIds((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function confirm() {
    if (!selectedSource || targetCount === 0) return
    const targets = targetRows.filter((row) => selectedTargetIds.has(row.id)).map((row) => row.element)
    onConnect(selectedSource, targets)
  }

  return (
    <div className="ume-dialog-backdrop">
      <section className="ume-dialog ume-dialog-wide ume-connect-dialog">
        <header>
          <div>
            <strong>{source ? t('umodelExplorer.action.connectTo') : t('umodelExplorer.dialog.connect.newLink')}</strong>
            <span>
              {source
                ? t('umodelExplorer.dialog.connect.fromSource', { name: titleForElement(source) })
                : t('umodelExplorer.dialog.connect.description')}
            </span>
          </div>
          <button className="ume-icon-button subtle" onClick={onClose} type="button">
            <X size={15} />
          </button>
        </header>
        {!source && (
          <div className="ume-connect-steps">
            <button className={step === 'source' ? 'active' : ''} onClick={() => setStep('source')} type="button">
              {t('umodelExplorer.dialog.connect.stepSource')} {selectedSource ? `(${titleForElement(selectedSource)})` : ''}
            </button>
            <button className={step === 'target' ? 'active' : ''} disabled={!selectedSourceId} onClick={() => setStep('target')} type="button">
              {t('umodelExplorer.dialog.connect.stepTarget')} {targetCount > 0 ? `(${targetCount})` : ''}
            </button>
          </div>
        )}
        <div className="ume-dialog-body">
          {step === 'source' && !source ? (
            <NodePickerTable
              rows={sourceRows}
              selectedIds={new Set(selectedSourceId ? [selectedSourceId] : [])}
              singleSelect
              onToggle={toggleSource}
            />
          ) : (
            <NodePickerTable rows={targetRows} selectedIds={selectedTargetIds} onToggle={toggleTarget} />
          )}
        </div>
        <footer>
          <span className="ume-connect-footer-note">
            {selectedSource
              ? t('umodelExplorer.dialog.connect.sourceWithName', { name: titleForElement(selectedSource) })
              : t('umodelExplorer.dialog.connect.selectSource')}
            {targetCount > 0
              ? ` · ${t(targetCount === 1 ? 'umodelExplorer.dialog.connect.targetsSelected' : 'umodelExplorer.dialog.connect.targetsSelectedOther', { count: targetCount })}`
              : ''}
          </span>
          <div>
            <button className="ume-secondary-inline" onClick={onClose} type="button">{t('common.cancel')}</button>
            <button className="ume-primary-button" disabled={!selectedSource || targetCount === 0} onClick={confirm} type="button">
              {targetCount > 0 ? t('umodelExplorer.action.createCount', { count: targetCount }) : t('common.create')}
            </button>
          </div>
        </footer>
      </section>
    </div>
  )
}

function DiffDialog({
  diff,
  serverElements,
  committing,
  submitFailure,
  onClose,
  onFocusElement,
  onSubmit,
}: {
  diff: DraftDiff
  serverElements: UModelElement[]
  committing: boolean
  submitFailure: SubmitFailure | null
  onClose: () => void
  onFocusElement: (element: UModelElement) => void
  onSubmit: () => void
}) {
  const { t } = useI18n()
  const [selectedIndex, setSelectedIndex] = useState(0)
  const serverById = useMemo(() => new Map(serverElements.map((element) => [elementKey(element), element])), [serverElements])
  const changes = [
    ...diff.added.map((element) => ({ type: 'added' as const, element, original: null })),
    ...diff.modified.map((element) => ({ type: 'modified' as const, element, original: serverById.get(elementKey(element)) || null })),
    ...diff.deleted.map((element) => ({ type: 'deleted' as const, element, original: element })),
  ]
  const selected = changes[selectedIndex] || changes[0]
  const originalJson = selected?.type === 'added' ? '' : stringify(selected?.original || {})
  const modifiedJson = selected?.type === 'deleted' ? '' : stringify(selected?.element || {})
  const selectedModelKey = selected ? `${selected.type}:${elementKey(selected.element)}` : 'empty'

  useEffect(() => {
    if (selectedIndex >= changes.length) setSelectedIndex(0)
  }, [changes.length, selectedIndex])

  return (
    <div className="ume-drawer-backdrop">
      <section className="ume-diff-drawer">
        <header>
          <div>
            <strong>{t('umodelExplorer.dialog.diff.title')}</strong>
            <span>{t('umodelExplorer.dialog.diff.description')}</span>
          </div>
          <button className="ume-icon-button subtle" disabled={committing} onClick={onClose} type="button">
            <X size={15} />
          </button>
        </header>
        <div className="ume-diff-summary">
          <span>{t('umodelExplorer.dialog.diff.added')} <strong>{diff.added.length}</strong></span>
          <span>{t('umodelExplorer.dialog.diff.modified')} <strong>{diff.modified.length}</strong></span>
          <span>{t('umodelExplorer.dialog.diff.deleted')} <strong>{diff.deleted.length}</strong></span>
        </div>
        <div className={`ume-submit-failure-slot ${submitFailure ? 'visible' : ''}`}>
          {submitFailure && (
            <div className="ume-submit-failure" role="alert">
              <strong>{submitFailure.summary}</strong>
              {submitFailure.details.length > 0 && (
                <ul>
                  {submitFailure.details.map((detail, index) => (
                    <li key={`${index}-${detail}`}>{detail}</li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </div>
        <div className="ume-dialog-body">
          <div className="ume-diff-list">
            {changes.map(({ type, element }, index) => (
              <div
                key={`${type}-${elementKey(element)}`}
                className={`ume-diff-row ${type} ${selectedIndex === index ? 'active' : ''}`}
                onClick={() => setSelectedIndex(index)}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault()
                    setSelectedIndex(index)
                  }
                }}
                role="button"
                tabIndex={0}
              >
                <span>{draftChangeTypeLabel(type, t)}</span>
                <strong>{titleForElement(element)}</strong>
                <code>{elementKey(element)}</code>
                {type !== 'deleted' ? (
                  <button
                    className="ume-diff-focus-action"
                    onClick={(event) => {
                      event.stopPropagation()
                      onFocusElement(element)
                      onClose()
                    }}
                    type="button"
                    title={t('umodelExplorer.action.focusInGraph')}
                  >
                    <Crosshair size={12} />
                  </button>
                ) : <em />}
              </div>
            ))}
            {changes.length === 0 && <div className="ume-empty-search">{t('umodelExplorer.empty.draftChanges')}</div>}
          </div>
          <div className="ume-diff-viewer">
            {selected ? (
              <>
                <div className="ume-diff-viewer-title">
                  <span>{draftChangeTypeLabel(selected.type, t)}</span>
                  <strong>{titleForElement(selected.element)}</strong>
                  <code>{elementKey(selected.element)}</code>
                </div>
                <SafeDiffEditor
                  modelKey={selectedModelKey}
                  original={originalJson}
                  modified={modifiedJson}
                />
              </>
            ) : (
              <div className="ume-empty-search">{t('umodelExplorer.empty.diffSelected')}</div>
            )}
          </div>
        </div>
        <footer>
          <button className="ume-secondary-inline" disabled={committing} onClick={onClose} type="button">{t('common.cancel')}</button>
          <button className="ume-primary-button" disabled={committing || changes.length === 0} onClick={onSubmit} type="button">
            <Save size={14} />
            {committing ? t('umodelExplorer.action.submitting') : t('umodelExplorer.action.confirmSubmit')}
          </button>
        </footer>
      </section>
    </div>
  )
}

function SafeDiffEditor({
  modelKey,
  original,
  modified,
}: {
  modelKey: string
  original: string
  modified: string
}) {
  const monaco = useMonaco()
  const monacoRef = useRef(monaco)
  const modelPathsRef = useRef<Set<string>>(new Set())
  const safeModelKey = encodeURIComponent(modelKey)
  const originalModelPath = `inmemory://umodel-diff/${safeModelKey}.original.json`
  const modifiedModelPath = `inmemory://umodel-diff/${safeModelKey}.modified.json`

  useEffect(() => {
    monacoRef.current = monaco
  }, [monaco])

  useEffect(() => {
    modelPathsRef.current.add(originalModelPath)
    modelPathsRef.current.add(modifiedModelPath)
  }, [modifiedModelPath, originalModelPath])

  useEffect(() => {
    return () => {
      const monacoInstance = monacoRef.current
      const modelPaths = Array.from(modelPathsRef.current)
      window.setTimeout(() => {
        for (const path of modelPaths) {
          const model = monacoInstance?.editor.getModel(monacoInstance.Uri.parse(path))
          model?.dispose()
        }
      }, 0)
    }
  }, [])

  return (
    <DiffEditor
      original={original}
      modified={modified}
      language="json"
      originalModelPath={originalModelPath}
      modifiedModelPath={modifiedModelPath}
      keepCurrentOriginalModel
      keepCurrentModifiedModel
      theme="vs"
      options={{
        automaticLayout: true,
        fontFamily: 'SF Mono, Fira Code, Consolas, monospace',
        fontSize: 12,
        minimap: { enabled: false },
        originalEditable: false,
        readOnly: true,
        renderMarginRevertIcon: false,
        renderSideBySide: true,
        scrollBeyondLastLine: false,
        wordWrap: 'on',
      }}
    />
  )
}

function rowToElement(row: Record<string, unknown>): UModelElement {
  const metadata = isObject(row.metadata) ? row.metadata : undefined
  const domain = optionalString(row.domain) || optionalString(metadata?.domain)
  const name = optionalString(row.name) || optionalString(metadata?.name)
  return {
    kind: String(row.kind || ''),
    domain: domain || '',
    name: name || '',
    version: optionalString(row.version) || optionalString(metadata?.version),
    spec: isObject(row.spec) ? row.spec : undefined,
  }
}

export function parseUModelElementsFromJson(json: string): UModelElement[] {
  return asArray(parseJson<UModelElement | UModelElement[]>(json, 'UModel elements'))
}

export function parseUModelElementsFromYamlOrJson(input: string, t?: TFunction): UModelElement[] {
  const text = input.trim()
  if (!text) return []
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch {
    parsed = YAML.load(text)
  }
  return normalizeUModelPayload(parsed, t)
}

function parseUploadPreview(input: string): UModelElement[] {
  try {
    return parseUModelElementsFromYamlOrJson(input)
  } catch {
    return []
  }
}

function normalizeUModelPayload(payload: unknown, t?: TFunction): UModelElement[] {
  if (Array.isArray(payload)) return payload.map((value) => normalizeUModelElement(value, t))
  if (isObject(payload) && Array.isArray(payload.elements)) return payload.elements.map((value) => normalizeUModelElement(value, t))
  if (isObject(payload) && Array.isArray(payload.items)) return payload.items.map((value) => normalizeUModelElement(value, t))
  if (isObject(payload) && Array.isArray(payload.rows)) return payload.rows.map((row) => normalizeUModelElement(row, t))
  if (isObject(payload)) return [normalizeUModelElement(payload, t)]
  throw new Error(t?.('umodelExplorer.validation.payloadMustContain') || 'YAML/JSON must contain one UModel element, an array, or an object with elements/items/rows.')
}

function normalizeUModelElement(value: unknown, t?: TFunction): UModelElement {
  if (!isObject(value)) throw new Error(t?.('umodelExplorer.validation.rowObjectRequired') || 'Each UModel element must be an object.')
  const element = rowToElement(value)
  if (!element.kind) throw new Error(t?.('umodelExplorer.validation.missingKind', { id: elementKey(element) || '<unknown>' }) || `Element ${elementKey(element) || '<unknown>'} is missing kind.`)
  if (!element.domain) throw new Error(t?.('umodelExplorer.validation.missingDomain', { id: element.name || '<unknown>' }) || `Element ${element.name || '<unknown>'} is missing domain.`)
  if (!element.name) throw new Error(t?.('umodelExplorer.validation.missingName', { id: element.domain || '<unknown>' }) || `Element ${element.domain || '<unknown>'} is missing name.`)
  return element
}

function formatValidationErrorLines(errors: Array<{ field?: string; reason?: string }> | undefined, t: TFunction) {
  return (errors || [])
    .map((item) => `${item.field || t('umodelExplorer.validation.elementLabel')}: ${item.reason || t('umodelExplorer.validation.failed')}`)
}

function formatValidationErrors(errors: Array<{ field?: string; reason?: string }> | undefined, t: TFunction) {
  return formatValidationErrorLines(errors, t).join('\n') || t('umodelExplorer.validation.failed')
}

function buildSubmitFailure(summary: string, details: string[]): SubmitFailure {
  return { summary, details: details.filter(Boolean) }
}

function formatWriteFailure(result: WriteResult, operation: string, t: TFunction): SubmitFailure | null {
  if (result.failed <= 0) return null
  const failedItems = (result.items || []).filter((item) => !item.ok)
  const visibleItems = failedItems.slice(0, 8)
  const details = visibleItems.map((item) => formatBatchFailure(item, t))
  const hiddenCount = failedItems.length - visibleItems.length
  if (hiddenCount > 0) {
    details.push(t('umodelExplorer.submitFailure.moreItems', { count: hiddenCount }))
  }
  if (details.length === 0) {
    details.push(t('umodelExplorer.submitFailure.noItemDetails'))
  }
  return {
    summary: t('umodelExplorer.submitFailure.writeResult', {
      operation,
      accepted: result.accepted,
      failed: result.failed,
    }),
    details,
  }
}

function formatBatchFailure(item: BatchItemResult, t: TFunction) {
  const id = item.id || t('umodelExplorer.submitFailure.unknownItem')
  const code = item.code ? ` [${item.code}]` : ''
  const message = item.message || t('umodelExplorer.submitFailure.noMessage')
  return `${id}${code}: ${message}`
}

type DraftChangeType = 'added' | 'modified' | 'deleted'

function draftChangeTypeLabel(type: DraftChangeType, t: TFunction) {
  if (type === 'added') return t('umodelExplorer.dialog.diff.type.added')
  if (type === 'modified') return t('umodelExplorer.dialog.diff.type.modified')
  return t('umodelExplorer.dialog.diff.type.deleted')
}
