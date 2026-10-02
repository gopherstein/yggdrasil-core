import i18n from '@/i18n'
import type { KnowledgeSource } from '@/types/api'

const driverNames = { sqlite: 'SQLite', postgres: 'PostgreSQL', mysql: 'MySQL' } as const

// sourceBadge labels how a source's content reaches Mimir.
export function sourceBadge(source: KnowledgeSource): string {
  switch (source.kind) {
    case 'path':
      return i18n.t('knowledge:badge.linked')
    case 'database':
      return source.remote?.driver ? driverNames[source.remote.driver] : i18n.t('knowledge:badge.database')
    case 'api':
      return i18n.t('knowledge:badge.api')
    default:
      return i18n.t('knowledge:badge.copy')
  }
}

// sourceWhere says where a source's content comes from.
export function sourceWhere(source: KnowledgeSource): string {
  if (source.kind === 'path') return source.path ?? ''
  if (source.kind === 'database') return source.remote?.database ?? source.remote?.query ?? ''
  if (source.kind === 'api') return source.remote?.url ?? ''
  return source.filename ?? ''
}

// refreshNote explains when a database or API source is fetched again.
export function refreshNote(source: KnowledgeSource): string | null {
  const minutes = source.remote?.refresh_minutes
  if (!minutes) return null
  const every =
    minutes % 1440 === 0
      ? i18n.t('knowledge:refresh.days', { count: minutes / 1440 })
      : minutes % 60 === 0
        ? i18n.t('knowledge:refresh.hours', { count: minutes / 60 })
        : i18n.t('knowledge:refresh.minutes', { count: minutes })
  return i18n.t('knowledge:refresh.note', { every })
}
