import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useTranslation } from '../i18n/LanguageContext'
import type { TranslationKey } from '../i18n/translations'

type Element = components['schemas']['dto.ElementResponse']
type HistoryRow = components['schemas']['dto.AssignmentHistoryResponse']

function safeT(t: (k: TranslationKey) => string, key: string, fallback: string): string {
  try {
    return t(key as TranslationKey)
  } catch {
    return fallback
  }
}

function AssignmentBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  return (
    <span className={`badge ${status === 'assigned' ? 'badge-success' : 'badge-neutral'}`}>
      {safeT(t, `status.${status}`, status)}
    </span>
  )
}

function HistoryTimeline({ elementId }: { elementId: string }) {
  const { t } = useTranslation()
  const { data, isLoading } = useQuery({
    queryKey: ['element-history', elementId],
    queryFn: async () => {
      const { data } = await api.GET('/elements/{id}/history', { params: { path: { id: elementId } } })
      return (data ?? []) as HistoryRow[]
    },
  })

  if (isLoading) return <p className="muted">{t('elements.historyLoading')}</p>
  if (!data || data.length === 0) return <p className="muted">{t('elements.historyEmpty')}</p>

  return (
    <table>
      <thead>
        <tr>
          <th>{t('elements.colWhen')}</th>
          <th>{t('elements.colTrigger')}</th>
          <th>{t('elements.colClusterKey')}</th>
          <th>{t('elements.colWarnings')}</th>
        </tr>
      </thead>
      <tbody>
        {data.map((h) => (
          <tr key={h.id}>
            <td className="muted">{h.created_at ? new Date(h.created_at).toLocaleString() : ''}</td>
            <td>{safeT(t, `trigger.${h.trigger_source}`, h.trigger_source ?? '')}</td>
            <td>{h.rendered_cluster_key || <span className="muted">{t('common.unassigned')}</span>}</td>
            <td className="error-text">{h.warnings ?? ''}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export default function ElementsPage() {
  const { t } = useTranslation()
  const [statusFilter, setStatusFilter] = useState('')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [expanded, setExpanded] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const elementsQuery = useQuery({
    queryKey: ['elements', statusFilter, query],
    queryFn: async () => {
      const { data } = await api.GET('/elements', {
        params: { query: { assignment_status: statusFilter || undefined, q: query || undefined } },
      })
      return (data ?? []) as Element[]
    },
  })

  const redistributeOne = useMutation({
    mutationFn: async (id: string) => {
      await api.POST('/elements/{id}/redistribute', { params: { path: { id } } })
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['elements'] }),
  })

  const redistributeBulk = useMutation({
    mutationFn: async () => {
      await api.POST('/elements/redistribute', {
        body: selected.size > 0 ? { element_ids: Array.from(selected) } : {},
      })
    },
    onSuccess: () => {
      setSelected(new Set())
      queryClient.invalidateQueries({ queryKey: ['elements'] })
    },
  })

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <div className="stack">
      <h1>{t('elements.title')}</h1>

      <div className="row">
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
          <option value="">{t('elements.allStatuses')}</option>
          <option value="assigned">{t('status.assigned')}</option>
          <option value="unassigned">{t('common.unassigned')}</option>
        </select>
        <input placeholder={t('elements.searchPlaceholder')} value={query} onChange={(e) => setQuery(e.target.value)} />
        <button
          className="secondary"
          onClick={() => redistributeBulk.mutate()}
          disabled={redistributeBulk.isPending}
        >
          {selected.size > 0 ? `${t('elements.redistributeSelected')} (${selected.size})` : t('elements.redistributeAll')}
        </button>
      </div>

      <div className="card">
        <table>
          <thead>
            <tr>
              <th></th>
              <th>{t('elements.colIP')}</th>
              <th>{t('elements.colSysName')}</th>
              <th>{t('common.status')}</th>
              <th>{t('elements.colLastSeen')}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {elementsQuery.data?.map((el) => (
              <>
                <tr key={el.id}>
                  <td>
                    <input
                      type="checkbox"
                      checked={selected.has(el.id!)}
                      onChange={() => toggle(el.id!)}
                    />
                  </td>
                  <td>{el.ip_address}</td>
                  <td>{el.sys_name || <span className="muted">—</span>}</td>
                  <td>
                    <AssignmentBadge status={el.assignment_status ?? 'unassigned'} />
                  </td>
                  <td className="muted">{el.last_seen_at ? new Date(el.last_seen_at).toLocaleString() : ''}</td>
                  <td className="row">
                    <button className="secondary" onClick={() => setExpanded(expanded === el.id ? null : el.id!)}>
                      {t('elements.history')}
                    </button>
                    <button
                      className="secondary"
                      onClick={() => redistributeOne.mutate(el.id!)}
                      disabled={redistributeOne.isPending}
                    >
                      {t('elements.redistribute')}
                    </button>
                  </td>
                </tr>
                {expanded === el.id && (
                  <tr>
                    <td colSpan={6}>
                      <HistoryTimeline elementId={el.id!} />
                    </td>
                  </tr>
                )}
              </>
            ))}
          </tbody>
        </table>
        {elementsQuery.data?.length === 0 && <p className="muted">{t('elements.empty')}</p>}
      </div>
    </div>
  )
}
