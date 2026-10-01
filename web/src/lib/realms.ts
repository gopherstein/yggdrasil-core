/**
 * The Norse name and Elder Futhark rune for each page. Tab labels stay in
 * plain English; the Norse name is a small heading above each page title.
 *
 * Pages backed by a subsystem use its name (see designSystem.ts). The rest
 * take a figure whose story fits the page.
 */

export type RuneId =
  | 'ansuz'
  | 'jera'
  | 'uruz'
  | 'kenaz'
  | 'laguz'
  | 'othala'
  | 'raidho'
  | 'ehwaz'
  | 'algiz'
  | 'mannaz'
  | 'tiwaz'
  | 'thurisaz'
  | 'isa'

export interface Realm {
  /** Norse name shown above the page title. */
  norse: string
  /** What the name means here: the page's job, in one sentence. */
  meaning: string
  /** A short lore entry: who or what this is in the myths. */
  story: string
  rune: RuneId
  /** Name of the rune, for the tooltip. */
  runeName: string
  /** Text color class for the name and the active rune. */
  accent: string
}

export const realms: Record<string, Realm> = {
  '/chat': {
    norse: 'Ratatoskr',
    meaning: 'Chat is where your messages are carried to the models on your computers, and the answers carried back.',
    story:
      'Ratatoskr is the squirrel who runs up and down the trunk of Yggdrasil, carrying messages between the eagle at the top of the tree and the dragon Níðhöggr gnawing at its roots.',
    rune: 'ansuz',
    runeName: 'Ansuz, the rune of speech',
    accent: 'text-ratatoskr',
  },
  '/automations': {
    norse: 'Norn',
    meaning: 'The Norns tend what is fated to happen and when. Norn runs scheduled work.',
    story:
      "The three Norns, Urðr, Verðandi and Skuld, live by the Well of Urðr beneath Yggdrasil. They water the tree's roots and shape what was, what is, and what shall be.",
    rune: 'jera',
    runeName: 'Jera, the rune of the turning year',
    accent: 'text-norn',
  },
  '/models': {
    norse: 'Ymir',
    meaning: 'Ymir is the first giant, from whom the world was made. Models are what everything else is built on.',
    story:
      'In the beginning there was Ymir, the first of the giants. When he was slain, the gods built the world from him: the earth from his flesh, the mountains from his bones, and the sky from his skull.',
    rune: 'uruz',
    runeName: 'Uruz, the rune of raw strength',
    accent: 'text-ygg',
  },
  '/train': {
    norse: 'Brokkr',
    meaning: "Brokkr is the smith who forged Mjölnir. Here you forge an AI of your own.",
    story:
      "Brokkr and his brother Sindri were dwarven smiths. In a wager with Loki they forged Mjölnir, Thor's hammer, with Brokkr working the bellows even as a fly bit him.",
    rune: 'kenaz',
    runeName: 'Kenaz, the rune of the torch and craft',
    accent: 'text-gungnir',
  },
  '/knowledge': {
    norse: 'Mimir',
    meaning: "Mimir guards the well of wisdom. Mimir keeps your knowledge searchable.",
    story:
      "Mímir guarded the well of wisdom beneath one of Yggdrasil's roots. Odin gave an eye for a drink from it, and kept Mímir's head after his death so it could go on giving counsel.",
    rune: 'laguz',
    runeName: 'Laguz, the rune of water and the well',
    accent: 'text-mimir',
  },
  '/memory': {
    norse: 'Muninn',
    meaning: "Muninn, Odin's raven of memory, keeps what you ask Yggdrasil to remember.",
    story:
      "Huginn and Muninn, Thought and Memory, are Odin's ravens. Each morning they fly over the world and return to tell him what they saw. Odin said he feared most that Muninn would not come back.",
    rune: 'othala',
    runeName: 'Othala, the rune of what is kept',
    accent: 'text-muninn',
  },
  '/nodes': {
    norse: 'Bifrost',
    meaning: 'Bifrost is the rainbow bridge between worlds. It connects your computers.',
    story:
      'Bifröst is the burning rainbow bridge between Midgard, the world of people, and Asgard, home of the gods. Heimdall keeps watch at its end.',
    rune: 'raidho',
    runeName: 'Raidho, the rune of the journey',
    accent: 'text-bifrost',
  },
  '/performance': {
    norse: 'Sleipnir',
    meaning: "Sleipnir is Odin's eight-legged horse, the fastest of all.",
    story:
      "Sleipnir is Odin's eight-legged grey horse, the swiftest of all steeds. He carries Odin across sky and sea, and once as far as Hel and back.",
    rune: 'ehwaz',
    runeName: 'Ehwaz, the rune of the horse',
    accent: 'text-huginn',
  },
  '/diagnostics': {
    norse: 'Heimdall',
    meaning: 'Heimdall watches over the bridge and misses nothing. He keeps watch on health.',
    story:
      'Heimdall is the watchman of the gods. He needs less sleep than a bird, sees a hundred leagues by night, and hears the grass grow. At Ragnarök he will sound the Gjallarhorn.',
    rune: 'algiz',
    runeName: 'Algiz, the rune of protection',
    accent: 'text-heimdall',
  },
  '/profiles': {
    norse: 'Odin',
    meaning: 'Odin sends out his ravens. Profiles decide how the assistant thinks and what it may do.',
    story:
      'Odin is the Allfather. He hung nine nights on Yggdrasil to win the runes, gave an eye for wisdom, and sends his ravens out over the world each day.',
    rune: 'mannaz',
    runeName: 'Mannaz, the rune of the self',
    accent: 'text-norn',
  },
  '/tools': {
    norse: 'Gungnir',
    meaning: "Gungnir is Odin's spear, which never misses. Tools are how the assistant acts.",
    story:
      "Gungnir is Odin's spear, forged by the dwarves called the sons of Ivaldi. Whoever it is thrown at, it never misses.",
    rune: 'tiwaz',
    runeName: 'Tiwaz, the spear-shaped rune',
    accent: 'text-gungnir',
  },
  '/api-access': {
    norse: 'Valgrind',
    meaning: 'Valgrind is the gate of Valhalla. API keys decide who may come in.',
    story:
      'Valgrind is the ancient gate that stands before Valhalla. It is holy and old, and few know how its lock is fastened.',
    rune: 'thurisaz',
    runeName: 'Thurisaz, the rune of the gate',
    accent: 'text-bifrost',
  },
  '/settings': {
    norse: 'Forseti',
    meaning: 'Forseti settles every matter fairly. Settings are the rules Yggdrasil keeps.',
    story:
      'Forseti, son of Baldr, sits in Glitnir, a hall with pillars of gold and a roof of silver. Everyone who brings a dispute to him leaves reconciled.',
    rune: 'isa',
    runeName: 'Isa, the rune of stillness',
    accent: 'text-huginn',
  },
}

/** The realm for a path such as /knowledge or /knowledge/abc. */
export function realmFor(pathname: string): Realm | undefined {
  const base = '/' + (pathname.split('/')[1] ?? '')
  return realms[base]
}

/**
 * Rune strokes on a 10 × 16 grid. Elder Futhark runes are made of straight
 * lines, so a few path commands draw each one crisply at any size, without
 * depending on a font that has the Runic block.
 */
export const runePaths: Record<RuneId, string> = {
  ansuz: 'M3 15V1M3 1l5 3.5M3 5l5 3.5',
  jera: 'M5.5 2 2 6l3.5 4M4.5 6 8 10l-3.5 4',
  uruz: 'M2 15V1l6 4v10',
  kenaz: 'M8 3 3 8l5 5',
  laguz: 'M3 15V1l5 4',
  othala: 'M2 13 8 5 5 1.5 2 5l6 8',
  raidho: 'M3 15V1l5 3.5L3 8l5 7',
  ehwaz: 'M2 15V1l3 4 3-4v14',
  algiz: 'M5 15V1M5 7 1.5 2M5 7l3.5-5',
  mannaz: 'M2 15V1l6 6V1v14M2 7l6-6',
  tiwaz: 'M5 15V1M1.5 5 5 1l3.5 4',
  thurisaz: 'M3 1v14M3 4l5 4-5 4',
  isa: 'M5 1v14',
}
