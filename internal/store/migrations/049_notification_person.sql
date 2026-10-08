-- Whose a notification is (#206): its person sees it, and Admins and the
-- Owner also see notices about the install. Everything before is the
-- Owner's.
ALTER TABLE notifications ADD COLUMN person_id TEXT NOT NULL DEFAULT 'owner';
CREATE INDEX IF NOT EXISTS idx_notifications_person ON notifications(person_id, dismissed_at, created_at);
