### Security

- Traffic between your computers is encrypted. Bifrost speaks TLS, and
  each computer is checked against the key it paired with, so another
  machine at its address is refused. Chats placed on another computer,
  their answers, tool jobs and their files, and training runs no longer
  cross the network readable. Once a computer has used encryption, it is
  never reached without it again.
- A paired computer still on an older Toskar keeps working over plain HTTP,
  and the Computers page marks it **Not encrypted** until it's updated.
