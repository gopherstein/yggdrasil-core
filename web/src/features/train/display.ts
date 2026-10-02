import i18n from '@/i18n'
import { ApiError } from '@/lib/api'
import type {
  ExampleFlag,
  MaterialUse,
  NodeTrainingFit,
  SpecializedAIView,
  TrainingJob,
  TrainingPreset,
  TrainingState,
} from '@/types/api'

/** What training is for, next to what connected knowledge is for. */
export function trainingExplainer(): string {
  return i18n.t('train:explainer')
}

export const MATERIAL_USES: MaterialUse[] = ['training', 'knowledge', 'both']

export function materialUseLabel(use: MaterialUse): string {
  return i18n.t(`train:uses.${use}.label`)
}

export function materialUseDescription(use: MaterialUse): string {
  return i18n.t(`train:uses.${use}.description`)
}

export const PRESETS: TrainingPreset[] = ['quick', 'balanced', 'quality']

export function presetLabel(preset: TrainingPreset): string {
  return i18n.t(`train:presets.${preset}.label`)
}

export function presetDescription(preset: TrainingPreset): string {
  return i18n.t(`train:presets.${preset}.description`)
}

// Blocking flags keep an example out of training; the others only warn.
const blockingFlags: ExampleFlag[] = ['empty', 'no_answer', 'duplicate', 'too_long']
const knownFlags: ExampleFlag[] = [...blockingFlags, 'short_answer', 'volatile_facts']

/** A flag's name, whether it keeps the example out, and why, or null for a flag this app doesn't know. */
export function flagInfo(flag: ExampleFlag): { label: string; blocking: boolean; help: string } | null {
  if (!knownFlags.includes(flag)) return null
  return {
    label: i18n.t(`train:flags.${flag}.label`),
    blocking: blockingFlags.includes(flag),
    help: i18n.t(`train:flags.${flag}.help`),
  }
}

/** How many examples have a flag, such as "3 duplicate". */
export function flagCount(flag: ExampleFlag, count: number): string {
  return knownFlags.includes(flag) ? i18n.t(`train:flagCount.${flag}`, { count }) : `${count} ${flag}`
}

const knownStates: TrainingState[] = [
  'queued',
  'preparing_dataset',
  'loading_model',
  'training',
  'exporting',
  'evaluating',
  'complete',
  'failed',
  'cancelled',
]

export function stateLabel(state: TrainingState): string {
  return knownStates.includes(state) ? i18n.t(`train:states.${state}`) : state
}

/** The steps a job passes through, in order, for the progress timeline. */
export const jobStages: TrainingState[] = [
  'queued',
  'preparing_dataset',
  'loading_model',
  'training',
  'exporting',
  'evaluating',
  'complete',
]

export function isTerminal(state: TrainingState): boolean {
  return state === 'complete' || state === 'failed' || state === 'cancelled'
}

export function formatDuration(totalSec: number | undefined | null): string {
  if (totalSec == null || totalSec < 0) return '—'
  const sec = Math.round(totalSec)
  if (sec < 60) return i18n.t('train:duration.s', { s: sec })
  const m = Math.floor(sec / 60)
  const s = sec % 60
  if (m < 60) return s ? i18n.t('train:duration.ms', { m, s }) : i18n.t('train:duration.m', { m })
  const h = Math.floor(m / 60)
  const rm = m % 60
  return rm ? i18n.t('train:duration.hm', { h, m: rm }) : i18n.t('train:duration.h', { h })
}

export function elapsedSec(job: TrainingJob, now: number = Date.now()): number {
  if (!job.started_at) return 0
  const start = Date.parse(job.started_at)
  const end = job.finished_at ? Date.parse(job.finished_at) : now
  return Math.max(0, (end - start) / 1000)
}

export function jobPercent(job: TrainingJob): number {
  const p = job.progress
  if (job.state === 'complete') return 100
  if (p.iter && p.iters) return Math.min(99, Math.round((p.iter / p.iters) * 100))
  return 0
}

export function fitTone(fit: Pick<NodeTrainingFit, 'label'>): string {
  switch (fit.label) {
    case 'comfortable':
      return 'bg-success/15 text-success'
    case 'tight':
      return 'bg-warning/15 text-warning'
    default:
      return 'bg-danger/15 text-danger'
  }
}

export function fitLabel(label: NodeTrainingFit['label']): string {
  return i18n.t(`train:fit.${label}`)
}

export type StepID = 'describe' | 'base' | 'material' | 'examples' | 'plan' | 'train' | 'test' | 'deploy'

// The build steps, in order; each name is train:steps.<id> in the catalog.
export const steps: { id: StepID }[] = [
  { id: 'describe' },
  { id: 'base' },
  { id: 'material' },
  { id: 'examples' },
  { id: 'plan' },
  { id: 'train' },
  { id: 'test' },
  { id: 'deploy' },
]

export function stepLabel(step: StepID): string {
  return i18n.t(`train:steps.${step}`)
}

/** Which steps are done, so the step bar can show progress through the build. */
export function completedSteps(view: SpecializedAIView): Set<StepID> {
  const done = new Set<StepID>()
  if (view.name && view.instructions) done.add('describe')
  if (view.base_model_id) done.add('base')
  if (view.materials.length > 0) done.add('material')
  if (view.dataset.usable >= 10) done.add('examples')
  if (view.jobs.length > 0) done.add('plan')
  if (view.revisions.length > 0) done.add('train')
  if (view.deployable_revisions.length > 0) done.add('test')
  if (view.deployed_revision > 0) done.add('deploy')
  return done
}

/** The first step that still needs attention. */
export function nextStep(view: SpecializedAIView): StepID {
  const done = completedSteps(view)
  const active = view.jobs.find((j) => !isTerminal(j.state))
  if (active) return 'train'
  for (const s of steps) {
    if (!done.has(s.id)) return s.id
  }
  return 'deploy'
}

export function lastUserTurn(messages: { role: string; content: string }[]): string {
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    if (messages[i].role === 'user') return messages[i].content
  }
  return ''
}

export function lastAnswer(messages: { role: string; content: string }[]): string {
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    if (messages[i].role === 'assistant') return messages[i].content
  }
  return ''
}

export function errorText(error: unknown, fallback?: string): string {
  return error instanceof ApiError || error instanceof Error ? error.message : (fallback ?? i18n.t('train:generic'))
}

// exportRevision is the revision to export as a GGUF file: the deployed one,
// or else the newest.
export function exportRevision(view: Pick<SpecializedAIView, 'deployed_revision' | 'revisions'>): number {
  if (view.deployed_revision > 0) return view.deployed_revision
  return view.revisions.reduce((max, r) => Math.max(max, r.revision), 0)
}
