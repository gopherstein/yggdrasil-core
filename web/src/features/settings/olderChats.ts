import type { Conversation } from '@/types/api'

const DAY_MS = 24 * 60 * 60 * 1000

/** Chats last used before the cutoff, leaving pinned ones. */
export function olderChats(chats: Conversation[], days: number, pinned: string[], now = Date.now()): Conversation[] {
  const cutoff = now - days * DAY_MS
  const keep = new Set(pinned)
  return chats.filter((c) => !keep.has(c.id) && new Date(c.updated_at || c.created_at).getTime() < cutoff)
}
