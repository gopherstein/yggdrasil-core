### Fixed

- Asked to run a command that deletes or wipes things, the assistant no
  longer refuses and hands you the command to run yourself, which skipped
  the approval step (#557). It asks to run it with your approval when it
  can, or says plainly that it wasn't done; a command left in the answer is
  taken out, with a notice. Questions about how to do it still get the
  command.
