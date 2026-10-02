### Added

- Join a computer to your Yggdrasil network with one command. On a computer in the network, `yggctl join-token create` prints a command; run it on the new computer, for example over SSH, and the two trust each other with no pairing prompt, mDNS, or configuration. The token lasts 15 minutes, works once, can be revoked, and never crosses the network: the command carries the first computer's fingerprint, so a different machine at that address is refused. `yggctl network` shows the computers this one trusts, and `yggctl leave` leaves the network.
