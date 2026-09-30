import { describe, expect, it } from 'vitest'
import type { SpecializedAIView, TrainingJob } from '@/types/api'
import { completedSteps, formatDuration, jobPercent, nextStep } from './display'

function view(partial: Partial<SpecializedAIView>): SpecializedAIView {
  return {
    id: 'a',
    slug: 'a',
    name: 'Tire Bot',
    goal: '',
    instructions: 'You are Tire Bot.',
    base_model_id: '',
    preset: 'balanced',
    knowledge_sources: [],
    deployed_revision: 0,
    created_at: '',
    updated_at: '',
    model_id: 'sai:a',
    materials: [],
    dataset: { total: 0, usable: 0, excluded: 0, flagged: {}, tokens: 0, p95_tokens: 0 },
    revisions: [],
    jobs: [],
    eval_runs: [],
    test_prompts: [],
    deployable_revisions: [],
    ...partial,
  }
}

const job = (state: TrainingJob['state'], extra: Partial<TrainingJob> = {}): TrainingJob => ({
  id: 'j',
  ai_id: 'a',
  revision: 1,
  node_id: 'n',
  backend: 'mlx',
  state,
  progress: {},
  hyper: {},
  created_at: '',
  ...extra,
})

describe('formatDuration', () => {
  it('reads naturally at each scale', () => {
    expect(formatDuration(42)).toBe('42s')
    expect(formatDuration(125)).toBe('2m 5s')
    expect(formatDuration(3600)).toBe('1h')
    expect(formatDuration(5460)).toBe('1h 31m')
    expect(formatDuration(undefined)).toBe('—')
  })
})

describe('build steps', () => {
  it('starts at the base model once the AI is described', () => {
    expect(nextStep(view({}))).toBe('base')
  })

  it('asks for examples until there are enough', () => {
    const v = view({
      base_model_id: 'm',
      materials: [{ id: 'x' } as SpecializedAIView['materials'][number]],
      dataset: { total: 4, usable: 4, excluded: 0, flagged: {}, tokens: 0, p95_tokens: 0 },
    })
    expect(nextStep(v)).toBe('examples')
  })

  it('jumps to training while a job runs', () => {
    expect(nextStep(view({ jobs: [job('training')] }))).toBe('train')
  })

  it('marks every step done for a deployed AI', () => {
    const v = view({
      base_model_id: 'm',
      materials: [{ id: 'x' } as SpecializedAIView['materials'][number]],
      dataset: { total: 20, usable: 20, excluded: 0, flagged: {}, tokens: 0, p95_tokens: 0 },
      jobs: [job('complete')],
      revisions: [{ revision: 1 } as SpecializedAIView['revisions'][number]],
      deployable_revisions: [1],
      deployed_revision: 1,
    })
    expect(completedSteps(v).size).toBe(8)
  })
})

describe('jobPercent', () => {
  it('follows iterations and caps below 100 until complete', () => {
    expect(jobPercent(job('training', { progress: { iter: 30, iters: 60 } }))).toBe(50)
    expect(jobPercent(job('exporting', { progress: { iter: 60, iters: 60 } }))).toBe(99)
    expect(jobPercent(job('complete'))).toBe(100)
  })
})
