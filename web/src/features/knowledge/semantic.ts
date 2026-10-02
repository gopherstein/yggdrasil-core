import i18n from '@/i18n'
import type { KnowledgeSource } from '@/types/api'

// meaningNote says whether a source can be searched by meaning, which needs
// an installed embedding model. Without one it returns null: search matches
// words, as always.
export function meaningNote(source: KnowledgeSource): string | null {
  const embedded = source.embedded_count ?? 0
  if (embedded === 0 || source.chunk_count === 0) return null
  if (embedded >= source.chunk_count) return i18n.t('knowledge:meaning.full')
  return i18n.t('knowledge:meaning.partial', { embedded, total: source.chunk_count })
}
