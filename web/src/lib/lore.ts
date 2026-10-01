import type { Realm, RuneId } from './realms'

/** A short lore entry, shown in a bubble when a Norse name, the mascot, or the logo is clicked. */
export interface Lore {
  /** The name, such as "Ratatoskr". */
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

export const MASCOT_LORE: Lore = {
  name: 'Ratatoskr',
  kind: 'The Yggdrasil mascot',
  story:
    'In the myths, Ratatoskr is the squirrel who runs up and down the World Tree, carrying messages between the eagle at the top and the dragon Níðhöggr at the roots. He was known to stir up a little trouble along the way.',
  here: 'He does the same job here, carrying your requests between your apps and the models on your computers. He thinks while a reply is written, celebrates when something finishes, and naps when no model is loaded.',
}

export const LOGO_LORE: Lore = {
  name: 'Yggdrasil',
  kind: 'The World Tree',
  story:
    'Yggdrasil is the great ash at the center of the Norse cosmos. Its branches reach over the heavens, its roots reach three wells, and the nine worlds hang among its boughs.',
  here: 'The mark draws the tree as circuit traces, with roots and branches ending in nodes, like the computers Yggdrasil connects into one system.',
}

/** The lore entry for a page's Norse name. */
export function realmLore(realm: Realm): Lore {
  return {
    name: realm.norse,
    kind: realm.runeName,
    story: realm.story,
    here: realm.meaning,
    rune: realm.rune,
    runeName: realm.runeName,
  }
}
