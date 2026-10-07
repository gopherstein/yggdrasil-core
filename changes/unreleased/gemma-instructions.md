### Fixed

- Gemma models answer the question instead of the instructions. Gemma
  has no place for system instructions, so Toskar's (the date, the reply
  language, how to answer) reached it unmarked, as if the user had written
  them. Gemma 3 4B answered "Say hi in three words" with "You've provided
  the current date and time…". The instructions are now marked as coming
  from the app.
