-- Rollback of 000042: restores the owner role in the CHECK constraint and
-- re-links schools to the disabled migrated owner rows (best effort).
--
-- The data transformation itself (role conversions, school detachment) is
-- NOT reverted: original owner→school linkage lives in owner_schools, which
-- this migration never touches. Production rollback should restore from the
-- pre-migration backup instead.

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
ALTER TABLE users ADD CONSTRAINT users_role_chk
    CHECK (role IN ('owner', 'super_admin', 'admin', 'teacher', 'parent', 'student'));
