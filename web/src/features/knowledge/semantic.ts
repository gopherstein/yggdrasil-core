import i18n from '@/i18n'
import type { KnowledgeSource, Model } from '@/types/api'
import { languageLevelFor, languageName } from '@/features/models/modelPresentation'

// meaningNote says whether a source can be searched by meaning, which needs
// an installed embedding model. Without one it returns null: search matches
// words, as always.
export function meaningNote(source: KnowledgeSource): string | null {
  const embedded = source.embedded_count ?? 0
  if (embedded === 0 || source.chunk_count === 0) return null
  if (embedded >= source.chunk_count) return i18n.t('knowledge:meaning.full')
  return i18n.t('knowledge:meaning.partial', { embedded, total: source.chunk_count })
}

/** The languages a source is written in, named in the App language, at most three. */
export function sourceLanguages(source: KnowledgeSource): string {
  const langs = source.languages ?? []
  const names = langs.slice(0, 3).map((l) => languageName(l.language))
  if (langs.length > 3) names.push('…')
  return names.join(', ')
}

/**
 * A word when a source is in another language than the App language and the
 * embedding model in use can't bridge the two (spec §19): questions find it
 * by words only, and a multilingual embedding model would find it by meaning.
 */
export function crossLanguageNote(source: KnowledgeSource, appLanguage: string, models: Model[]): string | null {
  const lang = source.language
  if (!lang || lang.split('-')[0] === appLanguage.split('-')[0] || appLanguage.startsWith('en-X') || appLanguage.startsWith('ar-X')) return null
  const embedder = models.find((m) => m.id === source.embedding_model)
  if (embedder && languageLevelFor(embedder, lang) && languageLevelFor(embedder, appLanguage)) return null
  return i18n.t('knowledge:languages.crossNote', { source: languageName(lang), yours: languageName(appLanguage) })
}
