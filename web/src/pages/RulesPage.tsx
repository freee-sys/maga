import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useTranslation } from '../i18n/LanguageContext'

type Rule = components['schemas']['dto.RuleResponse']

export default function RulesPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const rulesQuery = useQuery({
    queryKey: ['rules'],
    queryFn: async () => {
      const { data } = await api.GET('/rules')
      return (data ?? []) as Rule[]
    },
  })

  const reorder = useMutation({
    mutationFn: async (ruleIds: string[]) => {
      await api.POST('/rules/reorder', { body: { rule_ids: ruleIds } })
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rules'] }),
  })

  const deleteRule = useMutation({
    mutationFn: async (id: string) => {
      await api.DELETE('/rules/{id}', { params: { path: { id } } })
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['rules'] }),
  })

  const rules = rulesQuery.data ?? []

  const move = (index: number, dir: -1 | 1) => {
    const j = index + dir
    if (j < 0 || j >= rules.length) return
    const ids = rules.map((r) => r.id!)
    ;[ids[index], ids[j]] = [ids[j], ids[index]]
    reorder.mutate(ids)
  }

  return (
    <div className="stack">
      <div className="row" style={{ justifyContent: 'space-between' }}>
        <h1>{t('rules.title')}</h1>
        <button onClick={() => navigate('/rules/new')}>{t('rules.newRule')}</button>
      </div>

      <div className="card">
        <table>
          <thead>
            <tr>
              <th>{t('rules.colPriority')}</th>
              <th>{t('rules.colName')}</th>
              <th>{t('rules.colPattern')}</th>
              <th>{t('rules.colClusterKey')}</th>
              <th>{t('rules.colEnabled')}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rules.map((r, i) => (
              <tr key={r.id}>
                <td className="row">
                  <button className="secondary" onClick={() => move(i, -1)} disabled={i === 0}>
                    ↑
                  </button>
                  <button className="secondary" onClick={() => move(i, 1)} disabled={i === rules.length - 1}>
                    ↓
                  </button>
                  {r.priority}
                </td>
                <td>
                  <Link to={`/rules/${r.id}`}>{r.name}</Link>
                </td>
                <td className="muted" style={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                  {r.pattern}
                </td>
                <td>{r.cluster_key_template}</td>
                <td>
                  <span className={`badge ${r.is_enabled ? 'badge-success' : 'badge-neutral'}`}>
                    {r.is_enabled ? t('rules.enabled') : t('rules.disabled')}
                  </span>
                </td>
                <td>
                  <button
                    className="danger"
                    onClick={() => confirm(`${t('rules.deleteConfirm')} "${r.name}"?`) && deleteRule.mutate(r.id!)}
                  >
                    {t('common.delete')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {rules.length === 0 && <p className="muted">{t('rules.empty')}</p>}
      </div>
    </div>
  )
}
