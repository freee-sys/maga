// The Go DSL package (internal/dsl) serializes pipeline steps through
// custom MarshalJSON/UnmarshalJSON logic that swaggo can't see via
// struct reflection, so the auto-generated schema types for dsl.Op /
// dsl.Arg (positional {s,i,m} fields) do NOT match the real wire format.
// The real format — mirrored here — is:
//   { "op": "replace", "args": { "old": "-a-", "new": "-b-" } }
// This file is the single source of truth for the UI; keep ArgSpec in
// sync with internal/dsl/ops.go's registry by hand.

export type ArgKind = 'string' | 'int' | 'map'

export interface ArgSpec {
  name: string
  kind: ArgKind
  optional?: boolean
}

export interface OpSpec {
  op: string
  args: ArgSpec[]
}

export const DSL_OPS: OpSpec[] = [
  { op: 'trim', args: [] },
  { op: 'trimLeft', args: [{ name: 'cutset', kind: 'string' }] },
  { op: 'trimRight', args: [{ name: 'cutset', kind: 'string' }] },
  { op: 'upper', args: [] },
  { op: 'lower', args: [] },
  { op: 'replace', args: [{ name: 'old', kind: 'string' }, { name: 'new', kind: 'string' }] },
  {
    op: 'replaceN',
    args: [{ name: 'old', kind: 'string' }, { name: 'new', kind: 'string' }, { name: 'n', kind: 'int' }],
  },
  { op: 'replaceRegex', args: [{ name: 'pattern', kind: 'string' }, { name: 'repl', kind: 'string' }] },
  { op: 'deleteSubstring', args: [{ name: 'substr', kind: 'string' }] },
  { op: 'deleteRange', args: [{ name: 'start', kind: 'int' }, { name: 'end', kind: 'int' }] },
  { op: 'insertAt', args: [{ name: 'pos', kind: 'int' }, { name: 'text', kind: 'string' }] },
  { op: 'slice', args: [{ name: 'start', kind: 'int' }, { name: 'end', kind: 'int' }] },
  { op: 'prefix', args: [{ name: 'text', kind: 'string' }] },
  { op: 'suffix', args: [{ name: 'text', kind: 'string' }] },
  { op: 'padLeft', args: [{ name: 'length', kind: 'int' }, { name: 'pad', kind: 'string' }] },
  { op: 'padRight', args: [{ name: 'length', kind: 'int' }, { name: 'pad', kind: 'string' }] },
  { op: 'default', args: [{ name: 'value', kind: 'string' }] },
  {
    op: 'mapValues',
    args: [{ name: 'table', kind: 'map' }, { name: 'fallback', kind: 'string', optional: true }],
  },
]

export type DslArgValue = string | number | Record<string, string>

export interface DslStep {
  op: string
  args: Record<string, DslArgValue>
}

export type CaptureTransforms = Record<string, DslStep[]>

export function opSpec(name: string): OpSpec | undefined {
  return DSL_OPS.find((s) => s.op === name)
}
