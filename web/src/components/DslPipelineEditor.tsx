import { DSL_OPS, opSpec, type ArgSpec, type DslArgValue, type DslStep } from '../api/dsl'
import { useTranslation } from '../i18n/LanguageContext'

function defaultArgValue(spec: ArgSpec): DslArgValue {
  if (spec.kind === 'int') return 0
  if (spec.kind === 'map') return {}
  return ''
}

function ArgInput({
  spec,
  value,
  onChange,
}: {
  spec: ArgSpec
  value: DslArgValue
  onChange: (v: DslArgValue) => void
}) {
  if (spec.kind === 'int') {
    return (
      <input
        type="number"
        value={typeof value === 'number' ? value : 0}
        onChange={(e) => onChange(Number(e.target.value))}
        placeholder={spec.name}
        style={{ width: '5rem' }}
      />
    )
  }
  if (spec.kind === 'map') {
    // Simple "k1=v1,k2=v2" text form for the lookup table — enough for
    // Phase 1; a dedicated key/value row editor can replace this later.
    const table = typeof value === 'object' ? value : {}
    const text = Object.entries(table)
      .map(([k, v]) => `${k}=${v}`)
      .join(',')
    return (
      <input
        value={text}
        onChange={(e) => {
          const next: Record<string, string> = {}
          for (const pair of e.target.value.split(',')) {
            const [k, v] = pair.split('=')
            if (k) next[k.trim()] = (v ?? '').trim()
          }
          onChange(next)
        }}
        placeholder="dc=datacenter,br=branch"
      />
    )
  }
  return (
    <input
      value={typeof value === 'string' ? value : ''}
      onChange={(e) => onChange(e.target.value)}
      placeholder={spec.name}
    />
  )
}

export default function DslPipelineEditor({
  steps,
  onChange,
}: {
  steps: DslStep[]
  onChange: (steps: DslStep[]) => void
}) {
  const { t } = useTranslation()
  const addStep = (op: string) => {
    const spec = opSpec(op)
    if (!spec) return
    const args: Record<string, DslArgValue> = {}
    for (const a of spec.args) {
      if (!a.optional) args[a.name] = defaultArgValue(a)
    }
    onChange([...steps, { op, args }])
  }

  const updateStep = (i: number, step: DslStep) => {
    const next = [...steps]
    next[i] = step
    onChange(next)
  }

  const removeStep = (i: number) => onChange(steps.filter((_, idx) => idx !== i))

  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir
    if (j < 0 || j >= steps.length) return
    const next = [...steps]
    ;[next[i], next[j]] = [next[j], next[i]]
    onChange(next)
  }

  return (
    <div className="stack" style={{ gap: '0.4rem' }}>
      {steps.map((step, i) => {
        const spec = opSpec(step.op)
        return (
          <div key={i} className="row" style={{ background: 'var(--surface-2)', padding: '0.35rem 0.5rem', borderRadius: 6 }}>
            <strong>{step.op}(</strong>
            {spec?.args.map((a) => (
              <ArgInput
                key={a.name}
                spec={a}
                value={step.args[a.name] ?? defaultArgValue(a)}
                onChange={(v) => updateStep(i, { ...step, args: { ...step.args, [a.name]: v } })}
              />
            ))}
            <strong>)</strong>
            <button className="secondary" onClick={() => move(i, -1)} disabled={i === 0}>
              ↑
            </button>
            <button className="secondary" onClick={() => move(i, 1)} disabled={i === steps.length - 1}>
              ↓
            </button>
            <button className="danger" onClick={() => removeStep(i)}>
              ✕
            </button>
          </div>
        )
      })}
      <select value="" onChange={(e) => e.target.value && addStep(e.target.value)}>
        <option value="">{t('dsl.addStep')}</option>
        {DSL_OPS.map((o) => (
          <option key={o.op} value={o.op}>
            {o.op}
          </option>
        ))}
      </select>
    </div>
  )
}
