-- Replace recoverable form API credentials with deterministic lookup hashes.
-- Existing client credentials remain valid because requests are hashed before lookup.
ALTER TABLE core_service.form_access_tokens
    RENAME COLUMN token TO token_hash;

UPDATE core_service.form_access_tokens
SET token_hash = encode(digest(token_hash, 'sha256'), 'hex');

ALTER TABLE core_service.form_access_tokens
    ALTER COLUMN token_hash TYPE VARCHAR(64),
    ADD CONSTRAINT chk_form_access_tokens_token_hash_length
        CHECK (length(token_hash) = 64);
