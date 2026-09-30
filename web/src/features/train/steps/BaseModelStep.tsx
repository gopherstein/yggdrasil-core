import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'
import { formatBytes } from '@/lib/format'
import type { SpecializedAIView } from '@/types/api'
import { errorText, fitLabels, fitTone } from '../display'

export function BaseModelStep({ view, onNext }: { view: SpecializedAIView; onNext: () => void }) {
  const queryClient = useQueryClient()
  const goal = `${view.name}. ${view.goal}`
  const choices = useQuery({ queryKey: ['training', 'bases', goal], queryFn: () => api.baseModels(goal) })
  const choose = useMutation({
    mutationFn: (modelID: string) => api.updateAI(view.id, { base_model_id: modelID }),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['training'] }),
  })

  return (
    <div className="card space-y-4">
      <div>
        <h3 className="section-title">Choose a base model</h3>
        <p className="mt-1 text-sm text-ink-muted">
          Smaller models train faster and answer faster. Training fit is checked separately from running the model,
          because training needs several times more memory.
        </p>
      </div>
      {choices.isLoading && <p className="text-sm text-ink-muted">Checking what this computer can train…</p>}
      {choices.error && <p className="text-sm text-danger">{errorText(choices.error)}</p>}
      <ul className="space-y-2">
        {(choices.data ?? []).map((c) => {
          const selected = c.model_id === view.base_model_id
          return (
            <li key={c.model_id}>
              <button
                type="button"
                disabled={!c.fit.eligible || choose.isPending}
                className={[
                  'selectable w-full text-left disabled:cursor-not-allowed disabled:opacity-60',
                  selected ? 'shadow-[inset_0_0_0_1.5px_rgb(var(--rgb-primary))]' : '',
                ].join(' ')}
                onClick={() => choose.mutate(c.model_id)}
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium text-ink">{c.display_name}</span>
                  {c.recommended && <span className="badge-preferred">Recommended</span>}
                  {selected && <span className="status-chip bg-primary-soft text-primary-active">Selected</span>}
                  <span className={['status-chip', fitTone(c.fit)].join(' ')}>{fitLabels[c.fit.label]}</span>
                  {c.installed && <span className="status-chip bg-raised text-ink-muted">Installed</span>}
                </div>
                <p className="mt-1 text-xs text-ink-muted">
                  {c.license}
                  {c.fit.eligible && ` · training needs about ${formatBytes(c.fit.memory_needed_bytes)}`}
                  {c.fit.eligible && c.fit.download_bytes > 0 && ` · downloads ${formatBytes(c.fit.download_bytes)} once`}
                </p>
                <ul className="mt-1 space-y-0.5 text-sm text-ink-muted">
                  {c.reasons.map((r) => (
                    <li key={r}>{r}</li>
                  ))}
                </ul>
              </button>
            </li>
          )
        })}
      </ul>
      {view.base_model && !view.base_model.installed && (
        <p className="rounded-lg bg-warning/10 p-3 text-sm text-warning">
          {view.base_model.display_name} is not installed. Training works without it, but you need it to test and use
          the result. <Link to="/models" className="underline">Install it from Models</Link>.
        </p>
      )}
      {choose.error && <p className="text-sm text-danger">{errorText(choose.error)}</p>}
      <button type="button" className="btn-primary px-3 py-1.5 text-sm" disabled={!view.base_model_id} onClick={onNext}>
        Continue
      </button>
    </div>
  )
}
