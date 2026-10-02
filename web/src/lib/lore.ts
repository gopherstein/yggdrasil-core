import i18n from '@/i18n'
import type { Realm, RuneId } from './realms'

/** A short lore entry, shown in a bubble when a Norse name, the mascot, or the logo is clicked. */
export interface Lore {
  /** The name, such as "Ratatoskr"; the same in every language. */
  name: string
  /** What it is, in a few words, shown under the name. */
  kind: string
  /** Who or what it is in the myths. */
  story: string
  /** What it is in Yggdrasil. */
  here: string
  rune?: RuneId
  runeName?: string
}

// The text is in the lore catalog (i18n/locales/<language>/lore.json), read
// when shown so it follows the App language.

export function mascotLore(): Lore {
  return {
    name: 'Ratatoskr',
    kind: i18n.t('lore:mascot.kind'),
    story: i18n.t('lore:mascot.story'),
    here: i18n.t('lore:mascot.here'),
  }
}

export function logoLore(): Lore {
  return {
    name: 'Yggdrasil',
    kind: i18n.t('lore:logo.kind'),
    story: i18n.t('lore:logo.story'),
    here: i18n.t('lore:logo.here'),
  }
}

/** The lore entry for a page's Norse name. */
export function realmLore(realm: Realm): Lore {
  const runeName = i18n.t(`lore:realms.${realm.id}.rune`)
  return {
    name: realm.norse,
    kind: runeName,
    story: i18n.t(`lore:realms.${realm.id}.story`),
    here: i18n.t(`lore:realms.${realm.id}.meaning`),
    rune: realm.rune,
    runeName,
  }
}
