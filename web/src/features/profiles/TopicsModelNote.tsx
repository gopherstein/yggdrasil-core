import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { api } from '@/lib/api'

/**
 * Under Enforce, says when no installed model is verified to check topics
 * (#457): the checks then run on the profile's own model.
 */
export function TopicsModelNote() {
  const { t } = useTranslation('profiles')
  const caps = useQuery({ queryKey: ['capabilities'], queryFn: () => api.getCapabilities(), retry: false, staleTime: 15_000 })
  const ability = Array.isArray(caps.data?.abilities) ? caps.data.abilities.find((a) => a.id === 'topic_checks') : undefined
  if (!ability || ability.available) return null
  return (
    <p className="max-w-2xl rounded-lg bg-warning/10 p-3 text-xs text-ink" role="note">
      {t('topics.noVerifiedModel')}{' '}
      <Link to="/models" className="text-primary hover:underline">
        {t('topics.installVerified')}
      </Link>
    </p>
  )
}
