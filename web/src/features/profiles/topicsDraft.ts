import type { TopicPolicy } from '@/types/api'

/** Topic controls as the editor holds them: every field a string, lists one per line. */
export interface TopicsDraft {
  staysOn: string
  examples: string
  never: string
  reply: string
  /** Sites web search and pages keep to, one per line. */
  sites: string
  /** Words added to every web search, one per line. */
  words: string
  /** Guide gives the rules only; Enforce also checks each message and answer. */
  strict: 'guide' | 'enforce'
}

export function topicsDraft(t?: TopicPolicy): TopicsDraft {
  return {
    staysOn: t?.stays_on ?? '',
    examples: (t?.examples ?? []).join('\n'),
    never: (t?.never_discuss ?? []).join('\n'),
    reply: t?.off_topic_reply ?? '',
    sites: (t?.web_sites ?? []).join('\n'),
    words: (t?.web_keywords ?? []).join('\n'),
    strict: t?.strictness === 'enforce' ? 'enforce' : 'guide',
  }
}

const lines = (s: string) =>
  s
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)

/** The topic controls to save, or undefined for none. */
export function topicsFrom(d: TopicsDraft): TopicPolicy | undefined {
  const staysOn = d.staysOn.trim()
  if (!staysOn) return undefined
  const out: TopicPolicy = { stays_on: staysOn }
  const examples = lines(d.examples)
  const never = lines(d.never)
  if (examples.length) out.examples = examples
  if (never.length) out.never_discuss = never
  if (d.reply.trim()) out.off_topic_reply = d.reply.trim()
  const sites = lines(d.sites)
  const words = lines(d.words)
  if (sites.length) out.web_sites = sites
  if (words.length) out.web_keywords = words
  if (d.strict === 'enforce') out.strictness = 'enforce'
  return out
}
