### Security

- Pairing two computers is signed end to end. Each computer proves it holds
  its key, and only the computer that was asked can finish a pairing, with
  the code that was shown. Update Yggdrasil on both computers before pairing
  them or using them together.
- Pairing codes are limited: repeated wrong codes or failed answers end the
  pairing, and you start again with a new code.
- Pending pairing requests are no longer listed to other computers on the
  network.
- Requests between paired computers name the computer they are for and are
  good only once.

### Fixed

- Paired computers' fingerprints are stored in the same form join commands
  show; existing ones are updated when Yggdrasil starts.
