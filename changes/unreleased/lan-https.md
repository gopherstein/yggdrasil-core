### Security

- The API on port 7331 answers HTTPS. Phones, browsers, and apps on your
  network can use `https://` with the same port, so keys and chats no
  longer cross the network readable. Toskar makes and keeps its own
  certificate, and API Access shows the start of its fingerprint to compare
  with what a browser shows. You can use your own certificate with
  `api_tls_cert` and `api_tls_key` (or `TOSKAR_API_TLS_CERT` and
  `TOSKAR_API_TLS_KEY`). Plain HTTP still works from this computer and, for
  now, for phones on an older app.
