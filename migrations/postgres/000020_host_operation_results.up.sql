-- Idempotency ledger for coarse host-API operations. A first-party extension's
-- coarse operation (for example ingesting an application into a core contact and
-- case) writes several core rows in one transaction and returns their ids; a
-- retry must return the same ids without duplicating the rows. The core records
-- the operation result keyed by the extension's idempotency key, so a repeated
-- call returns the stored result instead of doing the work again. The ledger is
-- workspace-scoped and row-level-security isolated like the entities it keys.

CREATE TABLE IF NOT EXISTS core_platform.host_operation_results (
    workspace_id    UUID NOT NULL,
    extension_id    UUID NOT NULL,
    operation       TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    result          JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, extension_id, operation, idempotency_key)
);

ALTER TABLE core_platform.host_operation_results ENABLE ROW LEVEL SECURITY;
ALTER TABLE core_platform.host_operation_results FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON core_platform.host_operation_results;
CREATE POLICY tenant_isolation ON core_platform.host_operation_results
    FOR ALL USING (workspace_id = public.current_workspace_id());
