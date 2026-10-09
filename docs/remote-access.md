# Access from anywhere: design

Status: design (#456). Nothing here ships yet; this is what the pieces build to.

The phone, tablet, TV, and watch apps reach a person's own Toskar from anywhere, not only on the home network, with no port forwarding, VPN, or dynamic DNS to set up. Like Plex remote access, the connection is direct whenever it can be, and Toskar's service mostly introduces the two ends. Everything stays end-to-end encrypted to the computer the app already trusts.

## What exists today

- **Pairing** (Connect a device, #216) gives the app a device key that reaches only the routes a phone uses (`auth.DeviceMayReach`), and the API certificate's fingerprint, `sha256:<hex>` of the certificate (`auth.CertFingerprint`).
- **The pin** (#213, #405): the app keeps that fingerprint per `host:port` (`mobile/modules/tls-pins`, keychain key `toskar.tlsFingerprint`). For a pinned host and port, the certificate must match the pin; the system's name and CA checks don't apply. A changed certificate stops the app and asks the person, never falls back to plain HTTP.
- **The certificate** is persistent (`secrets/api-tls.*`, or one the person supplies), so the pin holds across restarts.
- **Who a request is from:** a request from another machine always needs a key; one through a proxy is never "this computer" or "the local network" (`auth.FromThisComputer`, `auth.FromLocalNetwork`).
- **Do it yourself:** Tailscale, a port forward, or Cloudflare Tunnel already work, because pairing accepts their addresses. These stay free and documented.

## The paths, in the order the app tries them

1. **Home network:** the address from pairing, as today. Free, never metered.
2. **Direct:** a public address the computer can be reached at: an IPv6 address, or an IPv4 port the computer opened on the router itself (UPnP-IGD, NAT-PMP, or PCP), or one the person forwarded by hand. Plain HTTPS from the app to the computer.
3. **Relay:** when the computer can't be reached directly (carrier-grade NAT, double NAT, a router that refuses port mapping), the app connects through Toskar's relay, which forwards the encrypted bytes without reading them.

The app tries them at once where it can, and keeps the first that answers with the pinned certificate: the LAN address and the direct ones race, and the relay starts after a short delay (about a second). It remembers which worked and tries that first next time. The current path shows in the app: "At home", "Direct", or "Through Toskar's relay (slower)".

**Hole punching is left out of the first version.** The apps talk HTTPS through the platform's own stack (URLSession, OkHttp), which can't do TCP simultaneous open or a UDP transport such as QUIC over a punched hole without a native networking layer of its own. With direct and relayed paths both end to end, hole punching is an optimization for relay cost, which is small for chat (see Cost). Revisit it if relayed traffic turns out large.

## End-to-end encryption on every path

Every path is the same TLS session, from the app to the computer's API listener, checked against the same pin:

- **Direct:** the app pins `<public address>:<port>` to the fingerprint it already has.
- **Relay:** the app connects to `relay.toskar.ai:443` with the server name `<route>.relay.toskar.ai` and pins `<route>.relay.toskar.ai:443` to the same fingerprint. The relay reads only the server name in the TLS ClientHello, picks the computer's tunnel by it, and passes the bytes through. It never terminates TLS, has no certificate for the computer, and can't read prompts, answers, files, or the device key inside.

A relay or rendezvous that lies, or is taken over, can at most drop or delay traffic: a connection it sends anywhere else fails the pin. The pin is never learned from the service; it only comes from pairing.

## The computer's side

When the person turns on **Settings → Access from anywhere** (off by default; Admins and the Owner only):

- **A remote listener.** The computer serves the device routes, and only those, on a separate port for traffic from outside: phone keys only (`DeviceMayReach`), never "this computer" or "the local network", with stricter rate limits and the same TLS certificate. The full control API, API Access keys, and settings stay on the LAN. Turning the setting off closes it at once.
- **Port mapping:** it asks the router for a mapping to that port with PCP, then NAT-PMP, then UPnP-IGD, renews it before it lapses, and removes it when the setting is turned off or Toskar quits. A port the person forwarded by hand can be entered instead.
- **Public addresses:** its global IPv6 addresses, and the router's external IPv4 address from the mapping protocol; with neither, it asks the rendezvous what address it saw.
- **Registration:** every few minutes, and whenever an address changes, it sends the rendezvous an address record (below).
- **Relay tunnel:** with the subscription active and no direct path working, it keeps one outbound connection to the relay, over TLS to the relay's own certificate, and accepts the streams the relay passes it as connections to the remote listener. No inbound port is needed.
- **Status in Settings:** "Reachable from anywhere: direct", "through IPv6", "through Toskar's relay", or "not reachable, and why" (no mapping, no subscription, the service unreachable), with the address and port it found.

## The rendezvous

A small stateless service with a key-value store. It holds no traffic.

- **The route ID.** At pairing, the computer gives the app a random 32-byte **route secret** (one per computer, kept with its secrets) beside the fingerprint. The route ID is `HKDF(route secret, "toskar route id")`; the record key is `HKDF(route secret, "toskar route key")`. Devices paired before this get the secret over the home network, with their device key, the first time they connect there.
- **The address record**, sent by the computer:
  - its addresses and ports, and a time;
  - signed with the API certificate's private key, with the certificate included;
  - encrypted with the record key (AES-GCM).
- **Lookup:** the app asks for the record by route ID, decrypts it, checks that the certificate's fingerprint is its pin and the signature holds, and only then uses the addresses.
- **What it learns:** route IDs, opaque blobs, and the computer's and apps' IP addresses as they connect. Not the fingerprint, not the addresses inside the record, not who paired with whom. Someone who learns a fingerprint can't find the computer, because the route ID comes from the secret, not the fingerprint.
- **Entitlement:** a lookup or relay use needs an active subscription for that route ID (below).

## The relay

- **Routing by server name:** the first label of `<route>.relay.toskar.ai` is a short form of the route ID. The computer's tunnel registers under it, authenticated by the route token the app obtained (below), so only the computer the subscription was bought for can take the route.
- **Pass-through:** the relay copies bytes between the app's TCP connection and a stream in the computer's tunnel, nothing more.
- **Limits:** a fair-use cap on relayed bytes per route a month, connection-rate limits per route and per client address, and an idle timeout. Over the cap, the app says so and keeps the home network and on-device answers.
- **Counting** is by bytes and connections; the relay can't count messages, because it can't read them. That's one reason for flat pricing (below).

## Subscription and entitlement

- **The subscription is bought in the app** with App Store or Play billing. No account with us: the store receipt is the entitlement.
- **Receipt exchange:** the app sends the receipt and the route ID to the rendezvous, which checks it with the App Store Server API or the Google Play Developer API and returns a **route token**: route ID, expiry, signed by the service. The app passes the token to the computer over its existing TLS session, and the computer presents it when registering and opening its tunnel.
- **Renewals, cancellations, refunds** reach the service through the stores' server notifications; an expired token stops new relay connections and lookups but never cuts off an answer in progress.
- **Pricing:** with direct paths first, most people cost almost nothing, so a flat monthly price with a fair-use cap on relayed traffic is simpler than metering messages. A household covers the devices on one computer. Final prices wait for the cost test.
- **Desktop to desktop** (Toskar Pro away from home reaching the home computer) can't use mobile store billing; it's a later option with billing on toskar.ai.

## Threat model

| Threat | What stops it |
| --- | --- |
| The relay or rendezvous reads traffic | TLS is end to end to the computer; the relay never terminates it. |
| The relay or rendezvous sends the app to a fake server | The pin, which only pairing sets; a fake fails the TLS handshake. |
| A forged or stale address record | Signed with the pinned certificate's key and time-stamped; the app drops records that don't verify or are old. |
| Someone finds a person's computer by its fingerprint | Route IDs come from the route secret, never the fingerprint; records are encrypted. |
| A stranger reaches the remote listener | It answers only phone keys, on phone routes, with rate limits; no "this computer" or LAN trust, no control API. |
| A stolen phone | Removing the device in Settings revokes its key, on every path. |
| Someone takes over another person's relay route | Registering a route needs that route's token, which only its subscription produced. |
| The service goes away | Home network and do-it-yourself routes keep working; nothing depends on the service to work at home. |
| A router port left open | The mapping lapses on its own (it's renewed, not permanent) and is removed when the setting is turned off. |

## Privacy

- **What the service keeps:** route IDs, encrypted address records, subscription status and expiry, relayed byte counts, and connection times, for billing and abuse limits. Never content.
- **Listings change** when this ships: toskar.ai/privacy/apps, the App Store privacy labels, and Play data safety ("encrypted in transit", "processed ephemerally"). Store text says "reach your computer from anywhere", without device brand names.

## Cost

- **Rendezvous:** one small instance, separate from the main cluster (which has no room); the push service (#460) can share it.
- **Relay:** chat is small. A question and its answer are a few kilobytes, streamed tokens included, so even a heavy user relays a few megabytes a month. Files and pictures dominate; the fair-use cap is in bytes for that reason.
- **The cost test** before prices: relay bandwidth and instance cost per thousand subscribers, and the share of people who need the relay at all.

## Building it

1. **This design.**
2. **Core:** the setting, the remote listener, port mapping (PCP, NAT-PMP, UPnP-IGD) written in Go without new dependencies, address discovery, and the status in Settings. Useful on its own, with a manual port forward.
3. **Core:** the route secret at pairing and on the LAN for paired devices, the signed and encrypted address record, registration, and the relay tunnel.
4. **`yeixio/toskar-relay`:** the rendezvous, receipt checks and route tokens, and the relay, with deployment on its own instance.
5. **Apps:** subscription screens (StoreKit 2, Play Billing), the receipt exchange, path selection (home, direct, relay) with the pin on each, the path shown, and the offline and over-the-cap messages.
6. **Store setup, privacy listings, docs** for the free routes beside the subscription, then the cost test and final prices.
