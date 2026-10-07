### Fixed

- Profiles you made before image generation existed couldn't make
  pictures: a tool a profile doesn't list counts as off, and only the
  built-in profiles gained new tools. Your own profiles now gain new tools
  that stay on this computer and only read or make files in Toskar's
  store (pictures, clips, audio, files, drafting automations). Web,
  browser, terminal, file writes, and Git still wait for you to turn them
  on, and a tool you turned off stays off.
- Asking for a picture in a chat whose profile keeps image generation off
  says so, and where to turn it on, instead of listing websites. A request
  for a picture or clip no longer searches the web first.
