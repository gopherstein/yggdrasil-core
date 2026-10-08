import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'

/**
 * Who this browser is signed in as, and signing out (#206). Shown only to a
 * signed-in browser: the Owner at this computer, or an app with a key,
 * isn't signed in.
 */
export function YourAccount() {
  const { t } = useTranslation('people')
  const queryClient = useQueryClient()
  const me = useQuery({ queryKey: ['me'], queryFn: () => api.getMe(), retry: false })
  if (me.data?.via !== 'session') return null
  const person = me.data.person
  return (
    <section className="card flex flex-wrap items-center justify-between gap-3" aria-label={t('account.label')}>
      <p className="text-sm text-ink">
        {t('account.signedInAs', { name: person.name, username: person.username, role: t(`roles.${person.role}`) })}
      </p>
      <button
        type="button"
        className="btn-secondary btn-sm"
        onClick={async () => {
          await api.signOut().catch(() => undefined)
          await queryClient.invalidateQueries()
        }}
      >
        {t('account.signOut')}
      </button>
    </section>
  )
}
