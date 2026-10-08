import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { Role } from '@/types/api'

const rank: Record<Role, number> = { visitor: 1, member: 2, admin: 3, owner: 4 }

/**
 * The signed-in person's role (#203), for showing only what they may do;
 * the service checks every request too. role is undefined until known. A
 * service from before people (#206) has no /me, and its one user is the
 * Owner.
 */
export function useRole(): { role: Role | undefined; atLeast: (min: Role) => boolean } {
  const me = useQuery({ queryKey: ['me'], queryFn: () => api.getMe(), staleTime: 60_000, retry: false })
  const role: Role | undefined = me.data?.person.role ?? (me.isError ? 'owner' : undefined)
  return { role, atLeast: (min) => role !== undefined && rank[role] >= rank[min] }
}
