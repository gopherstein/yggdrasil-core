import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useRole } from '@/lib/role'
import type { Role } from '@/types/api'

/** A page for people with at least role min (#203); others are told whose it is. */
export function RequireRole({ min, children }: { min: Role; children: ReactNode }) {
  const { t } = useTranslation()
  const { role, atLeast } = useRole()
  if (role === undefined) return null
  if (!atLeast(min)) {
    return (
      <div className="mx-auto w-full max-w-2xl min-w-0">
        <section className="card text-sm text-ink-muted" role="status">
          {min === 'member' ? t('roles.membersOnly') : t('roles.adminsOnly')}
        </section>
      </div>
    )
  }
  return <>{children}</>
}
