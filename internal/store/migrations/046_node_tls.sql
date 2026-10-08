-- Bifrost over TLS (#175): when a paired computer first spoke TLS. After
-- that it is never reached over plain HTTP again, so it can't be pushed back
-- to an unencrypted connection.
ALTER TABLE node_trust ADD COLUMN tls_at TEXT;
