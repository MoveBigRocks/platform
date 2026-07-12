-- =============================================================================
-- Provision the mbr_admin cross-workspace bypass role
-- =============================================================================
-- Run as a superuser (the application role cannot create roles). Creates the
-- BYPASSRLS admin role, grants it access to the core schemas, and grants it to
-- the application role so WithAdminContext can SET LOCAL ROLE mbr_admin for
-- cross-workspace work under enforced row-level security. Idempotent.
--
-- The application process resolves the admin role once at startup, so restart
-- the serving processes after running this and before enabling RLS.
-- =============================================================================

-- =============================================================================
-- ADMIN ROLE FOR CROSS-TENANT OPERATIONS
-- =============================================================================
-- Workers that need to query across all workspaces (e.g., auto-close, notifications)
-- use this role to bypass RLS. The application switches to this role using:
--   SET LOCAL ROLE mbr_admin;
-- within a transaction.

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'mbr_admin') THEN
        BEGIN
            CREATE ROLE mbr_admin;
        EXCEPTION
            WHEN insufficient_privilege THEN
                RAISE NOTICE 'Skipping CREATE ROLE mbr_admin: %', SQLERRM;
        END;
    END IF;
EXCEPTION
    WHEN duplicate_object THEN
        NULL;
END
$$;

-- Grant mbr_admin the ability to bypass RLS
DO $$
BEGIN
    ALTER ROLE mbr_admin BYPASSRLS;
EXCEPTION
    WHEN insufficient_privilege THEN
        RAISE NOTICE 'Skipping ALTER ROLE mbr_admin BYPASSRLS: %', SQLERRM;
    WHEN OTHERS THEN
        IF SQLERRM LIKE '%tuple concurrently updated%' THEN
            RAISE NOTICE 'Skipping concurrent ALTER ROLE mbr_admin BYPASSRLS';
        ELSE
            RAISE;
        END IF;
END
$$;

-- Grant mbr_admin permission to access all core schemas when the role exists.
DO $$
DECLARE
    schema_name TEXT;
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'mbr_admin') THEN
        FOREACH schema_name IN ARRAY ARRAY[
            'public',
            'core_infra',
            'core_identity',
            'core_platform',
            'core_service',
            'core_automation',
            'core_knowledge',
            'core_governance',
            'core_extension_runtime'
        ]
        LOOP
            IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = schema_name) THEN
                EXECUTE format('GRANT USAGE ON SCHEMA %I TO mbr_admin', schema_name);
                EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %I TO mbr_admin', schema_name);
                EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO mbr_admin', schema_name);
            END IF;
        END LOOP;
    ELSE
        RAISE NOTICE 'Skipping mbr_admin schema grants because the role does not exist';
    END IF;
EXCEPTION
    WHEN insufficient_privilege THEN
        RAISE NOTICE 'Skipping some mbr_admin grants: %', SQLERRM;
END
$$;

-- Allow the application database user to switch to mbr_admin role
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'mbr') THEN
        GRANT mbr_admin TO mbr;
    END IF;
EXCEPTION
    WHEN OTHERS THEN
        IF SQLERRM LIKE '%tuple concurrently updated%' THEN
            RAISE NOTICE 'Skipping concurrent GRANT mbr_admin TO mbr';
        ELSE
            RAISE;
        END IF;
END
$$;

-- Also grant mbr_admin to the current database user when possible. Production
-- environments may not use a username literally named "mbr".
DO $$
BEGIN
    EXECUTE format('GRANT mbr_admin TO %I', current_user);
EXCEPTION
    WHEN OTHERS THEN
        IF SQLERRM LIKE '%tuple concurrently updated%' THEN
            RAISE NOTICE 'Skipping concurrent GRANT mbr_admin TO %', current_user;
        ELSE
            RAISE NOTICE 'Could not grant mbr_admin to %: %', current_user, SQLERRM;
        END IF;
END
$$;

-- Verify the role can be used in environments where the grant succeeds.
DO $$
BEGIN
    SET LOCAL ROLE mbr_admin;
    RESET ROLE;
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING 'Cannot switch to mbr_admin role: %. Admin panel may not work correctly.', SQLERRM;
END
$$;

-- =============================================================================
-- COMPREHENSIVE GRANTS: sequences and functions
-- =============================================================================
-- WithAdminContext runs admin writes as mbr_admin, which is neither a superuser
-- nor the table owner, so it needs every privilege the application role uses,
-- not just table CRUD. Grant sequence usage (for any serial defaults) and
-- function execution on the core schemas, plus default privileges so objects
-- created later are covered.
DO $$
DECLARE
    schema_name TEXT;
BEGIN
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'mbr_admin') THEN
        FOREACH schema_name IN ARRAY ARRAY[
            'public',
            'core_infra',
            'core_identity',
            'core_platform',
            'core_service',
            'core_automation',
            'core_knowledge',
            'core_governance',
            'core_extension_runtime'
        ]
        LOOP
            IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = schema_name) THEN
                EXECUTE format('GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA %I TO mbr_admin', schema_name);
                EXECUTE format('GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %I TO mbr_admin', schema_name);
                EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT USAGE, SELECT ON SEQUENCES TO mbr_admin', schema_name);
                EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT EXECUTE ON FUNCTIONS TO mbr_admin', schema_name);
            END IF;
        END LOOP;
    END IF;
EXCEPTION
    WHEN insufficient_privilege THEN
        RAISE NOTICE 'Skipping some mbr_admin sequence/function grants: %', SQLERRM;
END
$$;
