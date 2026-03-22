package sql

import (
	"strings"

	"github.com/google/uuid"
)

// normalizePersistedUUID keeps only explicit canonical UUID overrides.
// Empty values and non-UUID placeholders are treated as "let PostgreSQL
// generate the row ID".
func normalizePersistedUUID(id *string) {
	if id == nil {
		return
	}

	value := strings.TrimSpace(*id)
	if value == "" {
		*id = ""
		return
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		*id = ""
		return
	}

	*id = parsed.String()
}

// nullableUUIDValue preserves explicit UUID values and maps blank strings to
// SQL NULL for nullable UUID columns.
func nullableUUIDValue(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return value
	}

	return parsed.String()
}

// nullableLegacyUUIDValue is reserved for optional audit/actor columns that
// historically accepted symbolic placeholders such as "system". Blank and
// non-UUID values are persisted as SQL NULL.
func nullableLegacyUUIDValue(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}

	return parsed.String()
}

func nullableUUIDPtrValue(value *string) interface{} {
	if value == nil {
		return nil
	}

	return nullableUUIDValue(*value)
}

func nullableLegacyUUIDPtrValue(value *string) interface{} {
	if value == nil {
		return nil
	}

	return nullableLegacyUUIDValue(*value)
}
