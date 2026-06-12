-- Org name is a display name, not a business key.
-- Domain is already UNIQUE and is the real deduplication mechanism.
ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_name_key;
