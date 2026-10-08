### Added

- Sign in through a reverse proxy such as Authelia, Authentik, Cloudflare
  Access, or Tailscale. List the proxy in `trusted_proxies`
  (`TOSKAR_TRUSTED_PROXIES`), and Toskar takes the person from its
  `Remote-User` header, makes them the first time, and gives them a role
  from their groups. The headers count only from the listed addresses.
