import { useQuery } from '@tanstack/react-query'
import { create } from 'zustand'
import { api } from '@/lib/api'
import type { LanguageRatingStats, RatingTag } from '@/types/api'
import { answerLanguages } from '@/i18n/answerLanguages'

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

/**
 * The tag a rating gives for a language (multilingual spec §23): its base,
 * such as pt for pt-BR, and Chinese with its script, so the ratings of each
 * language collect together. The ratings service takes nothing finer.
 */
export function ratingLanguage(tag: string): string {
  const [base, ...rest] = tag.toLowerCase().split('-')
  if (base !== 'zh') return base
  return rest.some((p) => p === 'hant' || p === 'tw' || p === 'hk' || p === 'mo') ? 'zh-Hant' : 'zh-Hans'
}

/** The languages a rating can say the model was used in. */
export const ratingLanguages: string[] = [...new Set(answerLanguages.map(ratingLanguage))]

/** How many languages a model card lists ratings for. */
const LANGUAGE_LINES = 3

/** A model's ratings by language: the App language's first, then those with the most ratings. */
export function languageLines(languages: LanguageRatingStats[] | undefined, appLanguage: string): LanguageRatingStats[] {
  const mine = ratingLanguage(appLanguage)
  return [...(languages ?? [])]
    .sort((a, b) => Number(b.language === mine) - Number(a.language === mine) || b.ratings - a.ratings)
    .slice(0, LANGUAGE_LINES)
}

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
