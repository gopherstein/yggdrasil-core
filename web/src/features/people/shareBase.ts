import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'

function loopback(host: string): boolean {
  return host === 'localhost' || host === '127.0.0.1' || host === '::1' || host === '[::1]' || host === ''
}

/**
 * Where other devices open this Toskar, for a sign-in link to work there
 * (#206): this page's own address when it was opened from another device,
 * otherwise the computer's address on the local network, over HTTPS when
 * the API has a certificate. reachable is false while local network access
 * is off, when only this computer could open the link.
 */
export function useShareBase(): { base: string; reachable: boolean } {
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => api.getSettings(), staleTime: 30_000 })
  const nodes = useQuery({ queryKey: ['nodes'], queryFn: () => api.getNodes(), staleTime: 30_000 })
  const tls = useQuery({ queryKey: ['api-tls'], queryFn: () => api.getApiTLS(), retry: false, staleTime: 60_000 })
  if (typeof window !== 'undefined' && !loopback(window.location.hostname) && window.location.protocol.startsWith('http')) {
    return { base: window.location.origin, reachable: true }
  }
  const lan = settings.data?.lan_api_enabled ?? false
  const port = settings.data?.api_port ?? 7331
  const host = (nodes.data ?? []).find((n) => n.is_local)?.address?.split(':')[0]?.trim() ?? ''
  if (lan && host && !loopback(host)) {
    return { base: `${tls.data?.enabled ? 'https' : 'http'}://${host}:${port}`, reachable: true }
  }
  return { base: typeof window !== 'undefined' ? window.location.origin : '', reachable: lan }
}
