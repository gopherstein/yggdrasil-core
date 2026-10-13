### Changed

- When this computer connects to a relay, it sends its name, so the relay
  operator's dashboard can tell computers apart (#602). Only the operator
  sees it. It never reaches devices or the address record, and the relay
  still can't read anything inside a connection.
