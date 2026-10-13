import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import { formatMilliseconds } from '@/i18n/format'
import type { FirstAnswersGroup } from '@/types/api'

const CLIENTS = ['phone', 'tablet', 'watch', 'tv', 'desktop', 'web', 'other'] as const

/** One row per app, with its warm and cold groups side by side, apps in a fixed order. */
export function rowsByClient(groups: FirstAnswersGroup[]) {
  const rows = new Map<string, { warm?: FirstAnswersGroup; cold?: FirstAnswersGroup }>()
  for (const g of groups) {
    const row = rows.get(g.client) ?? {}
    row[g.cold ? 'cold' : 'warm'] = g
    rows.set(g.client, row)
  }
  const rank = (client: string) => {
    const i = (CLIENTS as readonly string[]).indexOf(client)
    return i === -1 ? CLIENTS.length : i
  }
  return [...rows.entries()].sort(([a], [b]) => rank(a) - rank(b)).map(([client, row]) => ({ client, ...row }))
}

function Cell({ group, cold }: { group?: FirstAnswersGroup; cold: boolean }) {
  const { t } = useTranslation('diagnostics')
  if (!group) return <td className="py-2 pe-4 text-ink-faint">—</td>
  return (
    <td className="py-2 pe-4">
      <span className="tabular-nums text-ink">{formatMilliseconds(group.first_word_ms)}</span>
      <span className="block text-xs text-ink-muted">
        {cold && group.load_ms
          ? t('firstAnswers.coldDetail', { count: group.runs, load: formatMilliseconds(group.load_ms) })
          : t('firstAnswers.answers', { count: group.runs })}
      </span>
    </td>
  )
}

/**
 * How long people waited for the first word of an answer over the last week
 * (#498), by the app they asked from, with every model already running and
 * with one loading first.
 */
export function FirstAnswersPanel() {
  const { t } = useTranslation('diagnostics')
  const query = useQuery({ queryKey: ['first-answers'], queryFn: () => api.getFirstAnswers(), refetchInterval: 60_000 })
  // An older daemon, or a week with no answers, has nothing to show.
  const groups = Array.isArray(query.data?.groups) ? query.data.groups : []
  if (groups.length === 0) return null
  const rows = rowsByClient(groups)
  return (
    <section className="card space-y-3" aria-labelledby="first-answers-title">
      <div>
        <h2 id="first-answers-title" className="section-title">
          {t('firstAnswers.title')}
        </h2>
        <p className="mt-1 text-sm text-ink-muted">{t('firstAnswers.description')}</p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-ink-muted">
            <tr>
              <th scope="col" className="py-1 pe-4 text-start font-normal">{t('firstAnswers.app')}</th>
              <th scope="col" className="py-1 pe-4 text-start font-normal">{t('firstAnswers.warm')}</th>
              <th scope="col" className="py-1 pe-4 text-start font-normal">{t('firstAnswers.cold')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.client} className="border-t border-line/50">
                <th scope="row" className="py-2 pe-4 text-start font-medium text-ink">
                  {t(`firstAnswers.clients.${row.client === '' ? 'unnamed' : (CLIENTS as readonly string[]).includes(row.client) ? row.client : 'other'}`)}
                </th>
                <Cell group={row.warm} cold={false} />
                <Cell group={row.cold} cold />
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
