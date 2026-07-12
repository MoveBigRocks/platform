-- Public form handlers resolve a high-entropy public key before a workspace
-- context exists. Permit the application role to read active public form
-- definitions while keeping every write and all private forms tenant-scoped.
DROP POLICY IF EXISTS public_form_lookup ON core_service.form_specs;
CREATE POLICY public_form_lookup ON core_service.form_specs
    FOR SELECT
    USING (
        is_public = TRUE
        AND status = 'active'
        AND public_key IS NOT NULL
        AND deleted_at IS NULL
    );
