### Added

- Access from anywhere, first part: an Admin can turn on a listener for
  paired devices away from home (API Access → Access from anywhere). It
  serves only what a phone uses, over HTTPS with the computer's
  certificate, to paired devices' keys, and turns away an address that
  keeps sending wrong keys. For now it's reached through a port forwarded
  by hand or Tailscale; automatic setup comes next.
