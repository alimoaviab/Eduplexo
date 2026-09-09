#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Eduplexo Showcase Seeder Runner
# ═══════════════════════════════════════════════════════════════════════════
# Single-command runner to seed the complete demonstration school into
# PostgreSQL and synchronize the backend cache.
#
# Usage:
#   bash scripts/run_demo_seed.sh
# ═══════════════════════════════════════════════════════════════════════════

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SQL_FILE="${SCRIPT_DIR}/seed_demo_school.sql"

echo "══════════════════════════════════════════════════════════════════"
echo "        EDUPLEXO ENTERPRISE DEMO SHOWCASE SEEDER                "
echo "══════════════════════════════════════════════════════════════════"
echo "Target Account: dummy@gmail.com / Test@123"
echo "School Name:    Eduplexo Model Academy (SCH-DUMMY)"
echo "Plan:           Enterprise (1,000 Students, Full Capabilities)"
echo "══════════════════════════════════════════════════════════════════"

if [ ! -f "${SQL_FILE}" ]; then
  echo "Error: SQL file not found at ${SQL_FILE}"
  exit 1
fi

SEEDED=false

# ─── Option A: Production Docker Compose (Contabo VPS) ───────────────────
if command -v docker >/dev/null 2>&1 && [ -f "${ROOT_DIR}/docker-compose.prod.yml" ]; then
  echo "→ Checking for running production Docker containers..."
  if docker compose -f "${ROOT_DIR}/docker-compose.prod.yml" ps --services --filter "status=running" | grep -q "postgres"; then
    echo "✓ Found running production PostgreSQL container."
    echo "→ Applying ${SQL_FILE} to database..."
    
    # Load .env.prod if present for credentials
    if [ -f "${ROOT_DIR}/.env.prod" ]; then
      set -a
      # shellcheck disable=SC1091
      source "${ROOT_DIR}/.env.prod"
      set +a
    fi
    
    PG_USER="${POSTGRES_USER:-school_user}"
    PG_DB="${POSTGRES_DB:-school_db}"
    
    docker compose -f "${ROOT_DIR}/docker-compose.prod.yml" exec -T postgres psql -U "${PG_USER}" -d "${PG_DB}" < "${SQL_FILE}"
    echo "✓ Database seeded successfully!"
    
    echo "→ Refreshing backend-go cache to hydrate in-memory store..."
    docker compose -f "${ROOT_DIR}/docker-compose.prod.yml" restart backend-go
    echo "✓ backend-go restarted and synchronized."
    SEEDED=true
  fi
fi

# ─── Option B: Development Docker Compose ────────────────────────────────
if [ "$SEEDED" = false ] && command -v docker >/dev/null 2>&1 && [ -f "${ROOT_DIR}/docker-compose.yml" ]; then
  if docker compose -f "${ROOT_DIR}/docker-compose.yml" ps --services --filter "status=running" 2>/dev/null | grep -q "postgres"; then
    echo "✓ Found running local development PostgreSQL container."
    echo "→ Applying ${SQL_FILE} to database..."
    docker compose -f "${ROOT_DIR}/docker-compose.yml" exec -T postgres psql -U school_user -d school_db < "${SQL_FILE}"
    echo "✓ Database seeded successfully!"
    if docker compose -f "${ROOT_DIR}/docker-compose.yml" ps --services 2>/dev/null | grep -q "backend-go"; then
      docker compose -f "${ROOT_DIR}/docker-compose.yml" restart backend-go
    fi
    SEEDED=true
  fi
fi

# ─── Option C: Direct psql via DATABASE_URL ──────────────────────────────
if [ "$SEEDED" = false ]; then
  DB_URL="${DATABASE_URL:-}"
  if [ -z "${DB_URL}" ] && [ -f "${ROOT_DIR}/backend-go/.env" ]; then
    DB_URL=$(grep '^DATABASE_URL=' "${ROOT_DIR}/backend-go/.env" | cut -d '=' -f2- | tr -d '"' || true)
  fi
  if [ -z "${DB_URL}" ] && [ -f "${ROOT_DIR}/.env.local" ]; then
    DB_URL=$(grep '^DATABASE_URL=' "${ROOT_DIR}/.env.local" | cut -d '=' -f2- | tr -d '"' || true)
  fi

  if [ -n "${DB_URL}" ] && command -v psql >/dev/null 2>&1; then
    echo "✓ Connecting via DATABASE_URL: ${DB_URL%%:*}:***@..."
    psql "${DB_URL}" -v ON_ERROR_STOP=1 < "${SQL_FILE}"
    echo "✓ Database seeded successfully via direct psql connection!"
    echo "NOTE: If backend is running, restart it to refresh in-memory cache."
    SEEDED=true
  fi
fi

if [ "$SEEDED" = false ]; then
  echo "⚠ Warning: Could not automatically detect a running PostgreSQL instance."
  echo "To run manually on the VPS, execute:"
  echo "  docker compose -f docker-compose.prod.yml exec -T postgres psql -U school_user -d school_db < scripts/seed_demo_school.sql"
  echo "  docker compose -f docker-compose.prod.yml restart backend-go"
  echo ""
fi

# ─── Verification & Summary ──────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════════════════════════"
echo "               DEMO SHOWCASE CREDENTIALS SUMMARY                  "
echo "══════════════════════════════════════════════════════════════════"
echo ""
echo "👑 1. SCHOOL ADMIN PORTAL (Full Management Dashboard)"
echo "   URL:      https://app.eduplexo.com/auth/login"
echo "   Email:    dummy@gmail.com"
echo "   Password: Test@123"
echo "   Role:     School Administrator (Active Enterprise Plan)"
echo ""
echo "👩‍🏫 2. TEACHER PORTAL (Schedule, Attendance, Grading, Behavior)"
echo "   URL:      https://app.eduplexo.com/auth/login"
echo "   Password: Test@123 (Same password for all teachers)"
echo "   Teachers:"
echo "     - Sarah Khan (Math):     teacher1@dummy.eduplexo.com"
echo "     - Ahmed Malik (Physics): teacher2@dummy.eduplexo.com"
echo "     - Fatima Noor (English): teacher3@dummy.eduplexo.com"
echo "     - Usman Ali (CS):        teacher4@dummy.eduplexo.com"
echo "     - Ayesha Siddiqua (Chem):teacher5@dummy.eduplexo.com"
echo "     ... up to teacher10@dummy.eduplexo.com"
echo ""
echo "🎓 3. STUDENT PORTAL (Report Cards, Homework, Timetable, Fees)"
echo "   URL:      https://app.eduplexo.com/auth/login"
echo "   Password: Test@123 (Same password for all students)"
echo "   Students:"
echo "     - Ali Khan (Class 10-A):  student91@dummy.eduplexo.com"
echo "     - Sara Ahmed (Class 10-A):student92@dummy.eduplexo.com"
echo "     - Bilal Sheikh (Class 9-A):student81@dummy.eduplexo.com"
echo "     ... 100 total students (student1@.. to student100@..)"
echo ""
echo "══════════════════════════════════════════════════════════════════"
echo "  ✓ Seeder completed. See README_DEMO_SEED.md for full walkthrough!"
echo "══════════════════════════════════════════════════════════════════"
