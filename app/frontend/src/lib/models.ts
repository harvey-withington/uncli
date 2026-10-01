import type { ModelInfo } from './api'

export interface ModelOption {
  value: string
  label: string
  description: string
}

// Before the CLI has reported its models, offer the stable aliases.
const FALLBACK: ModelOption[] = [
  { value: 'opus', label: 'Opus', description: '' },
  { value: 'sonnet', label: 'Sonnet', description: '' },
  { value: 'haiku', label: 'Haiku', description: '' },
]

// modelOptions lists the models the CLI reported, dropping its "default"
// entry (UNCLI always names the model) and keeping aliases first.
export function modelOptions(models: ModelInfo[] | null, current?: string): ModelOption[] {
  const list: ModelOption[] = models?.length
    ? models.filter(m => m.value !== 'default').map(m => ({ value: m.value, label: m.displayName || m.value, description: m.description }))
    : [...FALLBACK]
  if (current && !list.some(o => o.value === current)) list.unshift({ value: current, label: current, description: '' })
  return list
}

// modelLabel gives a short display name for a model value.
export function modelLabel(models: ModelInfo[] | null, value: string): string {
  const m = models?.find(x => x.value === value || x.resolved === value)
  if (m?.displayName) return m.displayName
  return value.charAt(0).toUpperCase() + value.slice(1)
}
