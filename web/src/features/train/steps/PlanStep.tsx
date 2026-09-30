import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '@/lib/api'
import { KnowledgePicker } from '@/features/knowledge/KnowledgePicker'
import { formatBytes } from '@/lib/format'
import type { NodeTrainingFit, SpecializedAIView, TrainingHyper, TrainingPreset } from '@/types/api'
import { errorText, fitLabels, fitTone, formatDuration, presetInfo } from '../display'

export function PlanStep({ view, onStarted }: { view: SpecializedAIView; onStarted: () => void }) {
  const queryClient = useQueryClient()
  const plan = useQuery({ queryKey: ['training', 'plan', view.id], queryFn: () => api.trainingPlan(view.id) })
  const refresh = () => void queryClient.invalidateQueries({ queryKey: ['training'] })

  const setKnowledge = useMutation({
    mutationFn: (ids: string[]) => api.updateAI(view.id, { knowledge_sources: ids }),
    onSuccess: refresh,
  })
  const setPreset = useMutation({ mutationFn: (preset: TrainingPreset) => api.updateAI(view.id, { preset }), onSuccess: refresh })
  const start = useMutation({
    mutationFn: (nodeId?: string) => api.startTraining(view.id, nodeId),
    onSuccess: () => {
      refresh()
      onStarted()
    },
  })

  const p = plan.data
  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-2">
        <div className="card-outline space-y-2 border-l-4 !border-l-primary p-4">
          <p className="label-caps text-primary">Will be trained</p>
          <p className="text-sm text-ink">
            <span className="font-semibold tabular-nums">{p?.examples ?? view.dataset.usable}</span> examples teach how to
            respond.
          </p>
          <p className="text-xs text-ink-muted">Base model: {view.base_model?.display_name ?? 'not chosen'}</p>
        </div>
        <div className="card-outline space-y-2 border-l-4 !border-l-mimir p-4">
          <p className="label-caps text-mimir">Stays connected</p>
          <KnowledgePicker selected={view.knowledge_sources} disabled={setKnowledge.isPending} onChange={(ids) => setKnowledge.mutate(ids)} />
          {setKnowledge.error && <p className="text-xs text-danger">{errorText(setKnowledge.error)}</p>}
          <p className="text-xs text-ink-muted">Looked up on every question. Edit it later without retraining.</p>
        </div>
      </div>

      <div className="card space-y-3">
        <h3 className="section-title">Training effort</h3>
        <div className="grid gap-2 md:grid-cols-3">
          {(Object.keys(presetInfo) as TrainingPreset[]).map((key) => (
            <button
              key={key}
              type="button"
              className={['selectable text-left', view.preset === key ? 'shadow-[inset_0_0_0_1.5px_rgb(var(--rgb-primary))]' : ''].join(' ')}
              onClick={() => setPreset.mutate(key)}
            >
              <p className="font-medium text-ink">{presetInfo[key].label}</p>
              <p className="mt-1 text-xs text-ink-muted">{presetInfo[key].description}</p>
            </button>
          ))}
        </div>
        <AdvancedSettings view={view} planned={p?.hyper} />
      </div>

      <div className="card space-y-3">
        <h3 className="section-title">Where it trains</h3>
        {plan.isLoading && <p className="text-sm text-ink-muted">Estimating…</p>}
        <ul className="space-y-2">
          {(p?.fits ?? []).map((f) => (
            <FitRow
              key={f.node_id || f.node_name}
              fit={f}
              chosen={p?.chosen?.node_id === f.node_id}
              canStart={Boolean(p?.ready) && !start.isPending}
              onStart={() => start.mutate(f.node_id)}
            />
          ))}
        </ul>
      </div>

      {p && (p.blockers.length > 0 || p.warnings.length > 0) && (
        <div className="space-y-2">
          {p.blockers.map((b) => (
            <p key={b} className="rounded-md bg-danger/10 p-2 text-sm text-danger">
              {b}
            </p>
          ))}
          {p.warnings.map((w) => (
            <p key={w} className="rounded-md bg-warning/10 p-2 text-sm text-warning">
              {w}
            </p>
          ))}
        </div>
      )}
      {start.error && <p className="text-sm text-danger">{errorText(start.error)}</p>}
      <button type="button" className="btn-primary px-4 py-2 text-sm" disabled={!p?.ready || start.isPending} onClick={() => start.mutate(undefined)}>
        {start.isPending ? 'Starting…' : `Train revision ${p?.next_revision ?? 1}`}
      </button>
    </div>
  )
}

function FitRow({
  fit,
  chosen,
  canStart,
  onStart,
}: {
  fit: NodeTrainingFit
  chosen: boolean
  canStart: boolean
  onStart: () => void
}) {
  return (
    <li className={['rounded-lg bg-raised p-3 text-sm', chosen ? 'shadow-[inset_0_0_0_1.5px_rgb(var(--rgb-primary))]' : ''].join(' ')}>
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-medium text-ink">{fit.node_name}</span>
        {fit.local && <span className="text-xs text-ink-faint">this computer</span>}
        <span className={['status-chip', fitTone(fit)].join(' ')}>{fitLabels[fit.label]}</span>
        {chosen && <span className="status-chip bg-norn/15 text-norn">Norn picked this</span>}
        {!chosen && fit.eligible && (
          <button type="button" className="btn-secondary ml-auto px-2 py-0.5 text-xs" disabled={!canStart} onClick={onStart}>
            Train here instead
          </button>
        )}
      </div>
      <p className="mt-1 text-ink-muted">{fit.reason}</p>
      {fit.eligible && (
        <dl className="mt-2 grid grid-cols-2 gap-x-4 gap-y-1 text-xs sm:grid-cols-4">
          <div>
            <dt className="text-ink-faint">Memory</dt>
            <dd className="tabular-nums text-ink">
              {formatBytes(fit.memory_needed_bytes)} of {formatBytes(fit.memory_available_bytes)}
            </dd>
          </div>
          <div>
            <dt className="text-ink-faint">Download</dt>
            <dd className="tabular-nums text-ink">{fit.download_bytes ? formatBytes(fit.download_bytes) : 'None'}</dd>
          </div>
          <div>
            <dt className="text-ink-faint">Disk</dt>
            <dd className="tabular-nums text-ink">{formatBytes(fit.storage_needed_bytes)}</dd>
          </div>
          <div>
            <dt className="text-ink-faint">Time, roughly</dt>
            <dd className="tabular-nums text-ink">{formatDuration(fit.duration_sec)}</dd>
          </div>
        </dl>
      )}
      {(fit.notes ?? []).map((n) => (
        <p key={n} className="mt-1 text-xs text-ink-muted">
          {n}
        </p>
      ))}
    </li>
  )
}

const advancedFields: { key: keyof TrainingHyper; label: string; help: string; step?: string }[] = [
  { key: 'epochs', label: 'Epochs', help: 'Passes over all examples.' },
  { key: 'rank', label: 'LoRA rank', help: 'Adapter capacity. Higher learns more and needs more memory.' },
  { key: 'layers', label: 'Layers', help: 'How many of the last layers to train. -1 trains all.' },
  { key: 'learning_rate', label: 'Learning rate', help: 'Step size. Too high forgets the base model.', step: 'any' },
  { key: 'batch_size', label: 'Batch size', help: 'Examples per step.' },
  { key: 'max_seq_length', label: 'Max tokens', help: 'Longest example, in tokens.' },
]

function AdvancedSettings({ view, planned }: { view: SpecializedAIView; planned?: TrainingHyper }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<TrainingHyper>(view.advanced ?? {})
  const save = useMutation({
    mutationFn: (clear: boolean) => api.updateAI(view.id, clear ? { clear_advanced: true } : { advanced: draft }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['training'] }),
  })
  return (
    <details className="text-sm" open={Boolean(view.advanced)}>
      <summary className="cursor-pointer text-ink-muted">Advanced settings</summary>
      <div className="mt-3 space-y-3">
        <p className="text-xs text-ink-muted">
          Leave a field empty to use the preset ({planned ? `${planned.method?.toUpperCase()}, ${planned.iters} steps` : '…'}). Settings you pin here are
          used even if they do not fit in memory.
        </p>
        <label className="flex items-center gap-2 text-xs">
          <span className="w-28 text-ink">Method</span>
          <select
            className="field"
            value={draft.method ?? ''}
            onChange={(e) => setDraft({ ...draft, method: (e.target.value || undefined) as TrainingHyper['method'] })}
          >
            <option value="">Preset ({planned?.method ?? '—'})</option>
            <option value="lora">LoRA, full-precision base</option>
            <option value="qlora">QLoRA, 4-bit base</option>
          </select>
        </label>
        <div className="grid gap-2 sm:grid-cols-2">
          {advancedFields.map((f) => (
            <label key={f.key} className="flex flex-col gap-0.5 text-xs" title={f.help}>
              <span className="text-ink">{f.label}</span>
              <input
                type="number"
                step={f.step ?? '1'}
                className="field"
                placeholder={planned?.[f.key] != null ? String(planned[f.key]) : ''}
                value={draft[f.key] != null ? String(draft[f.key]) : ''}
                onChange={(e) => setDraft({ ...draft, [f.key]: e.target.value === '' ? undefined : Number(e.target.value) })}
              />
              <span className="text-ink-faint">{f.help}</span>
            </label>
          ))}
        </div>
        {save.error && <p className="text-danger">{errorText(save.error)}</p>}
        <div className="flex gap-2">
          <button type="button" className="btn-secondary px-3 py-1 text-xs" disabled={save.isPending} onClick={() => save.mutate(false)}>
            Save advanced settings
          </button>
          {view.advanced && (
            <button type="button" className="btn-secondary px-3 py-1 text-xs" disabled={save.isPending} onClick={() => { setDraft({}); save.mutate(true) }}>
              Use the preset
            </button>
          )}
        </div>
      </div>
    </details>
  )
}
