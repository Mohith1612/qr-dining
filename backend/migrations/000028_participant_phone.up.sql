-- Optional phone number on a session participant. Collected at join with a
-- "continue without phone" path, so it is nullable and additive. Improves
-- waiter/kitchen identification and is the anchor for future loyalty/CRM.
ALTER TABLE session_participants
    ADD COLUMN IF NOT EXISTS phone_e164 TEXT;
