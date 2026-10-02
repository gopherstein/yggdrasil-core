import { afterEach, describe, expect, it } from 'vitest'
import { applyLanguage } from '@/i18n'
import type { KnowledgeSource, Model } from '@/types/api'
import { crossLanguageNote, sourceLanguages } from './semantic'

afterEach(async () => {
  await applyLanguage('en')
  localStorage.clear()
})

const source = (over: Partial<KnowledgeSource>): KnowledgeSource =>
  ({ id: 's', name: 'policies.md', kind: 'text', status: 'ready', chunk_count: 4, ...over }) as KnowledgeSource

const embedder = (id: string, tags: string[]): Model =>
  ({
    id,
    display_name: id,
    installed: true,
    capabilities: { tool_calling: false, vision: false, coding: false },
    languages: tags.map((language) => ({ language, level: 'good' as const, confidence: 'medium' as const, sources: ['model_card'] })),
  }) as Model

describe('knowledge languages', () => {
  it('names a source’s languages in the App language', () => {
    expect(sourceLanguages(source({ languages: [{ language: 'es', passages: 3 }, { language: 'en', passages: 1 }] }))).toBe('Spanish, English')
    expect(sourceLanguages(source({}))).toBe('')
  })

  it('suggests a multilingual embedding model only when the one in use can’t bridge the languages', () => {
    const spanish = source({ language: 'es', embedding_model: 'nomic' })
    const models = [embedder('nomic', ['en']), embedder('bge-m3-q8', ['en', 'es'])]
    expect(crossLanguageNote(spanish, 'en', models)).toContain('BGE-M3')
    expect(crossLanguageNote({ ...spanish, embedding_model: 'bge-m3-q8' }, 'en', models)).toBeNull()
    // A source in the App language needs nothing.
    expect(crossLanguageNote(source({ language: 'en', embedding_model: 'nomic' }), 'en', models)).toBeNull()
  })
})
