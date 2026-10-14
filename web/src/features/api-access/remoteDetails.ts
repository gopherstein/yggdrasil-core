import type { api, ReachWay } from '@/lib/api'

type Status = NonNullable<Awaited<ReturnType<typeof api.getRemoteAccess>>>

const ipv4 = /\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b/g
// Something that may be an IPv6 address, bracketed or bare: hex groups
// and at least two colons. ipv6Kind decides.
const ipv6 = /\[([0-9a-f:]+)\]|(?<![\w:.])([0-9a-f]{0,4}(?::[0-9a-f]{0,4}){2,7})(?![\w:])/gi

/** The kind of a possible IPv6 address, or null to keep the text as it is. */
function ipv6Kind(addr: string): string | null {
  // A time such as 18:16:04 isn't an address: an IPv6 one has :: or a hex letter.
  if (!/::|[a-f]/i.test(addr)) return null
  // Unspecified, loopback, link-local, and unique local stay: they're on this network.
  if (addr === '::' || addr === '::1' || /^(f[cd]|fe80)/i.test(addr)) return null
  return '<public IPv6>'
}

/** The kind of an IPv4 address: kept when it's on the home network, named when it isn't. */
function v4Kind(a: number, b: number): string | null {
  if (a === 10 || a === 127 || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168)) return null
  if (a === 100 && b >= 64 && b <= 127) return '<carrier-grade NAT address>'
  return '<public IPv4>'
}

/**
 * Text with the internet addresses replaced by their kind, so a report
 * says what kind of network this is without saying where it is (#602,
 * #588). Addresses on the home network are kept: they help and give
 * nothing away.
 */
export function redactAddresses(text: string): string {
  return text
    .replace(ipv6, (m, bracketed: string | undefined, bare: string | undefined) => ipv6Kind(bracketed ?? bare ?? '') ?? m)
    .replace(ipv4, (m, a: string, b: string) => v4Kind(Number(a), Number(b)) ?? m)
}

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
