-- ═══════════════════════════════════════════════════════════════════════════
-- 000045_deduplicate_subscription_history.up.sql
-- Deduplicates repeated subscription_history rows created by background persister sync ticks.
-- Preserves the earliest record for each unique event tuple.
-- ═══════════════════════════════════════════════════════════════════════════

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY school_id, action, plan_name, start_date, end_date, amount
               ORDER BY created_at ASC, id ASC
           ) AS rnum
    FROM subscription_history
)
DELETE FROM subscription_history
WHERE id IN (
    SELECT id FROM ranked WHERE rnum > 1
);
