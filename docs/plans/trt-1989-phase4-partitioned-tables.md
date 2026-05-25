# TRT-1989 Phase 4: Partitioned Table DDL Migration Plan

## Context

Sippy's largest tables (`prow_job_runs`, `prow_job_run_tests`, `prow_job_run_test_outputs`, `prow_job_run_annotations`, `prow_job_run_prow_pull_requests`) are growing continuously and being joined in nearly every significant query. Converting these tables to PostgreSQL RANGE-partitioned tables will enable:

- **Partition pruning** during queries (dramatically reduced scan sizes)
- **Independent partition management** (drop old partitions instead of DELETE)
- **Improved query performance** via reduced join costs

**Prerequisites completed:**
- ✅ Phase 1: Denormalized `release` and `timestamp` columns added to all child tables
- ✅ Phase 2: Composite indexes on `(release, timestamp)` created
- ✅ Phase 3: Queries updated to filter on denormalized columns

**This phase** creates the migration DDL for new partitioned tables with temporary names, provides SQL for data migration and identity sequence syncing, and prepares for atomic table swap.

## Migration Strategy Overview

The migration will be split into **versioned migration files** under `pkg/db/migrations/` using golang-migrate's pattern:

1. **Create partitioned tables with `_new` suffix** (e.g., `prow_job_runs_new`)
2. **Create all required indexes** on the new tables
3. **Provide standalone SQL scripts** for data migration (run separately, not in migration files)
4. **Provide standalone SQL scripts** for sequence syncing after data migration
5. **Provide standalone SQL scripts** for atomic table swapping (rename old → `_old`, new → production name)

This approach allows:
- Migration DDL to be tracked in version control and applied via `sippy migrate`
- Data migration to run separately during planned maintenance windows
- Validation between each step
- Easy rollback by renaming tables back

## Tables to Partition

All 5 tables will use **nested partitioning**: LIST by release, then RANGE by timestamp (daily):

| Table | L1: List Partition | L2: Range Partition | FK Dependencies |
|-------|-------------------|---------------------|-----------------|
| `prow_job_runs` | `prow_job_release` | `timestamp` (daily) | Must migrate WITH annotations + pull requests |
| `prow_job_run_tests` | `prow_job_run_release` | `prow_job_run_timestamp` (daily) | Depends on prow_job_runs |
| `prow_job_run_test_outputs` | `prow_job_run_test_release` | `prow_job_run_test_timestamp` (daily) | Depends on prow_job_run_tests |
| `prow_job_run_annotations` | `prow_job_run_release` | `prow_job_run_timestamp` (daily) | Depends on prow_job_runs |
| `prow_job_run_prow_pull_requests` | `prow_job_run_release` | `prow_job_run_timestamp` (daily) | Depends on prow_job_runs |

**Partitioning Strategy:**
1. **Level 1 (LIST)**: Partition by release (e.g., "4.17", "4.18", "4.19")
2. **Level 2 (RANGE)**: Sub-partition each release by timestamp (daily granularity)
3. **Partition creation**: Managed separately (not part of this migration)

**Migration Order:**
- All 5 tables created simultaneously in single migration (no FK dependencies since we're dropping FKs)

## Migration File Structure

Create the following migration files under `pkg/db/migrations/`:

### 000002_create_partitioned_tables.up.sql

Creates all 5 partitioned tables with `_new` suffix:

1. `prow_job_runs_new` (LIST by release → RANGE by timestamp)
2. `prow_job_run_annotations_new` (LIST → RANGE)
3. `prow_job_run_prow_pull_requests_new` (LIST → RANGE)
4. `prow_job_run_tests_new` (LIST → RANGE)
5. `prow_job_run_test_outputs_new` (LIST → RANGE)

Plus all required indexes on each table.

**Note**: Since foreign key constraints are NOT being recreated (Option A), there are no dependencies between these tables, so they can all be created in a single migration file.

**Note**: Partition creation (release-level and daily sub-partitions) is handled separately via partition management system.

### 000002_create_partitioned_tables.down.sql

Drops all 5 `_new` tables:
- `DROP TABLE IF EXISTS prow_job_runs_new CASCADE;`
- `DROP TABLE IF EXISTS prow_job_run_annotations_new CASCADE;`
- `DROP TABLE IF EXISTS prow_job_run_prow_pull_requests_new CASCADE;`
- `DROP TABLE IF EXISTS prow_job_run_tests_new CASCADE;`
- `DROP TABLE IF EXISTS prow_job_run_test_outputs_new CASCADE;`

## DDL Details per Table

All tables below are defined in `000002_create_partitioned_tables.up.sql`.

### 1. prow_job_runs_new

**Primary Key Changes:**
- OLD: `PRIMARY KEY (id)`
- NEW: `PRIMARY KEY (id, prow_job_release, timestamp)`

**Partition Strategy:**
```sql
PARTITION BY LIST (prow_job_release)
-- Each release partition is further partitioned by RANGE (timestamp)
```

**Structure:**
```
prow_job_runs_new (LIST by prow_job_release)
  ├─ prow_job_runs_new_p4_17 PARTITION BY RANGE (timestamp)
  │   └─ Daily sub-partitions (managed separately)
  ├─ prow_job_runs_new_p4_18 PARTITION BY RANGE (timestamp)
  │   └─ Daily sub-partitions (managed separately)
  └─ prow_job_runs_new_default (catches unmapped releases)
```

**Note**: Actual partition creation (release-level and daily sub-partitions) is handled by the partition management system, not this migration.

**Indexes to Create:**
1. ✅ **Primary key** (composite): `(id, prow_job_release, timestamp)`
2. ✅ `idx_prow_job_runs_new_release_timestamp` on `(prow_job_release, timestamp)` — partition pruning when filtering on release + timestamp
3. ✅ `idx_prow_job_runs_new_prow_job_id` on `(prow_job_id)` — FK join to prow_jobs (71M scans)
4. ✅ `idx_prow_job_runs_new_timestamp` on `(timestamp)` — time-range queries without release filter (8.7M scans, needed by BuildClusterHealth)
5. ⚠️ **SKIP**: `idx_prow_job_runs_overall_result` (rarely used, 21 scans)
6. ⚠️ **SKIP**: `idx_prow_job_runs_timestamp_date` (unused, 0 scans)
7. ⚠️ **SKIP**: `idx_prow_job_runs_deleted_at` (unused, 0 scans)
8. ⚠️ **SKIP**: `idx_prow_job_runs_labels` (GIN, unused, 0 scans)
9. ⚠️ **SKIP**: `idx_prow_job_runs_created_at` (not used for business queries)

**Foreign Keys:**
- Outbound FK: `prow_job_id` → `prow_jobs(id)` **KEEP** (prow_jobs is NOT partitioned, no performance impact)
- Inbound FKs: **DROP** (will not be recreated - see Referential Integrity Strategy section)

**Identity Sequence:**
- Use PostgreSQL IDENTITY: `id BIGINT GENERATED BY DEFAULT AS IDENTITY`
- Sequence will be `prow_job_runs_new_id_seq`
- Must sync after data migration

### 2. prow_job_run_annotations_new

**Primary Key Changes:**
- OLD: `PRIMARY KEY (id)`
- NEW: `PRIMARY KEY (id, prow_job_run_release, prow_job_run_timestamp)`

**Partition Strategy:**
```sql
PARTITION BY LIST (prow_job_run_release)
-- Each release partition is further partitioned by RANGE (prow_job_run_timestamp)
```

**Unique Constraint Issue:**
- OLD: `UNIQUE (prow_job_run_id, key)`
- NEW: `UNIQUE (prow_job_run_id, key, prow_job_run_release, prow_job_run_timestamp)` — must include partition keys

**Indexes to Create:**
1. ✅ **Primary key** (composite): `(id, prow_job_run_release, prow_job_run_timestamp)`
2. ✅ `idx_prow_job_run_annotations_new_release_timestamp` on `(prow_job_run_release, prow_job_run_timestamp)` — partition pruning
3. ✅ `idx_prow_job_run_annotations_new_key` (UNIQUE) on `(prow_job_run_id, key, prow_job_run_release, prow_job_run_timestamp)` — enforces one value per annotation key per job run
4. ❌ **SKIP**: `idx_prow_job_run_id` — redundant (UNIQUE index already starts with prow_job_run_id, can serve lookups by job run ID)

### 3. prow_job_run_prow_pull_requests_new

**Primary Key Changes:**
- OLD: `PRIMARY KEY (prow_job_run_id, prow_pull_request_id)`
- NEW: `PRIMARY KEY (prow_job_run_id, prow_pull_request_id, prow_job_run_release, prow_job_run_timestamp)`

**Partition Strategy:**
```sql
PARTITION BY LIST (prow_job_run_release)
-- Each release partition is further partitioned by RANGE (prow_job_run_timestamp)
```

**Indexes to Create:**
1. ✅ **Primary key** (composite): `(prow_job_run_id, prow_pull_request_id, prow_job_run_release, prow_job_run_timestamp)` — PK serves lookups by job run ID
2. ✅ `idx_prow_job_run_prow_pull_requests_new_release_timestamp` on `(prow_job_run_release, prow_job_run_timestamp)` — partition pruning
3. ✅ `idx_prow_job_run_prow_pull_requests_new_prow_pull_request_id` on `(prow_pull_request_id)` — reverse lookup (which job runs used this PR?)

### 4. prow_job_run_tests_new

**Primary Key Changes:**
- OLD: `PRIMARY KEY (id)`
- NEW: `PRIMARY KEY (id, prow_job_run_release, prow_job_run_timestamp)`

**Partition Strategy:**
```sql
PARTITION BY LIST (prow_job_run_release)
-- Each release partition is further partitioned by RANGE (prow_job_run_timestamp)
```

**Indexes to Create:**
1. ✅ **Primary key** (composite): `(id, prow_job_run_release, prow_job_run_timestamp)`
2. ✅ `idx_prow_job_run_tests_new_release_timestamp` on `(prow_job_run_timestamp, prow_job_run_release)` — partition pruning (timestamp first for range filters)
3. ✅ `idx_prow_job_run_tests_new_prow_job_run_id` on `(prow_job_run_id)` — FK join to prow_job_runs (HOTTEST: 164M scans)
4. ✅ `idx_prow_job_run_tests_new_test_id` on `(test_id)` — FK join to tests (16M scans)
5. ✅ `idx_prow_job_run_tests_new_status` on `(status)` — result filtering (2.7M scans)
6. ✅ `idx_prow_job_run_tests_new_prow_job_id` on `(prow_job_id)` — variant queries (Phase 2 addition)
7. ✅ `idx_prow_job_run_tests_new_test_id_status` on `(test_id, status)` — composite for Component Readiness queries (defined in GORM model)
8. ⚠️ **SKIP**: `idx_prow_job_run_tests_deleted_at` (unused, 9.6 GB waste)
9. ⚠️ **SKIP**: `idx_prow_job_run_tests_created_at` (11 scans only, 7.4 GB waste)
10. ⚠️ **SKIP**: `idx_prow_job_run_tests_suite_id` (inefficient: 20K scans, 10 GB)

**Space Savings:** ~27 GB by dropping unused indexes

### 5. prow_job_run_test_outputs_new

**Primary Key Changes:**
- OLD: `PRIMARY KEY (id)`
- NEW: `PRIMARY KEY (id, prow_job_run_test_release, prow_job_run_test_timestamp)`

**Partition Strategy:**
```sql
PARTITION BY LIST (prow_job_run_test_release)
-- Each release partition is further partitioned by RANGE (prow_job_run_test_timestamp)
```

**Indexes to Create:**
1. ✅ **Primary key** (composite): `(id, prow_job_run_test_release, prow_job_run_test_timestamp)`
2. ✅ `idx_prow_job_run_test_outputs_new_release_timestamp` on `(prow_job_run_test_timestamp, prow_job_run_test_release)` — partition pruning
3. ✅ `idx_prow_job_run_test_outputs_new_prow_job_run_test_id` on `(prow_job_run_test_id)` — FK join to prow_job_run_tests
4. ⚠️ **SKIP**: `idx_prow_job_run_test_outputs_created_at` (not used for business queries)

## Data Migration SQL (Standalone Scripts)

Create separate SQL scripts (NOT in migration files) for data migration. These scripts are **idempotent** and can be run multiple times - they only copy data not already migrated.

### migrate_prow_job_runs_data.sql
```sql
-- Copy data from old tables to new partitioned tables
-- Can be run multiple times - only migrates new data (created_at > max in destination)

-- 1. prow_job_runs
DO $$
DECLARE
    max_created_at TIMESTAMP;
BEGIN
    -- Get the latest created_at already migrated (NULL if table is empty)
    SELECT MAX(created_at) INTO max_created_at FROM prow_job_runs_new;
    
    RAISE NOTICE 'Migrating prow_job_runs with created_at > %', COALESCE(max_created_at, '-infinity'::timestamp);
    
    INSERT INTO prow_job_runs_new (
        id, created_at, updated_at, deleted_at,
        prow_job_id, prow_job_release, cluster, gcs_bucket, url,
        test_failures, failed, infrastructure_failure, known_failure,
        succeeded, timestamp, duration, overall_result, labels
    )
    SELECT 
        id, created_at, updated_at, deleted_at,
        prow_job_id, prow_job_release, cluster, gcs_bucket, url,
        test_failures, failed, infrastructure_failure, known_failure,
        succeeded, timestamp, duration, overall_result, labels
    FROM prow_job_runs
    WHERE created_at > COALESCE(max_created_at, '-infinity'::timestamp)
    ORDER BY created_at, id;
    
    RAISE NOTICE 'Migrated % rows', FOUND;
END $$;

-- 2. prow_job_run_annotations
DO $$
DECLARE
    max_created_at TIMESTAMP;
BEGIN
    SELECT MAX(created_at) INTO max_created_at FROM prow_job_run_annotations_new;
    
    RAISE NOTICE 'Migrating prow_job_run_annotations with created_at > %', COALESCE(max_created_at, '-infinity'::timestamp);
    
    INSERT INTO prow_job_run_annotations_new (
        id, created_at, updated_at, deleted_at,
        prow_job_run_id, key, value,
        prow_job_run_release, prow_job_run_timestamp
    )
    SELECT 
        id, created_at, updated_at, deleted_at,
        prow_job_run_id, key, value,
        prow_job_run_release, prow_job_run_timestamp
    FROM prow_job_run_annotations
    WHERE created_at > COALESCE(max_created_at, '-infinity'::timestamp)
    ORDER BY created_at, id;
    
    RAISE NOTICE 'Migrated % rows', FOUND;
END $$;

-- 3. prow_job_run_prow_pull_requests (no created_at - use prow_job_run_timestamp)
DO $$
DECLARE
    max_timestamp TIMESTAMP;
BEGIN
    SELECT MAX(prow_job_run_timestamp) INTO max_timestamp FROM prow_job_run_prow_pull_requests_new;
    
    RAISE NOTICE 'Migrating prow_job_run_prow_pull_requests with prow_job_run_timestamp > %', COALESCE(max_timestamp, '-infinity'::timestamp);
    
    INSERT INTO prow_job_run_prow_pull_requests_new (
        prow_job_run_id, prow_pull_request_id,
        prow_job_run_release, prow_job_run_timestamp
    )
    SELECT 
        prow_job_run_id, prow_pull_request_id,
        prow_job_run_release, prow_job_run_timestamp
    FROM prow_job_run_prow_pull_requests
    WHERE prow_job_run_timestamp > COALESCE(max_timestamp, '-infinity'::timestamp)
    ORDER BY prow_job_run_timestamp, prow_job_run_id;
    
    RAISE NOTICE 'Migrated % rows', FOUND;
END $$;
```

### migrate_prow_job_run_tests_data.sql
```sql
-- Idempotent migration for prow_job_run_tests

DO $$
DECLARE
    max_created_at TIMESTAMP;
BEGIN
    SELECT MAX(created_at) INTO max_created_at FROM prow_job_run_tests_new;
    
    RAISE NOTICE 'Migrating prow_job_run_tests with created_at > %', COALESCE(max_created_at, '-infinity'::timestamp);
    
    INSERT INTO prow_job_run_tests_new (
        id, created_at, updated_at, deleted_at,
        prow_job_run_id, prow_job_id, prow_job_run_timestamp,
        prow_job_run_release, test_id, suite_id, status, duration
    )
    SELECT 
        id, created_at, updated_at, deleted_at,
        prow_job_run_id, prow_job_id, prow_job_run_timestamp,
        prow_job_run_release, test_id, suite_id, status, duration
    FROM prow_job_run_tests
    WHERE created_at > COALESCE(max_created_at, '-infinity'::timestamp)
    ORDER BY created_at, id;
    
    RAISE NOTICE 'Migrated % rows', FOUND;
END $$;
```

### migrate_prow_job_run_test_outputs_data.sql
```sql
-- Idempotent migration for prow_job_run_test_outputs

DO $$
DECLARE
    max_created_at TIMESTAMP;
BEGIN
    SELECT MAX(created_at) INTO max_created_at FROM prow_job_run_test_outputs_new;
    
    RAISE NOTICE 'Migrating prow_job_run_test_outputs with created_at > %', COALESCE(max_created_at, '-infinity'::timestamp);
    
    INSERT INTO prow_job_run_test_outputs_new (
        id, created_at, updated_at, deleted_at,
        prow_job_run_test_id, output,
        prow_job_run_test_timestamp, prow_job_run_test_release
    )
    SELECT 
        id, created_at, updated_at, deleted_at,
        prow_job_run_test_id, output,
        prow_job_run_test_timestamp, prow_job_run_test_release
    FROM prow_job_run_test_outputs
    WHERE created_at > COALESCE(max_created_at, '-infinity'::timestamp)
    ORDER BY created_at, id;
    
    RAISE NOTICE 'Migrated % rows', FOUND;
END $$;
```

**Migration Script Features:**
- **Idempotent**: Can be run multiple times safely
- **Incremental**: Only migrates data with `created_at` > max already in destination
- **Progress tracking**: Uses `RAISE NOTICE` to show what's being migrated
- **Empty table handling**: Uses `COALESCE(max_created_at, '-infinity'::timestamp)` to handle empty destination
- **Ordering**: Sorts by `created_at, id` to ensure chronological migration and stable ordering

## Identity Sequence Sync SQL (Standalone Scripts)

After data migration, sync sequences to ensure new inserts don't conflict:

### sync_sequences.sql
```sql
-- Sync identity sequences after data migration

-- 1. prow_job_runs
SELECT setval('prow_job_runs_new_id_seq', 
    (SELECT COALESCE(MAX(id), 1) FROM prow_job_runs_new), true);

-- 2. prow_job_run_annotations
SELECT setval('prow_job_run_annotations_new_id_seq',
    (SELECT COALESCE(MAX(id), 1) FROM prow_job_run_annotations_new), true);

-- 3. prow_job_run_tests
SELECT setval('prow_job_run_tests_new_id_seq',
    (SELECT COALESCE(MAX(id), 1) FROM prow_job_run_tests_new), true);

-- 4. prow_job_run_test_outputs
SELECT setval('prow_job_run_test_outputs_new_id_seq',
    (SELECT COALESCE(MAX(id), 1) FROM prow_job_run_test_outputs_new), true);
```

## Table Swap SQL (Atomic Rename)

After data migration and validation, swap tables atomically:

### swap_tables.sql
```sql
BEGIN;

-- Drop foreign keys (will NOT be recreated - see Referential Integrity section below)
ALTER TABLE prow_job_run_tests DROP CONSTRAINT IF EXISTS fk_prow_job_run_tests_prow_job_run_id;
ALTER TABLE prow_job_run_test_outputs DROP CONSTRAINT IF EXISTS fk_prow_job_run_test_outputs_prow_job_run_test_id;
ALTER TABLE prow_job_run_annotations DROP CONSTRAINT IF EXISTS fk_prow_job_run_annotations_prow_job_run_id;
ALTER TABLE prow_job_run_prow_pull_requests DROP CONSTRAINT IF EXISTS fk_prow_job_run_prow_pull_requests_prow_job_run_id;

-- Rename old tables to _old
ALTER TABLE prow_job_runs RENAME TO prow_job_runs_old;
ALTER TABLE prow_job_run_annotations RENAME TO prow_job_run_annotations_old;
ALTER TABLE prow_job_run_prow_pull_requests RENAME TO prow_job_run_prow_pull_requests_old;
ALTER TABLE prow_job_run_tests RENAME TO prow_job_run_tests_old;
ALTER TABLE prow_job_run_test_outputs RENAME TO prow_job_run_test_outputs_old;

-- Rename new tables to production names
ALTER TABLE prow_job_runs_new RENAME TO prow_job_runs;
ALTER TABLE prow_job_run_annotations_new RENAME TO prow_job_run_annotations;
ALTER TABLE prow_job_run_prow_pull_requests_new RENAME TO prow_job_run_prow_pull_requests;
ALTER TABLE prow_job_run_tests_new RENAME TO prow_job_run_tests;
ALTER TABLE prow_job_run_test_outputs_new RENAME TO prow_job_run_test_outputs;

-- Rename sequences (PostgreSQL automatically renames sequences with tables, but explicitly confirm)
ALTER SEQUENCE prow_job_runs_new_id_seq RENAME TO prow_job_runs_id_seq;
ALTER SEQUENCE prow_job_run_annotations_new_id_seq RENAME TO prow_job_run_annotations_id_seq;
ALTER SEQUENCE prow_job_run_tests_new_id_seq RENAME TO prow_job_run_tests_id_seq;
ALTER SEQUENCE prow_job_run_test_outputs_new_id_seq RENAME TO prow_job_run_test_outputs_id_seq;

-- Foreign keys are NOT recreated (see Referential Integrity section)

COMMIT;
```

## Referential Integrity Strategy

**Foreign key constraints will NOT be recreated** on the partitioned tables for the following reasons:

### Why Drop FKs

1. **Performance at Scale**: With nested LIST→RANGE daily partitioning across multiple releases, tables will have 1,000+ partitions (e.g., 10 releases × 365 days = 3,650 partitions). Each partition creates its own FK constraint trigger, causing significant INSERT/UPDATE overhead.

2. **Partition-Based Lifecycle**: When old partitions are detached/dropped (primary cleanup mechanism for partitioned tables), associated child data in dependent tables is automatically removed when their partitions are dropped. This provides implicit cascade deletion via partition lifecycle.

3. **Application-Level Integrity**: Sippy's data loader (`pkg/dataloader/prowloader/`) controls all writes and maintains referential integrity through:
   - Atomic transaction boundaries
   - Lookup of parent IDs before child inserts
   - GORM's association handling

4. **Write Throughput**: Sippy ingests high volumes of CI test results. FK validation across thousands of partitions would create a bottleneck.

### Integrity Guarantees

**Parent → Child relationships maintained through:**

1. **Application layer** (prowloader ensures parent exists before inserting children)
2. **Partition lifecycle** (dropping a prow_job_runs partition also drops matching prow_job_run_tests partitions)
3. **Composite primary keys** still enforce uniqueness and prevent duplicates
4. **Indexes on foreign key columns** (e.g., `idx_prow_job_run_tests_prow_job_run_id`) still enable efficient joins

**Still enforced at database level:**
- `prow_job_runs.prow_job_id → prow_jobs.id` (FK kept because prow_jobs is NOT partitioned - small table, no performance impact)
- All UNIQUE constraints
- All NOT NULL constraints
- Primary key constraints

### Monitoring

Add application-level checks to detect orphaned records:

```sql
-- Periodic validation: Find orphaned prow_job_run_tests
SELECT COUNT(*)
FROM prow_job_run_tests pjrt
LEFT JOIN prow_job_runs pjr 
    ON pjrt.prow_job_run_id = pjr.id
    AND pjrt.prow_job_run_release = pjr.prow_job_release
    AND pjrt.prow_job_run_timestamp = pjr.timestamp
WHERE pjr.id IS NULL;
```

Run as part of regular health checks or monitoring dashboard.

## Critical Files to Create/Modify

### New Migration Files (pkg/db/migrations/)
1. `000002_create_partitioned_tables.up.sql` — DDL for all 5 partitioned tables + indexes
2. `000002_create_partitioned_tables.down.sql` — Rollback DDL (drops all 5 tables)

### Standalone SQL Scripts (scripts/ or docs/sql/)
1. `migrate_prow_job_runs_data.sql` — Data migration for first group
2. `migrate_prow_job_run_tests_data.sql` — Data migration for tests
3. `migrate_prow_job_run_test_outputs_data.sql` — Data migration for outputs
4. `sync_sequences.sql` — Sequence syncing after data migration
5. `swap_tables.sql` — Atomic table swap with FK recreation

## Partition Management

**Partition creation is NOT included in these migration files.** The migration DDL only creates the parent table definitions with nested LIST→RANGE partitioning structure.

Actual partitions (release-level and daily sub-partitions) will be created and managed separately using the partition management system documented in `pkg/db/PARTITIONS_README.md`.

**Example partition structure** (created by partition management, not migrations):

```sql
-- Level 1: Release partition (LIST)
CREATE TABLE prow_job_runs_new_p4_18 PARTITION OF prow_job_runs_new
    FOR VALUES IN ('4.18')
    PARTITION BY RANGE (timestamp);

-- Level 2: Daily sub-partitions (RANGE) within 4.18
CREATE TABLE prow_job_runs_new_p4_18_2026_05_24 PARTITION OF prow_job_runs_new_p4_18
    FOR VALUES FROM ('2026-05-24') TO ('2026-05-25');

CREATE TABLE prow_job_runs_new_p4_18_2026_05_25 PARTITION OF prow_job_runs_new_p4_18
    FOR VALUES FROM ('2026-05-25') TO ('2026-05-26');
-- ... etc
```

**Migration DDL will include a DEFAULT partition** to catch any rows that don't match existing release partitions, preventing insert failures during the transition period.

## Validation Steps

After each migration step:

### 1. After DDL Migration (sippy migrate)
```sql
-- Verify partitioned tables exist
SELECT tablename FROM pg_tables WHERE tablename LIKE '%_new';

-- Verify partitions were created
SELECT
    parent.relname AS parent_table,
    child.relname AS partition_name
FROM pg_inherits
JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
JOIN pg_class child ON pg_inherits.inhrelid = child.oid
WHERE parent.relname IN (
    'prow_job_runs_new',
    'prow_job_run_tests_new',
    'prow_job_run_test_outputs_new',
    'prow_job_run_annotations_new',
    'prow_job_run_prow_pull_requests_new'
)
ORDER BY parent.relname, child.relname;

-- Verify indexes were created
SELECT indexname, tablename
FROM pg_indexes
WHERE tablename LIKE '%_new'
ORDER BY tablename, indexname;
```

### 2. After Data Migration
```sql
-- Verify row counts match
SELECT 'prow_job_runs' AS table_name,
    (SELECT COUNT(*) FROM prow_job_runs) AS old_count,
    (SELECT COUNT(*) FROM prow_job_runs_new) AS new_count;

SELECT 'prow_job_run_tests' AS table_name,
    (SELECT COUNT(*) FROM prow_job_run_tests) AS old_count,
    (SELECT COUNT(*) FROM prow_job_run_tests_new) AS new_count;

SELECT 'prow_job_run_test_outputs' AS table_name,
    (SELECT COUNT(*) FROM prow_job_run_test_outputs) AS old_count,
    (SELECT COUNT(*) FROM prow_job_run_test_outputs_new) AS new_count;

SELECT 'prow_job_run_annotations' AS table_name,
    (SELECT COUNT(*) FROM prow_job_run_annotations) AS old_count,
    (SELECT COUNT(*) FROM prow_job_run_annotations_new) AS new_count;

SELECT 'prow_job_run_prow_pull_requests' AS table_name,
    (SELECT COUNT(*) FROM prow_job_run_prow_pull_requests) AS old_count,
    (SELECT COUNT(*) FROM prow_job_run_prow_pull_requests_new) AS new_count;
```

### 3. After Sequence Sync
```sql
-- Verify sequences are synced correctly
SELECT 
    sequence_name,
    last_value
FROM information_schema.sequences
WHERE sequence_name LIKE '%_new_id_seq';
```

### 4. After Table Swap
```sql
-- Verify production tables are now partitioned
SELECT
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE tablename IN (
    'prow_job_runs',
    'prow_job_run_tests',
    'prow_job_run_test_outputs',
    'prow_job_run_annotations',
    'prow_job_run_prow_pull_requests'
);

-- Verify partition pruning is working
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM prow_job_run_tests
WHERE prow_job_run_timestamp > NOW() - INTERVAL '7 days'
  AND prow_job_run_release = '4.18';
-- Should show "Partitions pruned: N" in output
```

## Migration Execution Plan

### Step 1: Apply DDL Migrations (Non-disruptive)
```bash
# Run migration to create _new tables
go run ./cmd/sippy migrate --database-dsn "$SIPPY_PRODLIKE_DATABASE_DSN"
```
**Impact:** None — creates new empty tables alongside existing ones

### Step 2: Data Migration (Can Run Multiple Times)
```bash
# Optional: Stop sippy serve for faster migration (or leave running for incremental catch-up)
# Run data migration scripts (can be run multiple times - only migrates new data)
psql "$SIPPY_PRODLIKE_DATABASE_DSN" -f scripts/migrate_prow_job_runs_data.sql
psql "$SIPPY_PRODLIKE_DATABASE_DSN" -f scripts/migrate_prow_job_run_tests_data.sql
psql "$SIPPY_PRODLIKE_DATABASE_DSN" -f scripts/migrate_prow_job_run_test_outputs_data.sql
```
**Duration:** 
- Initial run: Depends on data size (estimate 1-2 hours for 816M rows in prow_job_run_tests)
- Subsequent runs: Only migrates new data since last run (much faster)

**Impact:** 
- Can run with sippy serve running (incremental catch-up pattern)
- Or stop sippy serve for faster bulk migration
- Scripts are idempotent - safe to re-run on failure or for catch-up

**Recommended approach:**
1. Run initial bulk migration with sippy serve stopped
2. Start sippy serve (app back online)
3. Re-run migration scripts periodically to catch up any new data inserted during migration
4. Final catch-up run before table swap

### Step 3: Sequence Sync
```bash
psql "$SIPPY_PRODLIKE_DATABASE_DSN" -f scripts/sync_sequences.sql
```
**Duration:** Seconds
**Impact:** None

### Step 4: Validation
```bash
# Run validation queries from section above
```

### Step 5: Atomic Table Swap (Brief Outage)
```bash
psql "$SIPPY_PRODLIKE_DATABASE_DSN" -f scripts/swap_tables.sql
```
**Duration:** Seconds (single transaction with table renames)
**Impact:** Brief write pause (1-2 seconds) while transaction commits

### Step 6: Restart and Verify
```bash
# Start sippy serve
go run ./cmd/sippy serve --database-dsn "$SIPPY_PRODLIKE_DATABASE_DSN"

# Verify partition pruning in application logs and EXPLAIN ANALYZE
```

## Rollback Plan

If issues are discovered after table swap:

```sql
BEGIN;

-- Rename partitioned tables back to _new
ALTER TABLE prow_job_runs RENAME TO prow_job_runs_new;
ALTER TABLE prow_job_run_tests RENAME TO prow_job_run_tests_new;
ALTER TABLE prow_job_run_test_outputs RENAME TO prow_job_run_test_outputs_new;
ALTER TABLE prow_job_run_annotations RENAME TO prow_job_run_annotations_new;
ALTER TABLE prow_job_run_prow_pull_requests RENAME TO prow_job_run_prow_pull_requests_new;

-- Restore old tables to production names
ALTER TABLE prow_job_runs_old RENAME TO prow_job_runs;
ALTER TABLE prow_job_run_tests_old RENAME TO prow_job_run_tests;
ALTER TABLE prow_job_run_test_outputs_old RENAME TO prow_job_run_test_outputs;
ALTER TABLE prow_job_run_annotations_old RENAME TO prow_job_run_annotations;
ALTER TABLE prow_job_run_prow_pull_requests_old RENAME TO prow_job_run_prow_pull_requests;

-- Rename sequences back
ALTER SEQUENCE prow_job_runs_id_seq RENAME TO prow_job_runs_new_id_seq;
ALTER SEQUENCE prow_job_run_annotations_id_seq RENAME TO prow_job_run_annotations_new_id_seq;
ALTER SEQUENCE prow_job_run_tests_id_seq RENAME TO prow_job_run_tests_new_id_seq;
ALTER SEQUENCE prow_job_run_test_outputs_id_seq RENAME TO prow_job_run_test_outputs_new_id_seq;

-- Recreate original foreign keys on old tables (if they were dropped)
ALTER TABLE prow_job_run_tests
    ADD CONSTRAINT fk_prow_job_run_tests_prow_job_run_id
    FOREIGN KEY (prow_job_run_id)
    REFERENCES prow_job_runs (id)
    ON DELETE CASCADE;

ALTER TABLE prow_job_run_test_outputs
    ADD CONSTRAINT fk_prow_job_run_test_outputs_prow_job_run_test_id
    FOREIGN KEY (prow_job_run_test_id)
    REFERENCES prow_job_run_tests (id)
    ON DELETE CASCADE;

ALTER TABLE prow_job_run_annotations
    ADD CONSTRAINT fk_prow_job_run_annotations_prow_job_run_id
    FOREIGN KEY (prow_job_run_id)
    REFERENCES prow_job_runs (id)
    ON DELETE CASCADE;

ALTER TABLE prow_job_run_prow_pull_requests
    ADD CONSTRAINT fk_prow_job_run_prow_pull_requests_prow_job_run_id
    FOREIGN KEY (prow_job_run_id)
    REFERENCES prow_job_runs (id)
    ON DELETE CASCADE;

COMMIT;
```

## Space Savings

By dropping unused indexes:
- `prow_job_run_tests`: ~27 GB (dropped deleted_at, created_at, suite_id indexes)
- `prow_job_runs`: ~15.5 MB (dropped timestamp_date, deleted_at indexes)

**Total estimated savings: ~27 GB**

## Future Work

After successful migration:

1. **Partition lifecycle management** — Implement partition creation for new releases and daily ranges
2. **Partition pruning** — Set up detach/drop workflow for partitions older than retention period (this naturally handles orphaned child data)
3. **Monitoring** — Add metrics for:
   - Partition count and size
   - Orphaned record detection queries (run weekly)
   - Write throughput improvement vs. old tables
4. **Documentation** — Update pkg/db/PARTITIONS_README.md with production partition management
5. **Cleanup** — Drop `_old` tables after 30-day safety period
6. **Optional: FK Re-evaluation** — If partition count stays low (<100) and write performance is not a concern, FKs can be added back later

## References

- `pkg/db/models/prow.go` — Model definitions
- `pkg/db/PARTITIONS_README.md` — Partition management API spec
- `pkg/db/MIGRATION_README.md` — Migration workflow spec
- `pkg/db/DB_UTILS_README.md` — Utility function spec
- `docs/plans/trt-1989-partitioning-prep.md` — Phase 1-3 background
- `docs/plans/trt-1989-phase3-query-optimization.md` — Query updates
- `.claude/db-index-usage-analysis.md` — Index usage analysis
