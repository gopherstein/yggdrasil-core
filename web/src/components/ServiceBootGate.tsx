import { useQuery } from '@tanstack/react-query'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { ScheduleBackgroundSync } from '@/components/ScheduleBackgroundSync'
import { ApiError, api, rememberApiKey } from '@/lib/api'
import { readScreenshotLaunch } from '@/lib/screenshotMode'
import { Ratatoskr } from '@/components/ui/Ratatoskr'
import { YggdrasilMark } from '@/components/ui/YggdrasilMark'

const BOOT_GIVE_UP_MS = 25_000

function BootSplash() {
  const { t } = useTranslation()
  return (
    <div
      className="flex h-full min-h-0 flex-col items-center justify-center bg-canvas px-6"
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <Ratatoskr state="idle" size={160} />
      <p className="mt-4 font-display text-xl font-semibold tracking-tight text-ink">
        {t('boot.starting')}
      </p>
      <p className="mt-2 text-sm text-ink-muted">{t('boot.preparing')}</p>
      <span
        className="mt-6 inline-block h-5 w-5 animate-spin rounded-full border-2 border-line border-t-primary"
        aria-hidden
      />
    </div>
  )
}

function BootFailed({ onRetry, busy }: { onRetry: () => void; busy: boolean }) {
  const { t } = useTranslation()
  return (
    <div className="flex h-full min-h-0 flex-col items-center justify-center bg-canvas px-6 text-center">
      <Ratatoskr state="error" size={96} />
      <h1 className="mt-4 font-display text-2xl font-semibold tracking-tight text-ink">
        {t('boot.unavailable')}
      </h1>
      <p className="mt-2 max-w-md text-sm leading-relaxed text-ink-muted">{t('boot.unavailableBody')}</p>
      <button
        type="button"
        className="btn-primary mt-6"
        disabled={busy}
        onClick={onRetry}
      >
        {busy ? t('boot.retrying') : t('boot.tryAgain')}
      </button>
    </div>
  )
}

/**
 * Blocks the main shell until the local daemon answers /api/v1/health.
 * Screenshot mode paints the shell immediately with the demo API.
 */
export function ServiceBootGate({ children }: { children: ReactNode }) {
  if (readScreenshotLaunch()?.enabled) {
    return <>{children}</>
  }
  return <DaemonBootGate>{children}</DaemonBootGate>
}

function needsApiKey(error: unknown): boolean {
  return error instanceof ApiError && (error.status === 401 || error.code === 'UNAUTHORIZED')
}

function DaemonBootGate({ children }: { children: ReactNode }) {
  const [deadline, setDeadline] = useState(() => Date.now() + BOOT_GIVE_UP_MS)
  const [timedOut, setTimedOut] = useState(false)
  const [keyDraft, setKeyDraft] = useState('')
  const { t } = useTranslation()

  const healthQuery = useQuery({
    queryKey: ['health'],
    queryFn: async () => {
      const health = await api.getHealth()
      if (!health || health.status !== 'ok') {
        throw new Error('Service not ready')
      }
      return health
    },
    retry: (_count, error) => !needsApiKey(error),
    retryDelay: (attempt) => Math.min(400 + attempt * 250, 2000),
    refetchInterval: (query) =>
      query.state.data?.status === 'ok' ? 15_000 : needsApiKey(query.state.error) ? false : 1_000,
    refetchOnWindowFocus: true,
  })

  const ready = healthQuery.data?.status === 'ok'
  const askForKey = needsApiKey(healthQuery.error)

  useEffect(() => {
    if (ready) {
      setTimedOut(false)
      return
    }
    const id = window.setInterval(() => {
      if (Date.now() >= deadline) {
        setTimedOut(true)
      }
    }, 400)
    return () => window.clearInterval(id)
  }, [ready, deadline])

  if (ready) {
    return (
      <div className="h-full min-h-0 min-w-0 overflow-hidden">
        <ScheduleBackgroundSync />
        {children}
      </div>
    )
  }

  if (askForKey) {
    return (
      <form
        className="flex h-full min-h-0 flex-col items-center justify-center bg-canvas px-6 text-center"
        onSubmit={(event) => {
          event.preventDefault()
          const secret = keyDraft.trim()
          if (!secret) return
          rememberApiKey(secret)
          setKeyDraft('')
          void healthQuery.refetch()
        }}
      >
        <YggdrasilMark size={64} lore />
        <h1 className="mt-6 font-display text-2xl font-semibold tracking-tight text-ink">
          {t('boot.keyRequired')}
        </h1>
        <p className="mt-2 max-w-md text-sm leading-relaxed text-ink-muted">{t('boot.keyBody')}</p>
        <input
          type="password"
          autoComplete="off"
          value={keyDraft}
          onChange={(event) => setKeyDraft(event.target.value)}
          // eslint-disable-next-line i18next/no-literal-string -- an example of what to type, not prose
          placeholder="ygg_…"
          className="field mt-6 w-full max-w-md"
        />
        <button type="submit" className="btn-primary mt-4" disabled={!keyDraft.trim() || healthQuery.isFetching}>
          {t('boot.continue')}
        </button>
      </form>
    )
  }

  if (timedOut) {
    return (
      <BootFailed
        busy={healthQuery.isFetching}
        onRetry={() => {
          setTimedOut(false)
          setDeadline(Date.now() + BOOT_GIVE_UP_MS)
          void healthQuery.refetch()
        }}
      />
    )
  }

  return <BootSplash />
}
