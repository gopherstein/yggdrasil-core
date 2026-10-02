import { useState } from 'react'
import { api } from '@/lib/api'
import type { FileRef } from '@/types/api'
import { AudioChip } from './FileChips'
import { speakableText } from './speakableText'

/** Reads an answer aloud on this computer and plays it. */
export function ReadAloudButton({ text, conversationId }: { text: string; conversationId?: string }) {
  const [audio, setAudio] = useState<FileRef | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (audio) return <AudioChip file={audio} autoPlay />
  return (
    <span className="inline-flex items-center gap-2">
      <button
        type="button"
        className="inline-flex items-center gap-1.5 rounded-full border border-line/70 bg-surface px-2.5 py-1 text-xs text-ink-muted transition hover:border-primary/50 hover:text-ink disabled:opacity-60"
        disabled={busy}
        title="Read this answer aloud on this computer"
        onClick={async () => {
          setBusy(true)
          setError(null)
          try {
            const res = await api.readAloud(speakableText(text), conversationId)
            if (res) setAudio(res.artifact)
          } catch (err) {
            setError(err instanceof Error ? err.message : 'It could not be read aloud.')
          } finally {
            setBusy(false)
          }
        }}
      >
        <svg viewBox="0 0 16 16" className="h-3.5 w-3.5" fill="none" stroke="currentColor" strokeWidth={1.4} aria-hidden>
          <path d="M2.5 6h2.5L8.5 3v10L5 10H2.5z" strokeLinejoin="round" />
          <path d="M11 5.5a3.5 3.5 0 0 1 0 5M12.8 3.5a6 6 0 0 1 0 9" strokeLinecap="round" />
        </svg>
        {busy ? 'Reading aloud… (the first time downloads a voice)' : 'Read aloud'}
      </button>
      {error ? <span className="text-xs text-danger">{error}</span> : null}
    </span>
  )
}
