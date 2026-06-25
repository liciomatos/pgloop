-- CRITICAL: ADD COLUMN with a non-volatile DEFAULT
ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active';
