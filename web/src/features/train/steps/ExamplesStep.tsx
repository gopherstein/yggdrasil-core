import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { api } from '@/lib/api'
import type { ExampleFlag, SpecializedAIView, TrainingExample, TrainingMessage } from '@/types/api'
import { errorText, flagLabels, lastAnswer, lastUserTurn } from '../display'

const PAGE = 50

export function ExamplesStep({ view, onNext }: { view: SpecializedAIView; onNext: () => void }) {
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState<'all' | 'flagged' | 'excluded'>('all')
  const [shown, setShown] = useState(PAGE)
  const examples = useQuery({ queryKey: ['training', 'examples', view.id], queryFn: () => api.listExamples(view.id) })
  const refresh = () => void queryClient.invalidateQueries({ queryKey: ['training'] })

  const update = useMutation({
    mutationFn: (args: { id: string; messages?: TrainingMessage[]; excluded?: boolean }) =>
      api.updateExample(view.id, args.id, { messages: args.messages, excluded: args.excluded }),
    onSuccess: refresh,
  })
  const remove = useMutation({ mutationFn: (id: string) => api.deleteExample(view.id, id), onSuccess: refresh })

  const stats = examples.data?.stats ?? view.dataset
  const list = (examples.data?.examples ?? []).filter((ex) =>
    filter === 'flagged' ? (ex.flags?.length ?? 0) > 0 : filter === 'excluded' ? ex.excluded : true,
  )

  return (
    <div className="space-y-4">
      <div className="card space-y-3">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <h3 className="section-title">Review examples</h3>
          <p className="text-sm text-ink-muted">
            <span className="font-semibold tabular-nums text-ink">{stats.usable}</span> of {stats.total} will train
          </p>
        </div>
        {(stats.warnings ?? []).map((w) => (
          <p key={w} className="rounded-md bg-warning/10 p-2 text-sm text-warning">
            {w}
          </p>
        ))}
        {Object.keys(stats.flagged).length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {(Object.entries(stats.flagged) as [ExampleFlag, number][]).map(([flag, n]) => (
              <span key={flag} className={['status-chip', flagLabels[flag]?.blocking ? 'bg-danger/15 text-danger' : 'bg-warning/15 text-warning'].join(' ')} title={flagLabels[flag]?.help}>
                {n} {flagLabels[flag]?.label.toLowerCase() ?? flag}
              </span>
            ))}
          </div>
        )}
        <div className="flex gap-1.5">
          {(['all', 'flagged', 'excluded'] as const).map((f) => (
            <button key={f} type="button" className={filter === f ? 'btn-primary px-3 py-1 text-xs' : 'btn-secondary px-3 py-1 text-xs'} onClick={() => setFilter(f)}>
              {f === 'all' ? 'All' : f === 'flagged' ? 'Flagged' : 'Excluded'}
            </button>
          ))}
        </div>
        {examples.isLoading && <p className="text-sm text-ink-muted">Loading examples…</p>}
        <ul className="space-y-2">
          {list.slice(0, shown).map((ex) => (
            <ExampleRow
              key={ex.id}
              example={ex}
              busy={update.isPending || remove.isPending}
              onToggle={() => update.mutate({ id: ex.id, excluded: !ex.excluded })}
              onSave={(messages) => update.mutate({ id: ex.id, messages })}
              onDelete={() => remove.mutate(ex.id)}
            />
          ))}
        </ul>
        {list.length > shown && (
          <button type="button" className="btn-secondary px-3 py-1 text-xs" onClick={() => setShown(shown + PAGE)}>
            Show more ({list.length - shown} left)
          </button>
        )}
        {(update.error || remove.error) && <p className="text-sm text-danger">{errorText(update.error ?? remove.error)}</p>}
      </div>
      <AddExample aiID={view.id} onAdded={refresh} />
      <button type="button" className="btn-primary px-3 py-1.5 text-sm" disabled={stats.usable < 10} onClick={onNext}>
        Continue
      </button>
    </div>
  )
}

function ExampleRow({
  example,
  busy,
  onToggle,
  onSave,
  onDelete,
}: {
  example: TrainingExample
  busy: boolean
  onToggle: () => void
  onSave: (messages: TrainingMessage[]) => void
  onDelete: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [answer, setAnswer] = useState(lastAnswer(example.messages))
  const turns = example.messages.filter((m) => m.role !== 'system').length
  return (
    <li className={['rounded-lg bg-raised p-3 text-sm', example.excluded ? 'opacity-60' : ''].join(' ')}>
      <div className="flex flex-wrap items-center gap-1.5">
        {(example.flags ?? []).map((f) => (
          <span key={f} className={['status-chip', flagLabels[f]?.blocking ? 'bg-danger/15 text-danger' : 'bg-warning/15 text-warning'].join(' ')} title={flagLabels[f]?.help}>
            {flagLabels[f]?.label ?? f}
          </span>
        ))}
        {example.excluded && <span className="status-chip bg-raised text-ink-muted">Excluded</span>}
        {turns > 2 && <span className="text-xs text-ink-faint">{turns} turns</span>}
      </div>
      <p className="mt-1 text-ink">
        <span className="label-caps mr-1">Q</span>
        {lastUserTurn(example.messages) || <em className="text-ink-faint">empty</em>}
      </p>
      {editing ? (
        <textarea className="field mt-1 min-h-20 w-full text-sm" value={answer} onChange={(e) => setAnswer(e.target.value)} aria-label="Answer" />
      ) : (
        <p className="mt-1 whitespace-pre-wrap text-ink-muted">
          <span className="label-caps mr-1">A</span>
          {lastAnswer(example.messages) || <em className="text-ink-faint">no answer</em>}
        </p>
      )}
      <div className="mt-2 flex gap-1.5">
        {editing ? (
          <>
            <button
              type="button"
              className="btn-primary px-2 py-1 text-xs"
              disabled={busy}
              onClick={() => {
                const msgs = [...example.messages]
                const i = msgs.map((m) => m.role).lastIndexOf('assistant')
                if (i >= 0) msgs[i] = { ...msgs[i], content: answer }
                else msgs.push({ role: 'assistant', content: answer })
                onSave(msgs)
                setEditing(false)
              }}
            >
              Save
            </button>
            <button type="button" className="btn-secondary px-2 py-1 text-xs" onClick={() => setEditing(false)}>
              Cancel
            </button>
          </>
        ) : (
          <button type="button" className="btn-secondary px-2 py-1 text-xs" onClick={() => setEditing(true)}>
            Edit answer
          </button>
        )}
        <button type="button" className="btn-secondary px-2 py-1 text-xs" disabled={busy} onClick={onToggle}>
          {example.excluded ? 'Include' : 'Exclude'}
        </button>
        <button type="button" className="btn-secondary px-2 py-1 text-xs" disabled={busy} onClick={onDelete}>
          Delete
        </button>
      </div>
    </li>
  )
}

function AddExample({ aiID, onAdded }: { aiID: string; onAdded: () => void }) {
  const [question, setQuestion] = useState('')
  const [answer, setAnswer] = useState('')
  const add = useMutation({
    mutationFn: () =>
      api.addExample(aiID, [
        { role: 'user', content: question },
        { role: 'assistant', content: answer },
      ]),
    onSuccess: () => {
      setQuestion('')
      setAnswer('')
      onAdded()
    },
  })
  return (
    <div className="card space-y-2">
      <h3 className="section-title">Write an example</h3>
      <p className="text-sm text-ink-muted">Show one question and the answer you want. Write how to use facts, not the facts themselves.</p>
      <input className="field w-full" value={question} onChange={(e) => setQuestion(e.target.value)} placeholder="Customer question" aria-label="Question" />
      <textarea className="field min-h-20 w-full" value={answer} onChange={(e) => setAnswer(e.target.value)} placeholder="The answer you want" aria-label="Answer" />
      {add.error && <p className="text-sm text-danger">{errorText(add.error)}</p>}
      <button type="button" className="btn-secondary px-3 py-1.5 text-sm" disabled={!question.trim() || !answer.trim() || add.isPending} onClick={() => add.mutate()}>
        Add example
      </button>
    </div>
  )
}
