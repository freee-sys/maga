import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { components } from '../api/schema'
import { useTranslation } from '../i18n/LanguageContext'

type Cluster = components['schemas']['dto.ClusterResponse']

function RenameRow({ cluster, onDone }: { cluster: Cluster; onDone: () => void }) {
  const { t } = useTranslation()
  const [name, setName] = useState(cluster.display_name ?? '')
  const queryClient = useQueryClient()

  const rename = useMutation({
    mutationFn: async () => {
      await api.PATCH('/clusters/{id}', {
        params: { path: { id: cluster.id! } },
        body: { display_name: name },
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      onDone()
    },
  })

  return (
    <span className="row">
      <input value={name} onChange={(e) => setName(e.target.value)} autoFocus />
      <button onClick={() => rename.mutate()} disabled={rename.isPending}>
        {t('common.save')}
      </button>
      <button className="secondary" onClick={onDone}>
        {t('common.cancel')}
      </button>
    </span>
  )
}

export default function ClustersPage() {
  const { t } = useTranslation()
  const [editingId, setEditingId] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const clustersQuery = useQuery({
    queryKey: ['clusters'],
    queryFn: async () => {
      const { data } = await api.GET('/clusters')
      return (data ?? []) as Cluster[]
    },
  })

  const deleteCluster = useMutation({
    mutationFn: async (id: string) => {
      const { error, response } = await api.DELETE('/clusters/{id}', { params: { path: { id } } })
      if (error && response.status === 409) throw new Error(t('clusters.deleteFailedNotEmpty'))
      if (error) throw new Error(t('clusters.deleteFailed'))
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['clusters'] }),
  })

  return (
    <div className="stack">
      <h1>{t('clusters.title')}</h1>
      <div className="card">
        <table>
          <thead>
            <tr>
              <th>{t('clusters.colDisplayName')}</th>
              <th>{t('clusters.colLookupKey')}</th>
              <th>{t('clusters.colMembers')}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {clustersQuery.data?.map((c) => (
              <tr key={c.id}>
                <td>{editingId === c.id ? <RenameRow cluster={c} onDone={() => setEditingId(null)} /> : c.display_name}</td>
                <td className="muted">{c.lookup_key}</td>
                <td>{c.member_count}</td>
                <td className="row">
                  {editingId !== c.id && (
                    <>
                      <button className="secondary" onClick={() => setEditingId(c.id!)}>
                        {t('clusters.rename')}
                      </button>
                      <button
                        className="danger"
                        disabled={!!c.member_count || deleteCluster.isPending}
                        title={c.member_count ? t('clusters.deleteTitle') : undefined}
                        onClick={() => deleteCluster.mutate(c.id!)}
                      >
                        {t('common.delete')}
                      </button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {clustersQuery.data?.length === 0 && <p className="muted">{t('clusters.empty')}</p>}
        {deleteCluster.isError && <p className="error-text">{deleteCluster.error.message}</p>}
      </div>
    </div>
  )
}
