// Taking out of text what could say who or where someone is, before it
// goes in a report (#588, #602). Core does the same to what it adds
// (internal/diagnostics/report.go).

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

const email = /[\w.+-]+@[\w-]+(?:\.[\w-]+)+/g
// Toskar's own keys, a bearer header or key=value, and a long run of
// letters and digits that a word or ID wouldn't be.
const key = /\b(?:ygg|tsk|toskar)_[\w-]+/g
const secret = /\b(bearer\s+|(?:token|secret|password|key)"?\s*[=:]\s*"?)[^\s",}<][^\s",}]*/gi
const long = /\b[A-Za-z0-9+/_-]{32,}={0,2}/g
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const home = /(\/Users\/|\/home\/|[A-Z]:\\Users\\)[^/\\\s"]+/gi

/**
 * Text with email addresses, keys and tokens, the name in a home folder,
 * and internet addresses taken out, so it can go in a report.
 */
export function scrub(text: string): string {
  return redactAddresses(
    text
      .replace(email, '<email>')
      .replace(key, '<key>')
      .replace(secret, '$1<hidden>')
      .replace(long, (m) => (uuid.test(m) || !/\d/.test(m) || !/[a-z]/i.test(m) ? m : '<hidden>'))
      .replace(home, '$1<you>'),
  )
}
