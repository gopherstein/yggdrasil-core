### Added

- People can sign in. Admins and the Owner add someone and get a one-time
  link; opening it, they choose a username and password and are signed in.
  Passwords are hashed with argon2id, and too many wrong ones make that
  username wait. Admins add Members and Visitors, the Owner adds Admins
  too, and disabling someone signs them out everywhere. A phone paired with
  a code now gets the key of whoever showed the code. The app's sign-in
  screen and People page come next.
