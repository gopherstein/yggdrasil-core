import { useTranslation } from 'react-i18next'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useRef, useState, type DragEvent } from 'react'
import { LoadingSpinner } from '@/components/ui/LoadingSpinner'
import { api, uploadModel } from '@/lib/api'
import { formatBytes } from '@/lib/format'
import { rovingKeyDown } from '@/lib/roving'
import type { FoundModel, ModelImportResult } from '@/types/api'

type Way = 'file' | 'apps' | 'link'

// Each app's name, as people know it.
const APP_NAMES: Record<string, string> = { lmstudio: 'LM Studio', ollama: 'Ollama', llamacpp: 'llama.cpp', gpt4all: 'GPT4All' }

function errorText(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

/**
 * Adds a model that isn't in the catalog (#467): a GGUF file, sent from
 * this browser or found by its location on this computer, one another
 * local AI app already downloaded, or a link. Models are added to this
 * computer.
 */
export function AddModelPanel({ onClose, onAdded }: { onClose: () => void; onAdded: (modelId: string) => void }) {
  const { t } = useTranslation('models')
  const [way, setWay] = useState<Way>('file')
  const [done, setDone] = useState<string | null>(null)
  const added = (result: ModelImportResult | null, fallbackName?: string) => {
    if (!result) return
    // The name it was saved with: the one asked for, else its header's.
    const name = fallbackName || result.details?.name || result.model_id
    setDone(result.status === 'copying' ? t('add.copying', { name }) : t('add.added', { name }))
    onAdded(result.model_id)
  }

  return (
    <section className="space-y-4 rounded-xl border border-line bg-surface p-4" aria-labelledby="add-model-title">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 id="add-model-title" className="section-title">
            {t('add.title')}
          </h2>
          <p className="mt-1 text-sm text-ink-muted">{t('add.description')}</p>
        </div>
        <button type="button" className="btn-secondary text-sm" onClick={onClose}>
          {t('browse.close')}
        </button>
      </div>

      <div role="tablist" aria-label={t('add.title')} className="segmented" onKeyDown={rovingKeyDown}>
        {(['file', 'apps', 'link'] as const).map((id) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={way === id}
            tabIndex={way === id ? 0 : -1}
            className="segmented-item"
            onClick={() => {
              setWay(id)
              setDone(null)
            }}
          >
            {t(`add.ways.${id}`)}
          </button>
        ))}
      </div>

      {way === 'file' ? <FromFile onAdded={added} /> : null}
      {way === 'apps' ? <FromApps onAdded={added} /> : null}
      {way === 'link' ? <FromLink onAdded={added} /> : null}

      {done ? (
        <p role="status" className="text-sm text-ink">
          {done}
        </p>
      ) : null}
    </section>
  )
}

function FromFile({ onAdded }: { onAdded: (r: ModelImportResult | null, name?: string) => void }) {
  const { t } = useTranslation('models')
  const input = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const [progress, setProgress] = useState<{ sent: number; total: number } | null>(null)
  const [path, setPath] = useState('')
  const [inPlace, setInPlace] = useState(false)
  const upload = useMutation({
    mutationFn: (file: File) => uploadModel(file, (sent, total) => setProgress({ sent, total })),
    onSuccess: (r) => onAdded(r),
    onSettled: () => setProgress(null),
  })
  const byPath = useMutation({
    mutationFn: () => api.importModelPath({ path: path.trim(), in_place: inPlace }),
    onSuccess: (r) => onAdded(r),
  })
  const send = (file?: File) => {
    if (!file) return
    upload.reset()
    upload.mutate(file)
  }
  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragging(false)
    send(e.dataTransfer.files[0])
  }

  return (
    <div className="space-y-4">
      <div
        onDragOver={(e) => {
          e.preventDefault()
          setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        className={[
          'flex flex-col items-center gap-2 rounded-xl border-2 border-dashed px-4 py-6 text-center text-sm',
          dragging ? 'border-primary bg-primary-soft' : 'border-line',
        ].join(' ')}
      >
        {progress ? (
          <>
            <p className="text-ink">{t('add.file.sending', { sent: formatBytes(progress.sent), total: formatBytes(progress.total) })}</p>
            <progress className="w-full max-w-sm" value={progress.sent} max={progress.total || 1} />
          </>
        ) : (
          <>
            <p className="text-ink-muted">{t('add.file.drop')}</p>
            <button type="button" className="btn-primary btn-sm" disabled={upload.isPending} onClick={() => input.current?.click()}>
              {t('add.file.choose')}
            </button>
            <input
              ref={input}
              type="file"
              accept=".gguf"
              className="sr-only"
              aria-label={t('add.file.choose')}
              onChange={(e) => {
                send(e.target.files?.[0])
                e.target.value = ''
              }}
            />
          </>
        )}
      </div>
      {upload.isError ? <p className="text-sm text-danger">{errorText(upload.error, t('add.failed'))}</p> : null}

      <form
        className="space-y-2"
        onSubmit={(e) => {
          e.preventDefault()
          if (path.trim()) byPath.mutate()
        }}
      >
        <label className="block text-sm text-ink-muted">
          {t('add.file.pathLabel')}
          <input
            className="field mt-1 w-full font-mono text-xs"
            value={path}
            placeholder={t('add.file.pathPlaceholder')}
            onChange={(e) => setPath(e.target.value)}
            spellCheck={false}
          />
        </label>
        <fieldset className="space-y-1 text-sm">
          <legend className="sr-only">{t('add.file.how')}</legend>
          <label className="flex items-start gap-2">
            <input type="radio" name="import-how" className="mt-1" checked={!inPlace} onChange={() => setInPlace(false)} />
            <span>
              <span className="text-ink">{t('add.file.copy')}</span>
              <span className="block text-xs text-ink-faint">{t('add.file.copyHint')}</span>
            </span>
          </label>
          <label className="flex items-start gap-2">
            <input type="radio" name="import-how" className="mt-1" checked={inPlace} onChange={() => setInPlace(true)} />
            <span>
              <span className="text-ink">{t('add.file.inPlace')}</span>
              <span className="block text-xs text-ink-faint">{t('add.file.inPlaceHint')}</span>
            </span>
          </label>
        </fieldset>
        <button type="submit" className="btn-secondary btn-sm" disabled={!path.trim() || byPath.isPending}>
          {byPath.isPending ? t('add.adding') : t('add.add')}
        </button>
        {byPath.isError ? <p className="text-sm text-danger">{errorText(byPath.error, t('add.failed'))}</p> : null}
      </form>
    </div>
  )
}

function FromApps({ onAdded }: { onAdded: (r: ModelImportResult | null, name?: string) => void }) {
  const { t } = useTranslation('models')
  const found = useQuery({ queryKey: ['models-found'], queryFn: () => api.findModelsInOtherApps(), retry: false })
  const add = useMutation({
    mutationFn: (m: FoundModel) => api.importModelPath({ path: m.path, in_place: true, display_name: m.name }),
    onSuccess: (r, m) => {
      onAdded(r, m.name)
      void found.refetch()
    },
  })
  const list = found.data?.models ?? []
  if (found.isLoading) return <LoadingSpinner label={t('add.apps.looking')} />
  return (
    <div className="space-y-3">
      <p className="text-sm text-ink-muted">{t('add.apps.description')}</p>
      {list.length === 0 ? (
        <p className="rounded-xl border border-line px-4 py-3 text-sm text-ink-muted">{t('add.apps.none')}</p>
      ) : (
        <ul className="divide-y divide-line rounded-xl border border-line">
          {list.map((m) => (
            <li key={m.path} className="flex min-w-0 flex-wrap items-center justify-between gap-3 px-4 py-3">
              <div className="min-w-0">
                <p className="font-medium text-ink">{m.name}</p>
                <p className="mt-0.5 truncate text-xs text-ink-muted" title={m.path}>
                  {[APP_NAMES[m.app] ?? m.app, m.parameters, m.quantization, formatBytes(m.size_bytes)].filter(Boolean).join(' · ')}
                </p>
              </div>
              {m.model_id ? (
                <span className="text-xs text-ink-faint">{t('add.apps.already')}</span>
              ) : (
                <button
                  type="button"
                  className="btn-primary btn-sm shrink-0"
                  disabled={add.isPending}
                  onClick={() => add.mutate(m)}
                >
                  {add.isPending && add.variables?.path === m.path ? t('add.adding') : t('add.add')}
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      {found.isError ? <p className="text-sm text-danger">{errorText(found.error, t('add.failed'))}</p> : null}
      {add.isError ? <p className="text-sm text-danger">{errorText(add.error, t('add.failed'))}</p> : null}
    </div>
  )
}

function FromLink({ onAdded }: { onAdded: (r: ModelImportResult | null, name?: string) => void }) {
  const { t } = useTranslation('models')
  const [url, setUrl] = useState('')
  const [name, setName] = useState('')
  const install = useMutation({
    mutationFn: () => api.installModelFromURL({ source_url: url.trim(), display_name: name.trim() || undefined }),
    onSuccess: (r) => r && onAdded({ model_id: r.model_id, status: 'copying', details: {} }, name.trim() || undefined),
  })
  return (
    <form
      className="space-y-2"
      onSubmit={(e) => {
        e.preventDefault()
        if (url.trim()) install.mutate()
      }}
    >
      <p className="text-sm text-ink-muted">{t('add.link.description')}</p>
      <label className="block text-sm text-ink-muted">
        {t('add.link.url')}
        <input
          className="field mt-1 w-full font-mono text-xs"
          type="url"
          value={url}
          placeholder="https://huggingface.co/…/resolve/main/model-Q4_K_M.gguf"
          onChange={(e) => setUrl(e.target.value)}
          spellCheck={false}
        />
      </label>
      <label className="block text-sm text-ink-muted">
        {t('add.link.name')}
        <input className="field mt-1 w-full" value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <button type="submit" className="btn-secondary btn-sm" disabled={!url.trim() || install.isPending}>
        {install.isPending ? t('add.adding') : t('add.add')}
      </button>
      {install.isError ? <p className="text-sm text-danger">{errorText(install.error, t('add.failed'))}</p> : null}
    </form>
  )
}
