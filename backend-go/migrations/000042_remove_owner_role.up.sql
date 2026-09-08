-- ═══════════════════════════════════════════════════════════════════════════
-- 000042_remove_owner_role.up.sql
-- Removes the legacy Owner role from active product logic.
--
-- Legacy architecture:
--   Owner (users.role='owner', school_id='system')
--     └── owner_schools junction + schools.owner_user_id/owner_email
--           ├── School A (independent operational tenant)
--           ├── School B
--           └── School C
--
-- Target architecture:
--   School A → School Admin A   (role='admin', school_id = School A)
--   School B → School Admin B
--   School C → School Admin C
--
-- Data preservation rules:
--   * Every existing school row is untouched (no school is deleted).
--   * Students, teachers, classes, fees, attendance, exams, payments,
--     academic sessions, settings — all keep their school_id FKs.
--   * Owner credentials are REUSED (bcrypt hash copied in-database); no
--     plaintext handling, no password weakening.
--   * owner_schools rows are RETAINED as audit/migration history.
--   * subscriptions/payment_requests/subscription_history keyed by the
--     owner's user id are remapped to the owner's primary school.
--   * schools.owner_user_id / owner_email are blanked (audit kept in
--     owner_schools); campuses.owner_user_id likewise.
--
-- Ordering note: the users_role_chk CHECK constraint is swapped only AFTER
-- every role='owner' row has been converted, because ADD CONSTRAINT
-- validates against existing rows.
-- ═══════════════════════════════════════════════════════════════════════════

-- ─── 0. Safety: only run against the expected schema ─────────────────────
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'role'
    ) THEN
        RAISE EXCEPTION 'users.role missing — wrong database?';
    END IF;
END $$;

-- ─── 1. Resolve each legacy owner's school set ───────────────────────────
-- A school is "owned" when owner_schools links it, or schools.owner_user_id /
-- owner_email points at the owner. Sentinel scopes are excluded.
CREATE TEMP TABLE _owner_school_map ON COMMIT DROP AS
SELECT u.id                    AS owner_user_id,
       u.email                 AS owner_email,
       s.school_id             AS school_id,
       ROW_NUMBER() OVER (
           PARTITION BY u.id
           ORDER BY s.created_at ASC, s.school_id ASC
       )                       AS school_rank
FROM users u
JOIN LATERAL (
    SELECT DISTINCT school_id, created_at
    FROM (
        SELECT s.school_id, s.created_at
        FROM schools s
        WHERE (s.owner_user_id = u.id OR s.owner_email = u.email)
          AND s.school_id NOT IN ('system', '__global__')
          AND s.school_id <> ''
        UNION
        SELECT s2.school_id, s2.created_at
        FROM owner_schools os
        JOIN schools s2 ON s2.school_id = os.school_id
        WHERE os.owner_user_id = u.id
          AND s2.school_id NOT IN ('system', '__global__')
          AND s2.school_id <> ''
    ) combined
) s ON true
WHERE u.role = 'owner';

-- ─── 2. Convert the primary school's owner into its School Admin ─────────
-- The owner user becomes the School Admin of their FIRST (primary) school.
-- school_id moves from the 'system' sentinel to the real tenant; role becomes
-- 'admin'. The bcrypt password_hash is copied verbatim — no plaintext work.
--
-- Email-collision handling: users uniqueness is (school_id, email). If that
-- school already has a DIFFERENT active admin with the same email, the
-- pre-existing admin wins and the migrated owner row is disabled instead.
-- The UPDATE below only targets rows that will NOT collide; the remainder
-- are disabled afterwards.
UPDATE users u
SET school_id   = m.school_id,
    role        = 'admin',
    permissions = ARRAY[]::TEXT[],
    updated_at  = NOW()
FROM _owner_school_map m
WHERE m.owner_user_id = u.id
  AND m.school_rank = 1
  AND u.role = 'owner'
  AND NOT EXISTS (
      SELECT 1 FROM users ex
      WHERE ex.school_id = m.school_id
        AND ex.email = u.email
        AND ex.id <> u.id
        AND ex.role IN ('admin', 'owner')
  );

-- Owners whose primary school already had another admin with the same email:
-- park them disabled in the system scope (audit preserved, login impossible).
UPDATE users u
SET school_id  = 'system',
    role       = 'admin',
    permissions = ARRAY[]::TEXT[],
    status     = 'disabled',
    updated_at = NOW()
FROM _owner_school_map m
WHERE m.owner_user_id = u.id
  AND m.school_rank = 1
  AND u.role = 'owner'
  AND EXISTS (
      SELECT 1 FROM users ex
      WHERE ex.school_id = m.school_id
        AND ex.email = u.email
        AND ex.id <> u.id
        AND ex.role IN ('admin', 'owner')
  );

-- ─── 3. Additional schools (extra campuses under one owner) ─────────────
-- Each remaining owned school needs its own School Admin. Reuse the owner's
-- bcrypt hash with a derived, deterministic email so every independent
-- school/admin context is complete. The original owner account now only
-- covers the primary school (step 2), so derived emails avoid the per-school
-- email uniqueness constraint.
INSERT INTO users (
    id, school_id, email, password_hash, role, permissions,
    profile_first, profile_last, profile_phone, status, created_at, updated_at
)
SELECT
    'usr_admin_' || m.school_id,
    m.school_id,
    -- deterministic derived identity: local+schoolcode@domain
    CASE
        WHEN position('@' in u.email) > 1 THEN
            split_part(u.email, '@', 1) || '+' || lower(regexp_replace(coalesce(sch.code, m.school_id), '[^a-zA-Z0-9]', '', 'g')) || '@' || split_part(u.email, '@', 2)
        ELSE 'admin+' || lower(regexp_replace(coalesce(sch.code, m.school_id), '[^a-zA-Z0-9]', '', 'g')) || '@eduplexo.local'
    END,
    u.password_hash,
    'admin',
    ARRAY[]::TEXT[],
    u.profile_first,
    u.profile_last,
    u.profile_phone,
    'active',
    NOW(),
    NOW()
FROM _owner_school_map m
JOIN users u      ON u.id = m.owner_user_id
JOIN schools sch  ON sch.school_id = m.school_id
WHERE m.school_rank > 1
  AND u.role = 'owner'
  AND NOT EXISTS (
      SELECT 1 FROM users ex
      WHERE ex.school_id = m.school_id AND ex.role = 'admin' AND ex.status = 'active'
  )
ON CONFLICT (id) DO NOTHING;

-- If a derived email still collides with an existing user in that school,
-- fall back to a unique suffix (CITEXT comparison is case-insensitive).
UPDATE users derived
SET email = split_part(derived.email, '@', 1) || '.' || right(derived.id, 6) || '@' || split_part(derived.email, '@', 2)
WHERE derived.role = 'admin'
  AND derived.id LIKE 'usr_admin_%'
  AND EXISTS (
      SELECT 1 FROM users other
      WHERE other.email = derived.email
        AND other.school_id = derived.school_id
        AND other.id <> derived.id
  );

-- ─── 3b. Promote secondary campuses into independent schools ─────────────
-- Under the legacy Owner system, an owner could create additional campuses
-- sharing the parent school_id (campuses table). Each operational campus must
-- now become its own independent school context.
CREATE TEMP TABLE _secondary_campuses ON COMMIT DROP AS
SELECT c.id                                AS campus_id,
       c.school_id                         AS parent_school_id,
       c.name                              AS campus_name,
       c.code                              AS campus_code,
       c.address                           AS campus_address,
       c.city                              AS campus_city,
       c.phone                             AS campus_phone,
       c.email                             AS campus_email,
       c.status                            AS campus_status,
       c.created_at                        AS campus_created_at,
       'sch_' || regexp_replace(c.id, '^cmp_', '') AS new_school_id,
       ps.owner_user_id                    AS parent_owner_user_id
FROM (
    SELECT c.*,
           ROW_NUMBER() OVER (
               PARTITION BY c.school_id
               ORDER BY c.created_at ASC, c.id ASC
           ) AS campus_rank
    FROM campuses c
    WHERE c.school_id NOT IN ('system', '__global__')
      AND c.school_id <> ''
) c
JOIN schools ps ON ps.school_id = c.school_id
WHERE c.campus_rank > 1;

-- 1. Insert newly promoted schools
INSERT INTO schools (
    id, school_id, name, code, address, contact_phone, contact_email, status,
    created_at, updated_at
)
SELECT
    'sch_' || sc.campus_id,
    sc.new_school_id,
    sc.campus_name,
    'CMP_' || SUBSTRING(MD5(sc.campus_id || RANDOM()::TEXT) FROM 1 FOR 6),
    sc.campus_address,
    sc.campus_phone,
    sc.campus_email,
    sc.campus_status,
    sc.campus_created_at,
    NOW()
FROM _secondary_campuses sc
ON CONFLICT (school_id) DO NOTHING;

-- 2. Remap the promoted campus row to its new school_id
UPDATE campuses c
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE c.id = sc.campus_id;

-- 3. Remap dependent entities belonging to the promoted campus
UPDATE users u
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE u.campus_id = sc.campus_id AND u.school_id = sc.parent_school_id;

UPDATE teachers t
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE t.campus_id = sc.campus_id AND t.school_id = sc.parent_school_id;

UPDATE students st
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE st.campus_id = sc.campus_id AND st.school_id = sc.parent_school_id;

UPDATE classes cl
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE cl.campus_id = sc.campus_id AND cl.school_id = sc.parent_school_id;

UPDATE attendance a
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE a.campus_id = sc.campus_id AND a.school_id = sc.parent_school_id;

UPDATE homework hw
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE hw.campus_id = sc.campus_id AND hw.school_id = sc.parent_school_id;

UPDATE exams ex
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE ex.campus_id = sc.campus_id AND ex.school_id = sc.parent_school_id;

UPDATE expenses exp
SET school_id = sc.new_school_id,
    updated_at = NOW()
FROM _secondary_campuses sc
WHERE exp.campus_id = sc.campus_id AND exp.school_id = sc.parent_school_id;

-- 4. Remap fees for migrated students
UPDATE fees f
SET school_id = st.school_id,
    updated_at = NOW()
FROM students st
JOIN _secondary_campuses sc ON st.campus_id = sc.campus_id
WHERE f.student_id = st.id AND f.school_id = sc.parent_school_id;

UPDATE fee_payments fp
SET school_id = st.school_id,
    updated_at = NOW()
FROM students st
JOIN _secondary_campuses sc ON st.campus_id = sc.campus_id
WHERE fp.student_id = st.id AND fp.school_id = sc.parent_school_id;

UPDATE fee_adjustments fa
SET school_id = st.school_id,
    updated_at = NOW()
FROM students st
JOIN _secondary_campuses sc ON st.campus_id = sc.campus_id
WHERE fa.student_id = st.id AND fa.school_id = sc.parent_school_id;

UPDATE student_fee_discounts sfd
SET school_id = st.school_id,
    updated_at = NOW()
FROM students st
JOIN _secondary_campuses sc ON st.campus_id = sc.campus_id
WHERE sfd.student_id = st.id AND sfd.school_id = sc.parent_school_id;

-- 5. Guarantee a School Admin exists for each newly promoted school
INSERT INTO users (
    id, school_id, email, password_hash, role, permissions,
    profile_first, profile_last, profile_phone, status, created_at, updated_at
)
SELECT
    'usr_admin_' || sc.new_school_id,
    sc.new_school_id,
    CASE
        WHEN position('@' in u.email) > 1 THEN
            split_part(u.email, '@', 1) || '+' || lower(regexp_replace(sc.new_school_id, '[^a-zA-Z0-9]', '', 'g')) || '@' || split_part(u.email, '@', 2)
        ELSE 'admin+' || lower(regexp_replace(sc.new_school_id, '[^a-zA-Z0-9]', '', 'g')) || '@eduplexo.local'
    END,
    u.password_hash,
    'admin',
    ARRAY[]::TEXT[],
    u.profile_first,
    u.profile_last,
    u.profile_phone,
    'active',
    NOW(),
    NOW()
FROM _secondary_campuses sc
JOIN users u ON u.id = sc.parent_owner_user_id
WHERE NOT EXISTS (
    SELECT 1 FROM users ex
    WHERE ex.school_id = sc.new_school_id AND ex.role = 'admin' AND ex.status = 'active'
)
ON CONFLICT (id) DO NOTHING;

-- 6. Guarantee a subscription row for each newly promoted school
INSERT INTO subscriptions (
    id, school_id, owner_user_id, plan_name, student_limit, price, currency,
    start_date, end_date, status, is_trial, trial_used, trial_start_date,
    trial_end_date, created_at, updated_at
)
SELECT
    'sub_promoted_' || sc.new_school_id,
    sc.new_school_id,
    '',
    'trial',
    500,
    0,
    'PKR',
    sc.campus_created_at,
    sc.campus_created_at + INTERVAL '14 days',
    CASE WHEN sc.campus_created_at + INTERVAL '14 days' > NOW() THEN 'trial' ELSE 'expired' END,
    true,
    true,
    sc.campus_created_at,
    sc.campus_created_at + INTERVAL '14 days',
    sc.campus_created_at,
    sc.campus_created_at
FROM _secondary_campuses sc
WHERE NOT EXISTS (
    SELECT 1 FROM subscriptions sub WHERE sub.school_id = sc.new_school_id
)
ON CONFLICT (id) DO NOTHING;

-- ─── 4. Remap owner-keyed subscription rows to real schools ─────────────
-- Legacy rows used school_id = owner_user_id (owner-keyed subscriptions).
-- Remap them onto the owner's primary school. Guarded against the partial
-- unique index subscriptions_school_active_uniq (one active/trial row per
-- school): when the target school already has a live row, the owner-keyed
-- row is cancelled instead of moved.
UPDATE subscriptions sub
SET school_id  = m.school_id,
    updated_at = NOW()
FROM _owner_school_map m
WHERE m.owner_user_id = sub.school_id
  AND m.school_rank = 1
  AND sub.school_id NOT IN (SELECT school_id FROM schools)
  AND NOT (
      sub.status IN ('active', 'trial')
      AND EXISTS (
          SELECT 1 FROM subscriptions live
          WHERE live.school_id = m.school_id
            AND live.status IN ('active', 'trial')
            AND live.id <> sub.id
      )
  );

UPDATE subscriptions sub
SET status    = 'cancelled',
    updated_at = NOW()
FROM _owner_school_map m
WHERE m.owner_user_id = sub.school_id
  AND m.school_rank = 1
  AND sub.school_id NOT IN (SELECT school_id FROM schools)
  AND sub.status IN ('active', 'trial')
  AND EXISTS (
      SELECT 1 FROM subscriptions live
      WHERE live.school_id = m.school_id
        AND live.status IN ('active', 'trial')
        AND live.id <> sub.id
  );

-- Stamp owner_user_id on rows that are school-keyed but missing it.
UPDATE subscriptions sub
SET owner_user_id = m.owner_user_id,
    updated_at    = NOW()
FROM _owner_school_map m
WHERE sub.school_id = m.school_id
  AND COALESCE(sub.owner_user_id, '') = ''
  AND m.school_rank = 1;

-- ─── 5. payment_requests / subscription_history owner backfill ──────────
UPDATE payment_requests pr
SET owner_user_id = m.owner_user_id
FROM _owner_school_map m
WHERE pr.school_id = m.school_id
  AND COALESCE(pr.owner_user_id, '') = ''
  AND m.school_rank = 1;

UPDATE subscription_history sh
SET owner_user_id = m.owner_user_id
FROM _owner_school_map m
WHERE sh.school_id = m.school_id
  AND COALESCE(sh.owner_user_id, '') = ''
  AND m.school_rank = 1;

-- ─── 6. Guarantee every migrated school has a subscription row ──────────
-- Owner-linked schools with no subscription at all get a trial row anchored
-- to the school's own created_at (never a fresh trial from NOW()).
INSERT INTO subscriptions (
    id, school_id, owner_user_id, plan_name, student_limit, price, currency,
    start_date, end_date, status, is_trial, trial_used, trial_start_date,
    trial_end_date, created_at, updated_at
)
SELECT
    'sub_migrated_' || sch.school_id,
    sch.school_id,
    '',
    'trial',
    500,
    0,
    'PKR',
    sch.created_at,
    sch.created_at + INTERVAL '14 days',
    CASE WHEN sch.created_at + INTERVAL '14 days' > NOW() THEN 'trial' ELSE 'expired' END,
    true,
    true,
    sch.created_at,
    sch.created_at + INTERVAL '14 days',
    sch.created_at,
    sch.created_at
FROM schools sch
WHERE sch.school_id NOT IN ('system', '__global__')
  AND sch.school_id <> ''
  AND (COALESCE(sch.owner_user_id, '') <> '' OR COALESCE(sch.owner_email::TEXT, '') <> '')
  AND NOT EXISTS (
      SELECT 1 FROM subscriptions sub WHERE sub.school_id = sch.school_id
  );

-- ─── 7. Expire any pending owner signups ────────────────────────────────
UPDATE pending_signups
SET status = 'expired', expires_at = NOW()
WHERE role = 'owner' AND status = 'pending';

-- ─── 8. Detach schools from Owner (audit stays in owner_schools) ────────
UPDATE schools
SET owner_user_id = '',
    owner_email   = '',
    updated_at    = NOW()
WHERE COALESCE(owner_user_id, '') <> ''
   OR COALESCE(owner_email::TEXT, '') <> '';

UPDATE campuses
SET owner_user_id = ''
WHERE COALESCE(owner_user_id, '') <> '';

-- ─── 9. Convert any remaining owner rows, then retire the constraint ────
-- Order matters: the CHECK swap validates existing rows, so every
-- role='owner' row must already be gone.
UPDATE users
SET role       = 'admin',
    school_id  = 'system',
    status     = 'disabled',
    permissions = ARRAY[]::TEXT[],
    updated_at = NOW()
WHERE role = 'owner';

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_chk;
ALTER TABLE users ADD CONSTRAINT users_role_chk
    CHECK (role IN ('super_admin', 'admin', 'teacher', 'parent', 'student'));
