import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useTranslation } from '../i18n/LanguageContext'
import type { TranslationKey } from '../i18n/translations'

type Job = components['schemas']['dto.DiscoveryJobResponse']
type JobResult = components['schemas']['dto.DiscoveryJobResultResponse']

function extractErrorMessage(body: unknown): string {
  if (body && typeof body === 'object' && 'error' in body) {
    const err = (body as { error?: { message?: string } }).error
    if (err?.message) return err.message
  }
  return 'Request failed'
}

function statusLabel(t: (k: TranslationKey) => string, status: string): string {
  const key = `status.${status}` as TranslationKey
  try {
    return t(key)
  } catch {
    return status
  }
}

function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  const cls =
    status === 'completed'
      ? 'badge-success'
      : status === 'failed' || status === 'cancelled'
        ? 'badge-danger'
        : status === 'running'
          ? 'badge-warning'
          : 'badge-neutral'
  return <span className={`badge ${cls}`}>{statusLabel(t, status)}</span>
}

function ResultStatusBadge({ status }: { status: string }) {
  const { t } = useTranslation()
  const cls = status === 'success' ? 'badge-success' : status === 'no_response' ? 'badge-neutral' : 'badge-danger'
  return <span className={`badge ${cls}`}>{statusLabel(t, status)}</span>
}

export default function DiscoveryPage() {
  const { t } = useTranslation()
  const [targets, setTargets] = useState('')
  const [snmpVersion, setSnmpVersion] = useState<'v2c' | 'v1'>('v2c')
  const [community, setCommunity] = useState('public')
  const [activeJobId, setActiveJobId] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const startScan = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await api.POST('/discovery/jobs', {
        body: { targets, snmp_version: snmpVersion, snmp_community: community },
      })
      if (error) throw new Error(extractErrorMessage(error))
      if (!response.ok || !data) throw new Error('Request failed')
      return data
    },
    onSuccess: (job) => {
      setActiveJobId(job.id ?? null)
      queryClient.invalidateQueries({ queryKey: ['discovery-jobs'] })
    },
  })

  const jobQuery = useQuery({
    queryKey: ['discovery-job', activeJobId],
    queryFn: async () => {
      const { data } = await api.GET('/discovery/jobs/{jobId}', { params: { path: { jobId: activeJobId! } } })
      return data as Job
    },
    enabled: !!activeJobId,
    refetchInterval: (query) => (query.state.data?.status === 'running' || query.state.data?.status === 'pending' ? 2000 : false),
  })

  const resultsQuery = useQuery({
    queryKey: ['discovery-job-results', activeJobId],
    queryFn: async () => {
      const { data } = await api.GET('/discovery/jobs/{jobId}/results', { params: { path: { jobId: activeJobId! } } })
      return (data ?? []) as JobResult[]
    },
    enabled: !!activeJobId && jobQuery.data?.status === 'completed',
  })

  const jobsListQuery = useQuery({
    queryKey: ['discovery-jobs'],
    queryFn: async () => {
      const { data } = await api.GET('/discovery/jobs')
      return (data ?? []) as Job[]
    },
  })

  const job = jobQuery.data

  return (
    <div className="stack">
      <h1>{t('discovery.title')}</h1>

      <form
        className="card stack"
        onSubmit={(e) => {
          e.preventDefault()
          startScan.mutate()
        }}
      >
        <div className="field">
          <label htmlFor="targets">{t('discovery.targetsLabel')}</label>
          <input
            id="targets"
            placeholder={t('discovery.targetsPlaceholder')}
            value={targets}
            onChange={(e) => setTargets(e.target.value)}
            required
          />
        </div>
        <div className="row">
          <div className="field">
            <label htmlFor="version">{t('discovery.versionLabel')}</label>
            <select id="version" value={snmpVersion} onChange={(e) => setSnmpVersion(e.target.value as 'v2c' | 'v1')}>
              <option value="v2c">v2c</option>
              <option value="v1">v1</option>
            </select>
          </div>
          <div className="field">
            <label htmlFor="community">{t('discovery.communityLabel')}</label>
            <input id="community" value={community} onChange={(e) => setCommunity(e.target.value)} required />
          </div>
        </div>
        <div className="row">
          <button type="submit" disabled={startScan.isPending}>
            {startScan.isPending ? t('discovery.starting') : t('discovery.startScan')}
          </button>
          {startScan.isError && <span className="error-text">{startScan.error.message}</span>}
        </div>
      </form>

      {job && (
        <div className="card stack">
          <div className="row">
            <h2 style={{ margin: 0 }}>Job {job.id?.slice(0, 8)}</h2>
            <StatusBadge status={job.status ?? 'unknown'} />
            <span className="muted">
              {job.total_targets} {t('discovery.targetsSuffix')}
            </span>
          </div>
          {(job.status === 'pending' || job.status === 'running') && (
            <button
              className="secondary"
              onClick={async () => {
                await api.POST('/discovery/jobs/{jobId}/cancel', { params: { path: { jobId: job.id! } } })
                jobQuery.refetch()
              }}
            >
              {t('discovery.cancelJob')}
            </button>
          )}
          {job.error_message && <p className="error-text">{job.error_message}</p>}

          {resultsQuery.data && resultsQuery.data.length > 0 && (
            <table>
              <thead>
                <tr>
                  <th>{t('discovery.colIP')}</th>
                  <th>{t('common.status')}</th>
                  <th>{t('discovery.colSysName')}</th>
                  <th>{t('discovery.colError')}</th>
                </tr>
              </thead>
              <tbody>
                {resultsQuery.data.map((r) => (
                  <tr key={r.ip_address}>
                    <td>{r.ip_address}</td>
                    <td>
                      <ResultStatusBadge status={r.status ?? ''} />
                    </td>
                    <td>{r.sys_name ?? '—'}</td>
                    <td className="muted">{r.error_message ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      <div className="card stack">
        <h2>{t('discovery.recentJobs')}</h2>
        <table>
          <thead>
            <tr>
              <th>{t('discovery.colInput')}</th>
              <th>{t('common.status')}</th>
              <th>{t('discovery.colTargets')}</th>
              <th>{t('discovery.colCreated')}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {jobsListQuery.data?.map((j) => (
              <tr key={j.id}>
                <td>{j.input_spec}</td>
                <td>
                  <StatusBadge status={j.status ?? 'unknown'} />
                </td>
                <td>{j.total_targets}</td>
                <td className="muted">{j.created_at ? new Date(j.created_at).toLocaleString() : ''}</td>
                <td>
                  <button className="secondary" onClick={() => setActiveJobId(j.id ?? null)}>
                    {t('discovery.view')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
