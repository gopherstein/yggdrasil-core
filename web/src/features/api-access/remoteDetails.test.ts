import { describe, expect, it } from 'vitest'
import { redactAddresses, remoteDetails } from './remoteDetails'

describe('redactAddresses', () => {
  it('names internet addresses by kind and keeps home-network ones and times', () => {
    expect(redactAddresses('mapped 198.51.100.9:7333 from 192.168.1.20 via 100.70.1.2')).toBe(
      'mapped <public IPv4>:7333 from 192.168.1.20 via <carrier-grade NAT address>',
    )
    expect(redactAddresses('[2001:db8::9]:7333 and fe80::1 and fd12:3456::7')).toBe('<public IPv6>:7333 and fe80::1 and fd12:3456::7')
    expect(redactAddresses('at 2026-10-13T18:16:04.000Z on [::]:7333')).toBe('at 2026-10-13T18:16:04.000Z on [::]:7333')
  })
})

describe('remoteDetails', () => {
  it('is plain text for a report, with no addresses, tokens, or full route', () => {
    const text = remoteDetails(
      {
        enabled: true,
        port: 7333,
        listening: '[::]:7333',
        port_mapping: true,
        mapped: '198.51.100.9:7333',
        mapped_by: 'upnp',
        map_reason: 'double_nat',
        map_error: 'upnp: AddPortMapping refused at 203.0.113.1',
        ipv6: ['2001:db8::9'],
        reachable: 'relay',
        relay: { name: 'relay.toskar.ai:4443', state: 'connected', route: 'bk2a5ci2' },
        check: {
          at: '2026-10-13T18:20:00Z',
          relay: { ok: true, ms: 562 },
          direct: [
            { address: '198.51.100.9:7333', ok: false, reason: 'wrong_computer', answered: 'Toskar on linix-rig' },
            { address: '[2001:db8::9]:7333', ok: false, reason: 'http', status: 502 },
          ],
        },
      },
      '1.8.3',
      new Date('2026-10-13T18:21:00Z'),
    )
    expect(text).toContain('Version: 1.8.3')
    expect(text).toContain('Router port: opened with upnp at <public IPv4>:7333 (double_nat)')
    expect(text).toContain('Router said: upnp: AddPortMapping refused at <public IPv4>')
    expect(text).toContain('IPv6: 1 public address')
    expect(text).toContain('Relay: relay.toskar.ai:4443, connected, route bk2a5ci2…')
    expect(text).toContain('  ✓ through the relay: reached in 562 ms')
    expect(text).toContain('  ✗ <public IPv4>:7333: wrong_computer, answered by "Toskar on linix-rig"')
    expect(text).toContain('  ✗ <public IPv6>:7333: http, HTTP 502')
    expect(text).toContain('Copied: 2026-10-13T18:21:00.000Z')
    expect(text).not.toMatch(/198\.51|203\.0|2001:db8/)
  })
})
