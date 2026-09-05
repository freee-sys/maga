import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api/client'
import type { CaptureTransforms, DslStep } from '../api/dsl'
import DslPipelineEditor from '../components/DslPipelineEditor'
import { useTranslation } from '../i18n/LanguageContext'

function extractGroupNames(pattern: string): string[] {
  const names: string[] = []
  const re = /\(\?P<([a-zA-Z_]\w*)>/g
  let m: RegExpExecArray | null
  while ((m = re.exec(pattern))) names.push(m[1])
  return names
}

function extractTemplateTokens(tmpl: string): string[] {
  const tokens: string[] = []
  const re = /\{(\w+)\}/g
  let m: RegExpExecArray | null
  while ((m = re.exec(tmpl))) tokens.push(m[1])
  return tokens
}

interface TestResult {
  matched?: boolean
  raw_captures?: Record<string, string>
  transformed_captures?: Record<string, string>
  rendered_cluster_key?: string
  would_create_new_cluster?: boolean
  warnings?: string[]
}

export default function RuleEditorPage() {
  const { t } = useTranslation()
  const { id } = useParams()
  const isNew = !id
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [pattern, setPattern] = useState('')
  const [clusterKeyTemplate, setClusterKeyTemplate] = useState('')
  const [priority, setPriority] = useState(10)
  const [isEnabled, setIsEnabled] = useState(true)
  const [transforms, setTransforms] = useState<CaptureTransforms>({})
  const [sampleHostname, setSampleHostname] = useState('')
  const [saveError, setSaveError] = useState<string | null>(null)

  const existing = useQuery({
    queryKey: ['rule', id],
    queryFn: async () => {
      const { data } = await api.GET('/rules/{id}', { params: { path: { id: id! } } })
      return data
    },
    enabled: !isNew,
  })

  useEffect(() => {
    if (existing.data) {
      setName(existing.data.name ?? '')
      setDescription(existing.data.description ?? '')
      setPattern(existing.data.pattern ?? '')
      setClusterKeyTemplate(existing.data.cluster_key_template ?? '')
      setPriority(existing.data.priority ?? 10)
      setIsEnabled(existing.data.is_enabled ?? true)
      setTransforms((existing.data.capture_transforms as unknown as CaptureTransforms) ?? {})
    }
  }, [existing.data])

  const groupNames = useMemo(() => extractGroupNames(pattern), [pattern])
  const templateTokens = useMemo(() => extractTemplateTokens(clusterKeyTemplate), [clusterKeyTemplate])
  const undefinedTokens = templateTokens.filter((tok) => !groupNames.includes(tok))

  const testMutation = useMutation({
    mutationFn: async () => {
      const { data, error } = await api.POST('/rules/test', {
        body: {
          hostname: sampleHostname,
          pattern,
          // Real wire format uses named args (see api/dsl.ts); the
          // generated request type is wrong for the same reason, so this
          // cast is intentional, not a shortcut.
          capture_transforms: transforms as unknown as never,
          cluster_key_template: clusterKeyTemplate,
        },
      })
      if (error) throw new Error(JSON.stringify(error))
      return data as TestResult
    },
  })

  const saveMutation = useMutation({
    mutationFn: async () => {
      const body = {
        name,
        description,
        pattern,
        capture_transforms: transforms as unknown as never,
        cluster_key_template: clusterKeyTemplate,
        priority,
        is_enabled: isEnabled,
      }
      const { error } = isNew
        ? await api.POST('/rules', { body })
        : await api.PUT('/rules/{id}', { params: { path: { id: id! } }, body })
      if (error) {
        const msg = (error as { error?: { message?: string; fields?: { field: string; message: string }[] } }).error
        const detail = msg?.fields?.map((f) => `${f.field}: ${f.message}`).join('; ')
        throw new Error(detail || msg?.message || t('ruleEditor.saveFailed'))
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['rules'] })
      navigate('/rules')
    },
    onError: (e: Error) => setSaveError(e.message),
  })

  return (
    <div className="stack">
      <h1>{isNew ? t('ruleEditor.newTitle') : t('ruleEditor.editTitle')}</h1>

      <div className="card stack">
        <div className="field">
          <label>{t('ruleEditor.nameLabel')}</label>
          <input value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="field">
          <label>{t('ruleEditor.descriptionLabel')}</label>
          <input value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div className="field">
          <label>{t('ruleEditor.patternLabel')}</label>
          <input
            style={{ fontFamily: 'monospace' }}
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            placeholder="^sw-(?P<site>[a-z0-9]+)-core-\d+$"
          />
          {groupNames.length > 0 && (
            <span className="muted">
              {t('ruleEditor.groupsLabel')}: {groupNames.join(', ')}
            </span>
          )}
        </div>

        {groupNames.map((g) => (
          <div key={g} className="field">
            <label>
              {t('ruleEditor.transformFor')} "{g}"
            </label>
            <DslPipelineEditor
              steps={transforms[g] ?? []}
              onChange={(steps: DslStep[]) => setTransforms({ ...transforms, [g]: steps })}
            />
          </div>
        ))}

        <div className="field">
          <label>{t('ruleEditor.clusterKeyTemplateLabel')}</label>
          <input value={clusterKeyTemplate} onChange={(e) => setClusterKeyTemplate(e.target.value)} />
          {undefinedTokens.length > 0 && (
            <span className="error-text">
              {t('ruleEditor.undefinedGroups')}: {undefinedTokens.join(', ')}
            </span>
          )}
        </div>

        <div className="row">
          <div className="field">
            <label>{t('ruleEditor.priorityLabel')}</label>
            <input type="number" value={priority} onChange={(e) => setPriority(Number(e.target.value))} style={{ width: '5rem' }} />
          </div>
          <label className="row" style={{ marginTop: '1.2rem' }}>
            <input type="checkbox" checked={isEnabled} onChange={(e) => setIsEnabled(e.target.checked)} />
            {t('ruleEditor.enabledLabel')}
          </label>
        </div>
      </div>

      <div className="card stack">
        <h2>{t('ruleEditor.testTitle')}</h2>
        <div className="row">
          <input
            placeholder={t('ruleEditor.sampleHostnamePlaceholder')}
            value={sampleHostname}
            onChange={(e) => setSampleHostname(e.target.value)}
          />
          <button className="secondary" onClick={() => testMutation.mutate()} disabled={testMutation.isPending || !pattern}>
            {t('ruleEditor.test')}
          </button>
        </div>
        {testMutation.isError && <p className="error-text">{(testMutation.error as Error).message}</p>}
        {testMutation.data && (
          <div className="stack">
            <p>
              {t('ruleEditor.matched')}: <strong>{String(testMutation.data.matched)}</strong>
            </p>
            {testMutation.data.raw_captures && (
              <p>
                {t('ruleEditor.raw')}: {JSON.stringify(testMutation.data.raw_captures)}
              </p>
            )}
            {testMutation.data.transformed_captures && (
              <p>
                {t('ruleEditor.transformed')}: {JSON.stringify(testMutation.data.transformed_captures)}
              </p>
            )}
            <p>
              {t('ruleEditor.clusterKey')}: <strong>{testMutation.data.rendered_cluster_key || t('ruleEditor.emptyValue')}</strong>{' '}
              {testMutation.data.would_create_new_cluster && (
                <span className="badge badge-warning">{t('ruleEditor.newCluster')}</span>
              )}
            </p>
            {testMutation.data.warnings?.map((w, i) => (
              <p key={i} className="error-text">
                {w}
              </p>
            ))}
          </div>
        )}
      </div>

      <div className="row">
        <button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending}>
          {t('common.save')}
        </button>
        <button className="secondary" onClick={() => navigate('/rules')}>
          {t('common.cancel')}
        </button>
        {saveError && <span className="error-text">{saveError}</span>}
      </div>
    </div>
  )
}
