import { useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Boxes, Check, Globe2, Info, List, Search, X } from 'lucide-react'
import type { UModelElement } from '../../api/types'
import { useI18n } from '../../i18n'
import {
  colorForKind,
  countEntries,
  diversifySearchEntries,
  escapeRegExp,
  kindActsAsGraphNode,
  kindRank,
  searchIndexSearch,
  splitWords,
  toggleValue,
  type EntitySetLinkDisplay,
  type SearchEntry,
  type SearchIndex,
  type ViewMode,
} from './model'
import { iconForKind } from './kindIcon'

export function SearchPanel({
  index,
  query,
  currentView,
  entitySetLinkDisplay,
  style,
  onApplyDomain,
  onApplyFullText,
  onApplyKind,
  onClose,
  onFocusElement,
}: {
  index: SearchIndex
  query: string
  currentView: ViewMode
  entitySetLinkDisplay: EntitySetLinkDisplay
  style?: CSSProperties
  onApplyDomain: (domain: string) => void
  onApplyFullText: (text: string) => void
  onApplyKind: (kind: string) => void
  onClose: () => void
  onFocusElement: (element: UModelElement) => void
}) {
  const { t } = useI18n()
  const [resultKindFilter, setResultKindFilter] = useState<string[]>([])
  const hasQuery = query.trim().length > 0
  const allResults = hasQuery ? searchIndexSearch(index, query, 200) : []
  const resultKindCounts = countEntries(allResults, (entry) => entry.element.kind)
  const resultKinds = [...resultKindCounts.entries()]
    .sort((left, right) => kindRank(left[0]) - kindRank(right[0]))
  const filteredResults = resultKindFilter.length === 0
    ? allResults
    : allResults.filter((entry) => resultKindFilter.includes(entry.element.kind))
  const visibleResults = diversifySearchEntries(filteredResults, 12)
  const kindCounts = countEntries(index.entries, (entry) => entry.element.kind)
  const domainCounts = countEntries(index.entries, (entry) => entry.element.domain || 'unknown')
  const orderedKinds = [...kindCounts.entries()]
    .filter(([kind]) => currentView !== 'graph' || kindActsAsGraphNode(kind, entitySetLinkDisplay))
    .sort((left, right) => kindRank(left[0]) - kindRank(right[0]))
    .slice(0, 10)
  const orderedDomains = [...domainCounts.entries()].sort((left, right) => right[1] - left[1]).slice(0, 8)

  useEffect(() => {
    setResultKindFilter([])
  }, [query])

  const toggleResultKind = (kind: string) => {
    setResultKindFilter((items) => toggleValue(items, kind))
  }

  const panel = (
    <div className="ume-search-panel" style={style} onMouseDown={(event) => event.preventDefault()}>
      {hasQuery && (
        <SearchSection title={t('umodelExplorer.search.exactFullTextFilter')} icon={<Search size={13} />}>
          <button
            className="ume-search-fulltext"
            onClick={() => {
              onApplyFullText(query)
              onClose()
            }}
            type="button"
          >
            <Search size={13} />
            <span>{query.trim()}</span>
            <strong>Enter</strong>
          </button>
        </SearchSection>
      )}

      {hasQuery && allResults.length > 0 && (
        <SearchSection
          title={t('umodelExplorer.search.matchedResults')}
          icon={<List size={13} />}
          extra={
            <div className="ume-search-kind-filter">
              <button className={resultKindFilter.length === 0 ? 'active' : ''} onClick={() => setResultKindFilter([])} type="button">
                {t('umodelExplorer.search.all')}
              </button>
              {resultKinds.map(([kind, count]) => {
                const color = colorForKind(kind)
                const active = resultKindFilter.includes(kind)
                return (
                  <button
                    key={kind}
                    className={active ? 'active' : ''}
                    onClick={() => toggleResultKind(kind)}
                    style={{
                      '--chip-color': color.dot,
                      '--chip-bg': color.bg,
                      '--chip-text': color.text,
                    } as CSSProperties}
                    type="button"
                  >
                    <span className="ume-search-kind-icon" style={{ color: color.dot }}>
                      {iconForKind(kind)}
                    </span>
                    {color.label}
                    <strong>{count}</strong>
                    {active && <Check size={9} />}
                  </button>
                )
              })}
              {resultKindFilter.length > 0 && (
                <button className="clear" onClick={() => setResultKindFilter([])} type="button">
                  <X size={9} />
                  {t('umodelExplorer.action.clear')}
                </button>
              )}
            </div>
          }
        >
          <div className="ume-search-results">
            {visibleResults.map((entry) => (
              <SearchResultButton
                key={entry.id}
                entry={entry}
                query={query}
                onClick={() => {
                  onFocusElement(entry.element)
                  onClose()
                }}
              />
            ))}
            {visibleResults.length === 0 && (
              <span className="ume-empty-search">{t('umodelExplorer.empty.matchingResultsForType')}</span>
            )}
          </div>
        </SearchSection>
      )}

      {hasQuery && allResults.length === 0 && (
        <SearchSection title={t('umodelExplorer.search.matchedResults')} icon={<List size={13} />}>
          <span className="ume-empty-search">{t('umodelExplorer.empty.matchingElementsFilter')}</span>
        </SearchSection>
      )}

      {!hasQuery && (
        <>
          <SearchSection title={t('umodelExplorer.search.byType')} icon={<Boxes size={13} />}>
            <div className="ume-search-hints">
              {orderedKinds.map(([kind, count]) => {
                const color = colorForKind(kind)
                return (
                  <button
                    key={kind}
                    className="ume-search-type-chip"
                    onClick={() => { onApplyKind(kind); onClose() }}
                    style={{ '--chip-color': color.dot, '--chip-bg': color.bg, '--chip-text': color.text } as CSSProperties}
                    type="button"
                  >
                    <span className="ume-search-kind-icon" style={{ color: color.dot }}>
                      {iconForKind(kind)}
                    </span>
                    {color.label}
                    <strong>{count}</strong>
                  </button>
                )
              })}
            </div>
          </SearchSection>
          <SearchSection title={t('umodelExplorer.search.byDomain')} icon={<Globe2 size={13} />}>
            <div className="ume-search-hints">
              {orderedDomains.map(([domain, count]) => (
                <button className="ume-search-domain-chip" key={domain} onClick={() => { onApplyDomain(domain); onClose() }} type="button">
                  {domain === 'unknown' ? t('umodelExplorer.misc.unknown') : domain}
                  <strong>{count}</strong>
                </button>
              ))}
            </div>
          </SearchSection>
          <div className="ume-search-footer">
            <Info size={13} />
            {t('umodelExplorer.search.footer')}
          </div>
        </>
      )}
    </div>
  )
  const portalTarget = typeof document === 'undefined' ? null : document.querySelector('.umodel-page')
  return portalTarget ? createPortal(panel, portalTarget) : panel
}

function SearchSection({
  title,
  icon,
  extra,
  children,
}: {
  title: string
  icon: ReactNode
  extra?: ReactNode
  children: ReactNode
}) {
  return (
    <div className="ume-search-panel-section">
      <div className="ume-search-panel-title">
        {icon}
        <span>{title}</span>
        {extra && <span className="ume-search-panel-extra">{extra}</span>}
      </div>
      {children}
    </div>
  )
}

function SearchResultButton({
  entry,
  query,
  onClick,
}: {
  entry: SearchEntry
  query: string
  onClick: () => void
}) {
  const color = colorForKind(entry.element.kind)
  return (
    <button onClick={onClick} type="button">
      <span className="ume-search-kind-icon" style={{ background: color.bg, color: color.text }}>
        {iconForKind(entry.element.kind)}
      </span>
      <strong><HighlightText text={entry.title} query={query} /></strong>
      <code><HighlightText text={entry.subtitle} query={query} /></code>
    </button>
  )
}

function HighlightText({ text, query }: { text: string; query: string }) {
  const needles = [...new Set(splitWords(query))]
    .filter((needle) => needle.length > 0)
    .slice(0, 8)
  if (needles.length === 0) return <>{text}</>
  const matcher = new RegExp(`(${needles.map(escapeRegExp).join('|')})`, 'ig')
  return (
    <>
      {text.split(matcher).map((part, index) => {
        const isMatch = needles.includes(part.toLowerCase())
        return isMatch ? <mark key={`${part}-${index}`}>{part}</mark> : part
      })}
    </>
  )
}
