-- Full Fortuna database reset for a clean redeploy (dev/test only).
-- Destructive: removes every object and row in the public schema, including
-- users, vulnerability catalogs, and schema_migrations. Keep a verified dump
-- before running this file. Core recreates the application schema on startup.
-- Only run against the dedicated database named fortuna. The PostgreSQL PVC,
-- database, roles, and Kubernetes Secrets are not removed.

BEGIN;

DO $$
BEGIN
  IF current_database() <> 'fortuna' THEN
    RAISE EXCEPTION 'Refusing to reset database %; expected fortuna', current_database();
  END IF;
END
$$;

DROP SCHEMA IF EXISTS public CASCADE;
CREATE SCHEMA public AUTHORIZATION pg_database_owner;
GRANT USAGE ON SCHEMA public TO PUBLIC;

COMMIT;
