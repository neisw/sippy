-- TRT-1989 Phase 4: Rollback Partitioned Tables
--
-- Drops partitioned tables created in the up migration.
-- CASCADE ensures all partitions (if any were created) are also dropped.

DROP TABLE IF EXISTS prow_job_run_test_outputs CASCADE;
DROP TABLE IF EXISTS prow_job_run_tests CASCADE;
