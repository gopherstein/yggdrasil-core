import { useQuery } from '@tanstack/react-query'
import { create } from 'zustand'
import { api } from '@/lib/api'
import type { RatingTag } from '@/types/api'

/** The reasons a rating may give, in the order they are offered. */
export const RATING_TAGS: RatingTag[] = [
  'great_responses',
  'fast',
  'slow',
  'stable',
  'crashed',
  'too_much_memory',
  'great_for_coding',
  'great_for_chat',
  'good_tool_use',
  'poor_tool_use',
]

/** Which model the rating dialog is open for. */
export const useRatingDialog = create<{
  model: { id: string; name: string } | null
  open: (id: string, name: string) => void
  close: () => void
}>((set) => ({
  model: null,
  open: (id, name) => set({ model: { id, name } }),
  close: () => set({ model: null }),
}))

/** Everyone's ratings of the models here; empty unless community ratings are on. */
export function useCommunityRatings() {
  return useQuery({
    queryKey: ['ratings-community'],
    queryFn: () => api.getCommunityRatings(),
    retry: false,
    staleTime: 10 * 60_000,
  })
}

/** This person's rating of a model. */
export function useModelRating(id: string | undefined) {
  return useQuery({
    queryKey: ['model-rating', id],
    queryFn: () => api.getModelRating(id as string),
    enabled: Boolean(id),
    retry: false,
  })
}
