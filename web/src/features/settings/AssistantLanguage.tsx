import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { directionOf } from '@/i18n'
import { api } from '@/lib/api'
import type { AssistantLanguageMode } from '@/types/api'

/**
 * Languages answers can be written in: the spec's three tiers (§27). Models
 * write many more languages than the app's menus come in, so this list is
 * longer than the App language's.
 */
const LANGUAGES = [
  'en', 'de', 'es', 'fr', 'it', 'pt-BR', 'pt-PT', 'ja', 'ko', 'zh-Hans', 'zh-Hant',
  'nl', 'pl', 'sv', 'nb', 'da', 'fi', 'cs', 'tr', 'uk', 'ru', 'id', 'vi', 'th', 'hi',
  'ar', 'he', 'fa', 'ur',
]

/** A language's name in itself, such as Deutsch, from Intl. */
function nameInItself(tag: string): string {
  try {
    const name = new Intl.DisplayNames([tag], { type: 'language' }).of(tag) ?? tag
    return name.charAt(0).toLocaleUpperCase(tag) + name.slice(1)
  } catch {
    return tag
  }
}

/**
 * The assistant language (multilingual spec §11, §30): the language answers
 * are written in, apart from the App language. A language asked for in a
 * message, such as "answer in English", always wins.
 */
export function AssistantLanguage() {
  const { t } = useTranslation('settings')
  const queryClient = useQueryClient()
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => api.getSettings(), retry: false })
  const mode: AssistantLanguageMode = settings.data?.assistant_language_mode ?? 'auto'
  const language = settings.data?.assistant_language ?? ''
  const value = mode === 'language' && language ? language : mode === 'app' ? ':app' : ':auto'
  const save = useMutation({
    mutationFn: (choice: string) =>
      api.updateSettings(
        choice === ':auto'
          ? { assistant_language_mode: 'auto' }
          : choice === ':app'
            ? { assistant_language_mode: 'app' }
            : { assistant_language_mode: 'language', assistant_language: choice },
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['settings'] }),
  })
  const choices = LANGUAGES.includes(language) || !language ? LANGUAGES : [...LANGUAGES, language]

  return (
    <div className="space-y-1">
      <label className="block text-sm">
        <span className="text-ink-muted">{t('language.assistantLanguage')}</span>
        <select
          className="field mt-1 w-full sm:w-80"
          value={value}
          disabled={settings.isPending || save.isPending}
          onChange={(e) => save.mutate(e.target.value)}
        >
          <option value=":auto">{t('language.assistantAuto')}</option>
          <option value=":app">{t('language.assistantApp')}</option>
          {choices.map((tag) => (
            <option key={tag} value={tag} lang={tag} dir={directionOf(tag)}>
              {nameInItself(tag)}
            </option>
          ))}
        </select>
      </label>
      <p className="text-xs text-ink-faint">{t('language.assistantHint')}</p>
      {save.isError ? (
        <p className="text-sm text-danger">
          {t('language.saveFailed', { error: save.error instanceof Error ? save.error.message : String(save.error) })}
        </p>
      ) : null}
    </div>
  )
}
