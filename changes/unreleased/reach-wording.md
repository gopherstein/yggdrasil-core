### Fixed

- The access-from-anywhere check says what answered instead of "Something
  other than Toskar answered on this port": another Toskar by name ("Toskar
  on linix-rig"), a web server that isn't Toskar, or this computer
  answering with an unexpected HTTP status (#602). The details need relay
  0.3.4.
- "Checked moments ago" is a note about the latest result, not an error.
  Settings shows the start of this computer's route ID, as the relay's
  dashboard lists it.
- The route secret is made only once, even when several requests need it
  at the same moment. Before, a computer could enroll on the relay twice,
  under two routes, with its devices knowing only one (#602).
