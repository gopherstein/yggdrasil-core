/**
 * Languages answers can be written in: the spec's three tiers (§27). Models
 * write many more languages than the app's menus come in, so this list is
 * longer than the App language's.
 */
export const answerLanguages = [
  'en', 'de', 'es', 'fr', 'it', 'pt-BR', 'pt-PT', 'ja', 'ko', 'zh-Hans', 'zh-Hant',
  'nl', 'pl', 'sv', 'nb', 'da', 'fi', 'cs', 'tr', 'uk', 'ru', 'id', 'vi', 'th', 'hi',
  'ar', 'he', 'fa', 'ur',
]

/** A language's name in itself, such as Deutsch, from Intl. */
export function nameInItself(tag: string): string {
  try {
    const name = new Intl.DisplayNames([tag], { type: 'language' }).of(tag) ?? tag
    return name.charAt(0).toLocaleUpperCase(tag) + name.slice(1)
  } catch {
    return tag
  }
}
