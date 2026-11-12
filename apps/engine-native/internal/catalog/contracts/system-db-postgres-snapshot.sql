WITH observed AS MATERIALIZED (
 SELECT * FROM pg_stat_database WHERE datname=current_database()
)
SELECT json_build_object(
 'capture_start_ns', (extract(epoch FROM statement_timestamp())*1000000000)::numeric(30,0)::text,
 'capture_end_ns', (extract(epoch FROM clock_timestamp())*1000000000)::numeric(30,0)::text,
 'postmaster_start_ns',(extract(epoch FROM pg_postmaster_start_time())*1000000000)::numeric(30,0)::text,
 'stats_reset_ns',(extract(epoch FROM stats_reset)*1000000000)::numeric(30,0)::text,
 'numbackends',numbackends::text,
 'xact_commit',xact_commit::text,'xact_rollback',xact_rollback::text,
 'blks_read',blks_read::text,'blks_hit',blks_hit::text,
 'tup_returned',tup_returned::text,'tup_fetched',tup_fetched::text,
 'tup_inserted',tup_inserted::text,'tup_updated',tup_updated::text,'tup_deleted',tup_deleted::text,
 'conflicts',conflicts::text,'temp_files',temp_files::text,'temp_bytes',temp_bytes::text,
 'deadlocks',deadlocks::text,'sessions',sessions::text
) FROM observed;
