import type { api, ReachWay } from '@/lib/api'
import { redactAddresses } from '@/lib/scrub'

type Status = NonNullable<Awaited<ReturnType<typeof api.getRemoteAccess>>>

export { redactAddresses }

function way(w: ReachWay, label: string): string {
  if (w.ok) return `  ✓ ${label}: reached in ${w.ms ?? 0} ms`
  const detail = [w.reason ?? 'unreachable', w.status ? `HTTP ${w.status}` : '', w.answered ? `answered by "${w.answered}"` : '']
    .filter(Boolean)
    .join(', ')
  return `  ✗ ${label}: ${detail}`
}

/**
 * Access from anywhere as plain text, in English, for a bug report or a
 * support question (#602): versions, settings, the router's answer, the
 * relay, and the last outside check. It never holds a token, a secret, or
 * a full route ID, and internet addresses are replaced by their kind.
 */
export function remoteDetails(status: Status, version?: string, now: Date = new Date()): string {
  const lines = [
    'Toskar access from anywhere',
    `Version: ${version || 'unknown'}`,
    `Copied: ${now.toISOString()}`,
    `On: ${status.enabled ? 'yes' : 'no'}`,
    `Port: ${status.port ?? '?'}${status.listening ? `, listening on ${status.listening}` : ''}${status.error ? `, error: ${status.error}` : ''}`,
  ]
  if (status.address) lines.push(`Forwarded address: ${status.address}`)
  lines.push(
    `Router port: ${status.port_mapping === false ? 'not asked' : status.mapped_by ? `opened with ${status.mapped_by} at ${status.mapped}` : 'not opened'}` +
      (status.map_reason ? ` (${status.map_reason})` : ''),
  )
  if (status.map_error) lines.push(`Router said: ${status.map_error}`)
  if (status.ipv6?.length) lines.push(`IPv6: ${status.ipv6.length} public address${status.ipv6.length === 1 ? '' : 'es'}`)
  lines.push(`Reachable: ${status.reachable ?? '?'}${status.reason ? ` (${status.reason})` : ''}`)
  if (status.relay) {
    lines.push(
      `Relay: ${status.relay.name}, ${status.relay.state}${status.relay.error ? ` (${status.relay.error})` : ''}${status.relay.route ? `, route ${status.relay.route}…` : ''}`,
    )
  }
  const check = status.check
  if (check?.at) {
    lines.push(`Outside check: ${check.at}`)
    if (check.relay) lines.push(way(check.relay, 'through the relay'))
    for (const w of check.direct ?? []) lines.push(way(w, w.address ?? 'direct'))
  } else {
    lines.push('Outside check: not run yet')
  }
  if (check?.error) lines.push(`Last check attempt: ${check.error}`)
  for (const o of status.others ?? []) {
    lines.push(`Another Toskar on this computer: version ${o.version}, port ${o.api_port}${o.remote ? ', access from anywhere on' : ''}`)
  }
  return redactAddresses(lines.join('\n'))
}
