import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { YggdrasilMark } from '@/components/ui/YggdrasilMark'
import { api, forgetApiKey, getApiBase } from '@/lib/api'
import { useUIStore } from '@/stores/uiStore'

/** Why sign-in with the provider sent the browser back, from ?oidc_error=. */
const providerErrors = ['failed', 'refused', 'disabled', 'unavailable'] as const

/**
 * Signing in to a Toskar shared with other people (#206), with a username
 * and password from an invite link, or with an OpenID Connect provider when
 * one is set up. An API key still works instead.
 */
export function SignInForm({ onSignedIn, onUseKey }: { onSignedIn: () => void; onUseKey: () => void }) {
  const { t } = useTranslation('people')
  const setOnboardingComplete = useUIStore((s) => s.setOnboardingComplete)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const provider = useQuery({ queryKey: ['oidc'], queryFn: () => api.getOIDC(), retry: false, staleTime: 60_000 })
  const providerError = new URLSearchParams(window.location.search).get('oidc_error')
  const providerMessage = providerErrors.find((e) => e === providerError)
  const providerName = provider.data?.label ?? ''
  // Back to a chat portal a Member signed in for (#205), or where they were.
  const next = new URLSearchParams(window.location.search).get('next') ?? ''
  const portalNext = /^\/p\/[a-z0-9-]+$/.test(next) ? next : ''
  const returnTo = portalNext || (window.location.pathname === '/' ? '/chat' : window.location.pathname)
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
          if (portalNext) {
            window.location.assign(portalNext)
            return
          }
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
        {providerMessage ? (
          <p className="text-sm text-danger" role="alert">
            {t(`signIn.provider.${providerMessage}`, { name: providerName })}
          </p>
        ) : null}
        {provider.data?.enabled ? (
          <>
            <a className="btn-primary text-center" href={`${getApiBase()}/api/v1/oidc/start?return=${encodeURIComponent(returnTo)}`}>
              {t('signIn.withProvider', { name: providerName })}
            </a>
            <p className="text-center text-xs text-ink-faint">{t('signIn.or')}</p>
          </>
        ) : null}
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
        <button type="submit" className={provider.data?.enabled ? 'btn-secondary' : 'btn-primary'} disabled={busy || !username.trim() || !password}>
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
