import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { Toggle } from './Toggle'

/**
 * Access from anywhere (#456, docs/remote-access.md): a listener for the
 * devices paired with this computer when they're away from home. It serves
 * only what a phone uses, over HTTPS, to their keys. For now it's reached
 * through a port the person forwards (or Tailscale); automatic setup comes
 * next.
 */
export function RemoteAccessCard() {
  const { t } = useTranslation('apiAccess')
  const queryClient = useQueryClient()
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => api.getSettings() })
  const status = useQuery({ queryKey: ['remote-access'], queryFn: () => api.getRemoteAccess(), refetchInterval: 15_000 })
  const enabled = settings.data?.remote_access_enabled ?? false
  const [port, setPort] = useState('')
  const [address, setAddress] = useState('')
  const [error, setError] = useState('')
  useEffect(() => {
    if (settings.data) {
      setPort(String(settings.data.remote_access_port ?? 7333))
      setAddress(settings.data.remote_access_address ?? '')
    }
  }, [settings.data])
  const save = useMutation({
    mutationFn: (patch: Parameters<typeof api.updateSettings>[0]) => api.updateSettings(patch),
    onSuccess: () => {
      setError('')
      void queryClient.invalidateQueries({ queryKey: ['settings'] })
      void queryClient.invalidateQueries({ queryKey: ['remote-access'] })
    },
    onError: (err) => setError(err instanceof Error ? err.message : String(err)),
  })
  const st = status.data
  const current = String(settings.data?.remote_access_port ?? 7333)
  const changed = port !== current || address.trim() !== (settings.data?.remote_access_address ?? '')
  return (
    <section className="card space-y-4" aria-labelledby="remote-access">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 id="remote-access" className="section-title">
            {t('remote.title')}
          </h2>
          <p className="mt-1 text-sm leading-relaxed text-ink-muted">{t('remote.body')}</p>
        </div>
        <Toggle
          label={t('remote.title')}
          checked={enabled}
          disabled={save.isPending || !settings.data}
          onChange={() => save.mutate({ remote_access_enabled: !enabled })}
        />
      </div>
      {enabled && st ? <ReachLine status={st} /> : null}
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={settings.data?.remote_access_port_mapping ?? true}
          disabled={save.isPending || !settings.data}
          onChange={(e) => save.mutate({ remote_access_port_mapping: e.target.checked })}
        />
        <span>{t('remote.portMapping')}</span>
      </label>
      <form
        className="grid gap-3 sm:grid-cols-[8rem_1fr_auto] sm:items-end"
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate({ remote_access_port: Number(port) || 0, remote_access_address: address.trim() })
        }}
      >
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">{t('remote.port')}</span>
          <input className="field w-full" inputMode="numeric" value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ''))} />
        </label>
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">{t('remote.address')}</span>
          <input
            className="field w-full"
            placeholder={t('remote.addressExample')}
            value={address}
            onChange={(e) => setAddress(e.target.value)}
          />
        </label>
        <button type="submit" className="btn-secondary" disabled={!changed || save.isPending}>
          {t('remote.save')}
        </button>
      </form>
      {error ? <p className="text-sm text-danger">{error}</p> : null}
      <p className="text-xs text-ink-faint">{t('remote.notes', { port: current })}</p>
    </section>
  )
}

type Status = NonNullable<Awaited<ReturnType<typeof api.getRemoteAccess>>>

/** How the internet reaches this computer, in a line. */
function ReachLine({ status }: { status: Status }) {
  const { t } = useTranslation('apiAccess')
  if (!status.listening) {
    return (
      <p className="text-sm text-danger" role="status">
        {t('remote.notListening', { port: status.port, error: status.error ?? '' })}
      </p>
    )
  }
  const by = status.mapped_by ? t(`remote.method.${status.mapped_by}`) : ''
  let line: string
  let ok = true
  switch (status.reachable) {
    case 'direct':
      line = t('remote.reach.direct', { address: status.mapped, method: by })
      break
    case 'ipv6':
      line = t('remote.reach.ipv6', { address: status.ipv6?.[0] ?? '', port: status.port })
      break
    case 'manual':
      line = t('remote.reach.manual', { address: status.address })
      break
    default:
      ok = false
      line = status.reason === 'carrier_nat' ? t('remote.reach.carrierNat') : t('remote.reach.noPort', { port: status.port })
  }
  return (
    <p className={`text-sm ${ok ? 'text-success' : 'text-warning'}`} role="status">
      {line}
    </p>
  )
}
