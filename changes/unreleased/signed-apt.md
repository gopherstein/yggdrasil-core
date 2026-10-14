### Security

- Toskar's apt repository is signed, and the install steps no longer add
  it with `[trusted=yes]`, which turned off apt's signature check (#599).
  New installs add its key from `https://toskar.ai/apt/toskar.asc`
  (fingerprint `7124 28C8 E821 E64C D418 32F5 2D28 4623 90D0 C604`), and
  the source is now `/etc/apt/sources.list.d/toskar.list`. An existing
  install moves to the signed form by itself when the package upgrades.
