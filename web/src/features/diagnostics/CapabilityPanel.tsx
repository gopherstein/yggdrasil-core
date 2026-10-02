import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import i18n from '@/i18n'
import { api } from '@/lib/api'

function bytesText(b: number): string {
  if (b >= 1 << 30) return i18n.t('diagnostics:abilities.gigabytes', { value: (b / (1 << 30)).toFixed(1) })
  if (b >= 1 << 20) return i18n.t('diagnostics:abilities.megabytes', { value: (b / (1 << 20)).toFixed(1) })
  return i18n.t('diagnostics:abilities.kilobytes', { value: Math.round(b / 1024) })
}

/**
 * What Yggdrasil can do right now (spec §37): each ability, how it works
 * or what would make it possible, and what it is built from.
 */
export function CapabilityPanel() {
  const { t } = useTranslation('diagnostics')
  const query = useQuery({ queryKey: ['capabilities'], queryFn: () => api.getCapabilities(), retry: false, staleTime: 15_000 })
  const snap = query.data
  // A daemon too old for the inventory, or a malformed reply, hides the
  // panel instead of failing the whole Diagnostics page.
  if (!snap || !Array.isArray(snap.abilities)) return null
  const list = <T,>(v: T[] | undefined): T[] => (Array.isArray(v) ? v : [])
  const models = list(snap.models)
  const nodes = list(snap.nodes)
  const online = nodes.filter((n) => n.online).length
  const tools = list(snap.tools).filter((t) => t.enabled).length
  const connected = list(snap.connectors).filter((c) => c.connected).length
  const providers = list(snap.providers)
  const healthy = providers.filter((p) => p.healthy).length
  return (
    <section className="card space-y-3">
      <div>
        <h2 className="section-title">{t('abilities.title')}</h2>
        <p className="mt-1 text-sm text-ink-muted">{t('abilities.description')}</p>
      </div>
      <ul className="grid gap-x-4 gap-y-1.5 text-sm sm:grid-cols-2">
        {snap.abilities.map((a) => (
          <li key={a.id} className="flex items-start gap-2">
            <span aria-hidden className={a.available ? 'text-success' : 'text-ink-faint'}>
              {a.available ? '✓' : '–'}
            </span>
            <span className="min-w-0">
              <span className={a.available ? 'text-ink' : 'text-ink-muted'}>{a.label}</span>
              <span className="sr-only">{a.available ? t('abilities.available') : t('abilities.notAvailable')}</span>
              {(a.via?.length || a.note) && (
                <span className="block text-xs text-ink-faint">
                  {a.via?.length ? t('abilities.via', { via: a.via.join(', ') }) : ''}
                  {a.note}
                </span>
              )}
            </span>
          </li>
        ))}
      </ul>
      <p className="text-xs text-ink-faint">
        {t('abilities.summary', {
          models: t('abilities.models', { count: models.length }),
          computers: t('abilities.computers', { online, total: nodes.length }),
          tools: t('abilities.tools', { count: tools }),
          services: t('abilities.services', { count: connected }),
          providers: t('abilities.providers', { healthy, total: providers.length }),
          files: t('abilities.files', { count: snap.artifacts?.count ?? 0, size: bytesText(snap.artifacts?.bytes ?? 0) }),
        })}
      </p>
    </section>
  )
}
