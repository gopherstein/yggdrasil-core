import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { YggdrasilMark } from '@/components/ui/YggdrasilMark'
import { api, forgetApiKey } from '@/lib/api'
import { useUIStore } from '@/stores/uiStore'

/**
 * Signing in to a Toskar shared with other people (#206), with a username
 * and password from an invite link. An API key still works instead.
 */
export function SignInForm({ onSignedIn, onUseKey }: { onSignedIn: () => void; onUseKey: () => void }) {
  const { t } = useTranslation('people')
  const setOnboardingComplete = useUIStore((s) => s.setOnboardingComplete)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <form
      className="flex h-full min-h-0 flex-col items-center justify-center bg-canvas px-6 text-center"
      onSubmit={async (e) => {
        e.preventDefault()
        setBusy(true)
        setError('')
        try {
          await api.signIn(username.trim(), password)
          // The session speaks for this person; a stored key would speak
          // for whoever made it.
          forgetApiKey()
          setOnboardingComplete(true)
          setPassword('')
          onSignedIn()
        } catch (err) {
          setError(err instanceof Error ? err.message : String(err))
        } finally {
          setBusy(false)
        }
      }}
    >
      <YggdrasilMark size={64} lore />
      <h1 className="mt-6 font-display text-2xl font-semibold tracking-tight text-ink">{t('signIn.title')}</h1>
      <p className="mt-2 max-w-md text-sm leading-relaxed text-ink-muted">{t('signIn.body')}</p>
      <div className="mt-6 flex w-full max-w-sm flex-col gap-3 text-start">
        <label className="space-y-1 text-sm">
          <span className="text-ink-muted">{t('signIn.username')}</span>
          <input
            className="field w-full"
            autoComplete="username"
            autoCapitalize="none"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
          />
        </label>
        <label className="space-y-1 text-sm">
          <span className="text-ink-muted">{t('signIn.password')}</span>
          <input
            className="field w-full"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        {error ? (
          <p className="text-sm text-danger" role="alert">
            {error}
          </p>
        ) : null}
        <button type="submit" className="btn-primary" disabled={busy || !username.trim() || !password}>
          {t('signIn.submit')}
        </button>
        <p className="text-center text-xs text-ink-faint">{t('signIn.forgot')}</p>
        <button type="button" className="text-center text-xs text-ink-muted hover:text-ink" onClick={onUseKey}>
          {t('signIn.useKey')}
        </button>
      </div>
    </form>
  )
}
