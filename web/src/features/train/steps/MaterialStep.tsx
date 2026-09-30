import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import type { ClassifyResult, MaterialUse, SpecializedAIView, TrainingMaterial } from '@/types/api'
import { ConceptCards } from '../ConceptCards'
import { SampleFiles } from '../SampleFiles'
import { errorText, useDescriptions, useLabels } from '../display'

const ACCEPT = '.txt,.md,.markdown,.csv,.tsv,.json,.jsonl,.html,.htm'

export function UseBadge({ use }: { use: MaterialUse }) {
  const tone =
    use === 'training'
      ? 'bg-primary-soft text-primary-active'
      : use === 'knowledge'
        ? 'bg-mimir/15 text-mimir'
        : 'bg-accent-soft text-accent'
  return <span className={['status-chip', tone].join(' ')}>{useLabels[use]}</span>
}

export function MaterialStep({ view, onNext }: { view: SpecializedAIView; onNext: () => void }) {
  const [mode, setMode] = useState<'file' | 'chats'>('file')
  return (
    <div className="space-y-4">
      <ConceptCards compact />
      <div className="card space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="section-title">Add material</h3>
          <div className="flex gap-1.5">
            <button type="button" className={mode === 'file' ? 'btn-primary px-3 py-1 text-xs' : 'btn-secondary px-3 py-1 text-xs'} onClick={() => setMode('file')}>
              File or paste
            </button>
            <button type="button" className={mode === 'chats' ? 'btn-primary px-3 py-1 text-xs' : 'btn-secondary px-3 py-1 text-xs'} onClick={() => setMode('chats')}>
              Saved chats
            </button>
          </div>
        </div>
        {mode === 'file' ? <AddFile aiID={view.id} /> : <AddChats aiID={view.id} />}
        <SampleFiles open={Boolean(view.example)} />
      </div>
      <MaterialList view={view} />
      <button type="button" className="btn-primary px-3 py-1.5 text-sm" disabled={view.materials.length === 0} onClick={onNext}>
        Continue
      </button>
    </div>
  )
}

function AddFile({ aiID }: { aiID: string }) {
  const queryClient = useQueryClient()
  const [filename, setFilename] = useState('')
  const [text, setText] = useState('')
  const [use, setUse] = useState<MaterialUse | null>(null)
  const [preview, setPreview] = useState<ClassifyResult | null>(null)
  const [readError, setReadError] = useState('')

  const effectiveName = filename || 'pasted.txt'
  const classify = useMutation({
    mutationFn: (choice?: MaterialUse) => api.classifyMaterial(effectiveName, text, choice),
    onSuccess: (res) => {
      if (!res) return
      setPreview(res)
      setUse(res.use)
    },
  })
  const add = useMutation({
    mutationFn: () => api.addMaterial(aiID, { filename: effectiveName, text, use: use ?? undefined }),
    onSuccess: () => {
      setText('')
      setFilename('')
      setPreview(null)
      setUse(null)
      void queryClient.invalidateQueries({ queryKey: ['training'] })
    },
  })

  // Re-check the recommendation after the content settles.
  useEffect(() => {
    if (!text.trim()) {
      setPreview(null)
      return
    }
    const t = setTimeout(() => classify.mutate(undefined), 400)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, filename])

  async function readFile(file: File) {
    setReadError('')
    if (file.size > 20 * 1024 * 1024) {
      setReadError('That file is larger than 20 MB. Connect it from Knowledge instead, which reads it from disk.')
      return
    }
    setFilename(file.name)
    setText(await file.text())
  }

  const rec = preview?.recommendation
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <label className="btn-secondary cursor-pointer px-3 py-1.5 text-xs">
          Choose a file
          <input type="file" accept={ACCEPT} className="sr-only" onChange={(e) => e.target.files?.[0] && void readFile(e.target.files[0])} />
        </label>
        <span className="text-xs text-ink-faint">
          {filename ? filename : 'or paste below. JSONL chats, Q&A tables, CSV, Markdown, and text work.'}
        </span>
      </div>
      <textarea
        className="field min-h-36 w-full font-mono text-xs"
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={'Q: A question your AI should handle\nA: The answer you want it to give\n\nQ: …'}
        aria-label="Material"
      />
      {readError && <p className="text-sm text-danger">{readError}</p>}
      {rec && (
        <div className="rounded-lg bg-raised p-3 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-ink">Yggdrasil recommends</span>
            <UseBadge use={rec.use} />
            {rec.example_count > 0 && <span className="text-xs text-ink-muted">{rec.example_count} examples found</span>}
          </div>
          <ul className="mt-2 list-disc space-y-0.5 pl-5 text-ink-muted">
            {rec.reasons.map((r) => (
              <li key={r}>{r}</li>
            ))}
          </ul>
          <fieldset className="mt-3 space-y-1.5">
            <legend className="text-xs font-medium text-ink">Use this material for</legend>
            {(['training', 'knowledge', 'both'] as MaterialUse[]).map((u) => (
              <label key={u} className="flex items-start gap-2">
                <input
                  type="radio"
                  name="use"
                  value={u}
                  aria-label={useLabels[u]}
                  className="mt-1"
                  checked={use === u}
                  onChange={() => {
                    setUse(u)
                    classify.mutate(u)
                  }}
                />
                <span>
                  <span className="text-ink">{useLabels[u]}</span>
                  {u === rec.use && <span className="text-xs text-ink-faint"> (recommended)</span>}
                  <span className="block text-xs text-ink-muted">{useDescriptions[u]}</span>
                </span>
              </label>
            ))}
          </fieldset>
          {preview?.warning && <p className="mt-2 rounded-md bg-warning/10 p-2 text-warning">{preview.warning}</p>}
          {preview?.error && <p className="mt-2 rounded-md bg-danger/10 p-2 text-danger">{preview.error}</p>}
        </div>
      )}
      {add.error && <p className="text-sm text-danger">{errorText(add.error)}</p>}
      <button
        type="button"
        className="btn-primary px-3 py-1.5 text-sm"
        disabled={!text.trim() || !preview || Boolean(preview.error) || add.isPending}
        onClick={() => add.mutate()}
      >
        {add.isPending ? 'Adding…' : 'Add material'}
      </button>
    </div>
  )
}

function AddChats({ aiID }: { aiID: string }) {
  const queryClient = useQueryClient()
  const conversations = useQuery({ queryKey: ['conversations'], queryFn: () => api.getConversations() })
  const [picked, setPicked] = useState<string[]>([])
  const add = useMutation({
    mutationFn: () => api.addConversations(aiID, picked),
    onSuccess: () => {
      setPicked([])
      void queryClient.invalidateQueries({ queryKey: ['training'] })
    },
  })
  const list = conversations.data ?? []
  return (
    <div className="space-y-3">
      <p className="text-sm text-ink-muted">
        Pick chats where the answers are the kind you want. Each becomes one training example you can edit in the next
        step.
      </p>
      {list.length === 0 && <p className="text-sm text-ink-faint">No saved chats.</p>}
      <ul className="max-h-64 space-y-1 overflow-y-auto">
        {list.map((c) => (
          <li key={c.id}>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={picked.includes(c.id)}
                onChange={(e) => setPicked(e.target.checked ? [...picked, c.id] : picked.filter((id) => id !== c.id))}
              />
              <span className="text-ink">{c.title || 'Untitled chat'}</span>
            </label>
          </li>
        ))}
      </ul>
      {add.error && <p className="text-sm text-danger">{errorText(add.error)}</p>}
      <button type="button" className="btn-primary px-3 py-1.5 text-sm" disabled={picked.length === 0 || add.isPending} onClick={() => add.mutate()}>
        Add {picked.length || ''} {picked.length === 1 ? 'chat' : 'chats'}
      </button>
    </div>
  )
}

function MaterialList({ view }: { view: SpecializedAIView }) {
  const queryClient = useQueryClient()
  const remove = useMutation({
    mutationFn: (m: TrainingMaterial) => api.deleteMaterial(view.id, m.id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['training'] }),
  })
  if (view.materials.length === 0) return null
  return (
    <div className="card space-y-2">
      <h3 className="section-title">Material</h3>
      <ul className="divide-y divide-line/60">
        {view.materials.map((m) => (
          <li key={m.id} className="flex flex-wrap items-start gap-3 py-2">
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium text-ink">{m.name}</span>
                <UseBadge use={m.use} />
                {m.use !== m.recommended.use && <span className="text-xs text-ink-faint">recommended {useLabels[m.recommended.use]}</span>}
              </div>
              <p className="mt-0.5 text-xs text-ink-muted">
                {m.example_count > 0 && `${m.example_count} examples`}
                {m.example_count > 0 && m.knowledge_source_id && ' · '}
                {m.knowledge_source_id && 'Connected as knowledge'}
              </p>
              {m.warning && <p className="mt-1 text-xs text-warning">{m.warning}</p>}
            </div>
            <button
              type="button"
              className="btn-secondary px-2 py-1 text-xs"
              disabled={remove.isPending}
              onClick={() => {
                if (window.confirm(`Remove ${m.name}? Its examples and connected knowledge are removed too.`)) remove.mutate(m)
              }}
            >
              Remove
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
