import { useTranslation } from 'react-i18next'
import i18n from '@/i18n'
import { formatSize } from '@/i18n/format'
import { useEffect, useState } from 'react'
import { artifactObjectUrl, downloadArtifact } from '@/lib/api'
import { isAudioName, isImageName } from '@/lib/upload'
import type { FileRef } from '@/types/api'


// A file's kind, as chat:attachments.kinds.<kind> names it.
const knownKinds = ['spreadsheet', 'pdf', 'document', 'code', 'image', 'audio', 'video']

function kindLabel(kind: string): string {
  return i18n.t(`chat:attachments.kinds.${knownKinds.includes(kind) ? kind : 'other'}`)
}

function FileIcon({ kind }: { kind: string }) {
  const tone =
    kind === 'spreadsheet' ? 'text-success' : kind === 'pdf' ? 'text-danger' : kind === 'code' ? 'text-bifrost' : 'text-mimir'
  if (kind === 'video') {
    return (
      <svg viewBox="0 0 16 16" className="h-4 w-4 shrink-0 text-bifrost" fill="none" stroke="currentColor" strokeWidth={1.4} aria-hidden>
        <rect x="1.5" y="3" width="10" height="10" rx="1.5" />
        <path d="M11.5 6.5l3-1.5v6l-3-1.5" strokeLinejoin="round" />
      </svg>
    )
  }
  if (kind === 'image') {
    return (
      <svg viewBox="0 0 16 16" className="h-4 w-4 shrink-0 text-bifrost" fill="none" stroke="currentColor" strokeWidth={1.4} aria-hidden>
        <rect x="1.5" y="2.5" width="13" height="11" rx="1.5" />
        <circle cx="5.5" cy="6" r="1.2" />
        <path d="M2 12l3.5-3.5 2.5 2.5 2-2 4 4" strokeLinejoin="round" />
      </svg>
    )
  }
  if (kind === 'audio') {
    return (
      <svg viewBox="0 0 16 16" className="h-4 w-4 shrink-0 text-norn" fill="none" stroke="currentColor" strokeWidth={1.4} aria-hidden>
        <path d="M2.5 6v4M5.5 3.5v9M8.5 5.5v5M11.5 2.5v11M14 6.5v3" strokeLinecap="round" />
      </svg>
    )
  }
  return (
    <svg viewBox="0 0 16 16" className={`h-4 w-4 shrink-0 ${tone}`} fill="none" stroke="currentColor" strokeWidth={1.4} aria-hidden>
      <path d="M4 1.5h5.5L13 5v9.5H4z" strokeLinejoin="round" />
      <path d="M9.5 1.5V5H13" strokeLinejoin="round" />
      {kind === 'spreadsheet' ? <path d="M6 8h5M6 10.5h5M8.5 7v5" /> : <path d="M6 8.5h4.5M6 11h3" />}
    </svg>
  )
}

/** A stored file. Clicking it downloads the file; audio plays instead. */
export function FileChip({ file }: { file: FileRef }) {
  if (file.kind === 'audio') return <AudioChip file={file} />
  if (file.kind === 'image') return <ImageChip file={file} />
  if (file.kind === 'video') return <VideoChip file={file} />
  return <DownloadChip file={file} />
}

function DownloadChip({ file }: { file: FileRef }) {
  const { t } = useTranslation('chat')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  return (
    <button
      type="button"
      className="inline-flex max-w-[18rem] items-center gap-2 rounded-lg border border-line/70 bg-surface px-2.5 py-1.5 text-left text-xs text-ink transition hover:border-primary/50"
      title={error ?? t('attachments.download', { name: file.name })}
      disabled={busy}
      onClick={async () => {
        setBusy(true)
        setError(null)
        try {
          await downloadArtifact(file)
        } catch (err) {
          setError(err instanceof Error ? err.message : t('attachments.downloadFailed'))
        } finally {
          setBusy(false)
        }
      }}
    >
      <FileIcon kind={file.kind} />
      <span className="min-w-0">
        <span className="block truncate font-medium">{file.name}</span>
        <span className={`block text-[11px] ${error ? 'text-danger' : 'text-ink-faint'}`}>
          {error ??
            `${t('attachments.meta', { kind: kindLabel(file.kind), size: formatSize(file.size_bytes) })}${busy ? ` · ${t('attachments.downloading')}` : ''}`}
        </span>
      </span>
    </button>
  )
}

/** An audio file: Play loads it and shows a player; it can still be downloaded. */
export function AudioChip({ file, autoPlay = false }: { file: FileRef; autoPlay?: boolean }) {
  const { t } = useTranslation('chat')
  const [url, setUrl] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = async () => {
    setBusy(true)
    setError(null)
    try {
      setUrl(await artifactObjectUrl(file.id))
    } catch (err) {
      setError(err instanceof Error ? err.message : t('attachments.audioFailed'))
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    if (autoPlay) void load()
    // Load once, when the chip first appears.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  useEffect(() => () => {
    if (url) URL.revokeObjectURL(url)
  }, [url])

  return (
    <span className="inline-flex max-w-full flex-col gap-1.5 rounded-lg border border-line/70 bg-surface px-2.5 py-1.5 text-xs text-ink">
      <span className="flex items-center gap-2">
        <FileIcon kind="audio" />
        <span className="min-w-0 flex-1">
          <span className="block truncate font-medium">{file.name}</span>
          <span className={`block text-[11px] ${error ? 'text-danger' : 'text-ink-faint'}`}>
            {error ??
              `${t('attachments.meta', { kind: kindLabel('audio'), size: formatSize(file.size_bytes) })}${busy ? ` · ${t('attachments.loading')}` : ''}`}
          </span>
        </span>
        {url ? null : (
          <button
            type="button"
            className="shrink-0 rounded-md border border-line/70 px-2 py-0.5 text-ink-muted transition hover:border-primary/50 hover:text-ink"
            disabled={busy}
            onClick={() => void load()}
          >
            {t('attachments.play')}
          </button>
        )}
        <button
          type="button"
          className="shrink-0 rounded-md px-1.5 py-0.5 text-ink-faint transition hover:text-ink"
          title={t('attachments.download', { name: file.name })}
          aria-label={t('attachments.download', { name: file.name })}
          onClick={() =>
            void downloadArtifact(file).catch((err: unknown) =>
              setError(err instanceof Error ? err.message : t('attachments.downloadFailed')),
            )
          }
        >
          ↓
        </button>
      </span>
      {url ? <audio controls autoPlay src={url} className="h-8 w-64 max-w-full" aria-label={file.name} /> : null}
    </span>
  )
}

/** An image, shown in the chat; it can be downloaded. */
export function ImageChip({ file }: { file: FileRef }) {
  const { t } = useTranslation('chat')
  const [url, setUrl] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    let made: string | null = null
    artifactObjectUrl(file.id)
      .then((u) => {
        made = u
        if (cancelled) URL.revokeObjectURL(u)
        else setUrl(u)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t('attachments.imageFailed')))
    return () => {
      cancelled = true
      if (made) URL.revokeObjectURL(made)
    }
  }, [file.id])

  return (
    <span className="inline-flex max-w-full flex-col gap-1.5 rounded-lg border border-line/70 bg-surface p-1.5 text-xs text-ink">
      {url ? (
        <img src={url} alt={file.name} className="max-h-80 w-auto max-w-full rounded-md object-contain sm:max-w-[24rem]" />
      ) : (
        <span className="flex h-24 w-40 items-center justify-center rounded-md bg-raised text-ink-faint">{error ? t('attachments.notAvailable') : t('attachments.loading')}</span>
      )}
      <span className="flex items-center gap-2 px-1">
        <FileIcon kind="image" />
        <span className={`min-w-0 flex-1 truncate ${error ? 'text-danger' : ''}`} title={error ?? file.name}>
          {error ?? file.name}
        </span>
        <button
          type="button"
          className="shrink-0 rounded-md px-1.5 py-0.5 text-ink-faint transition hover:text-ink"
          title={t('attachments.download', { name: file.name })}
          aria-label={t('attachments.download', { name: file.name })}
          onClick={() => void downloadArtifact(file).catch((err: unknown) => setError(err instanceof Error ? err.message : t('attachments.downloadFailed')))}
        >
          ↓
        </button>
      </span>
    </span>
  )
}

/** A video, which plays in the chat; it can be downloaded. */
export function VideoChip({ file }: { file: FileRef }) {
  const { t } = useTranslation('chat')
  const [url, setUrl] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    let made: string | null = null
    artifactObjectUrl(file.id)
      .then((u) => {
        made = u
        if (cancelled) URL.revokeObjectURL(u)
        else setUrl(u)
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : t('attachments.videoFailed')))
    return () => {
      cancelled = true
      if (made) URL.revokeObjectURL(made)
    }
  }, [file.id, t])

  return (
    <span className="inline-flex max-w-full flex-col gap-1.5 rounded-lg border border-line/70 bg-surface p-1.5 text-xs text-ink">
      {url ? (
        <video src={url} controls loop playsInline className="max-h-80 w-auto max-w-full rounded-md sm:max-w-[28rem]" aria-label={file.name} />
      ) : (
        <span className="flex h-24 w-40 items-center justify-center rounded-md bg-raised text-ink-faint">
          {error ? t('attachments.notAvailable') : t('attachments.loading')}
        </span>
      )}
      <span className="flex items-center gap-2 px-1">
        <FileIcon kind="video" />
        <span className={`min-w-0 flex-1 truncate ${error ? 'text-danger' : ''}`} title={error ?? file.name}>
          {error ?? file.name}
        </span>
        <button
          type="button"
          className="shrink-0 rounded-md px-1.5 py-0.5 text-ink-faint transition hover:text-ink"
          title={t('attachments.download', { name: file.name })}
          aria-label={t('attachments.download', { name: file.name })}
          onClick={() => void downloadArtifact(file).catch((err: unknown) => setError(err instanceof Error ? err.message : t('attachments.downloadFailed')))}
        >
          ↓
        </button>
      </span>
    </span>
  )
}

/** A file waiting in the composer: uploading, ready, or failed. */
export interface PendingFile {
  key: string
  name: string
  size: number
  status: 'uploading' | 'ready' | 'error'
  /** Set once the upload finishes. */
  file?: FileRef
  error?: string
}

export function PendingFileChip({ file, onRemove }: { file: PendingFile; onRemove: () => void }) {
  const { t } = useTranslation('chat')
  return (
    <span
      className={[
        'inline-flex max-w-[18rem] items-center gap-2 rounded-lg border bg-surface px-2.5 py-1.5 text-xs',
        file.status === 'error' ? 'border-danger/60' : 'border-line/70',
      ].join(' ')}
      title={file.error ?? file.name}
    >
      <FileIcon
        kind={file.name.toLowerCase().endsWith('.pdf') ? 'pdf' : isAudioName(file.name) ? 'audio' : isImageName(file.name) ? 'image' : 'document'}
      />
      <span className="min-w-0">
        <span className="block truncate font-medium text-ink">{file.name}</span>
        <span className={`block truncate text-[11px] ${file.status === 'error' ? 'text-danger' : 'text-ink-faint'}`}>
          {file.status === 'uploading' ? t('attachments.adding') : file.status === 'error' ? file.error : formatSize(file.size)}
        </span>
      </span>
      <button
        type="button"
        className="ml-1 shrink-0 rounded px-1 text-ink-faint hover:text-ink"
        aria-label={t('attachments.remove', { name: file.name })}
        onClick={onRemove}
      >
        ×
      </button>
    </span>
  )
}
