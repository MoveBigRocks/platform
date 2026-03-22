package sql

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	shareddomain "github.com/movebigrocks/platform/internal/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

// marshalJSONString marshals a value to a JSON string.
// Returns empty string for nil values.
func marshalJSONString(v interface{}, fieldName string) (string, error) {
	if v == nil {
		return "", nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal %s: %w", fieldName, err)
	}
	return string(data), nil
}

// unmarshalJSONField unmarshals a JSON string into the target.
// Logs warnings on errors but does not fail - leaves target unchanged.
func unmarshalJSONField(jsonStr string, target interface{}, table, field string) {
	if jsonStr == "" {
		return
	}
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		log := logger.New()
		log.Warn("Failed to unmarshal JSON field",
			"table", table,
			"field", field,
			"error", err)
	}
}

// unmarshalJSONBytes unmarshals JSON bytes into the target.
// Logs warnings on errors but does not fail - leaves target unchanged.
func unmarshalJSONBytes(data []byte, target interface{}, table, field string) {
	if len(data) == 0 {
		return
	}
	if err := json.Unmarshal(data, target); err != nil {
		log := logger.New()
		log.Warn("Failed to unmarshal JSON bytes",
			"table", table,
			"field", field,
			"error", err)
	}
}

// unmarshalMetadataOrEmpty unmarshals a JSON string into shareddomain.Metadata.
// Returns empty Metadata on error or empty input.
func unmarshalMetadataOrEmpty(jsonStr string, table, field string) shareddomain.Metadata {
	if jsonStr == "" {
		return shareddomain.NewMetadata()
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		log := logger.New()
		log.Warn("Failed to unmarshal metadata",
			"table", table,
			"field", field,
			"error", err)
		return shareddomain.NewMetadata()
	}
	return shareddomain.MetadataFromMap(m)
}

// unmarshalTypedSchemaOrEmpty unmarshals a JSON string into shareddomain.TypedSchema.
// Returns empty TypedSchema on error or empty input.
func unmarshalTypedSchemaOrEmpty(jsonStr string, table, field string) shareddomain.TypedSchema {
	if jsonStr == "" {
		return shareddomain.NewTypedSchema()
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		log := logger.New()
		log.Warn("Failed to unmarshal typed schema",
			"table", table,
			"field", field,
			"error", err)
		return shareddomain.NewTypedSchema()
	}
	return shareddomain.TypedSchemaFromMap(m)
}

// unmarshalTypedSchemaSliceOrEmpty unmarshals a JSON array of objects into TypedSchema values.
func unmarshalTypedSchemaSliceOrEmpty(jsonStr string, table, field string) []shareddomain.TypedSchema {
	if jsonStr == "" {
		return []shareddomain.TypedSchema{}
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		log := logger.New()
		log.Warn("Failed to unmarshal typed schema slice",
			"table", table,
			"field", field,
			"error", err)
		return []shareddomain.TypedSchema{}
	}

	result := make([]shareddomain.TypedSchema, len(items))
	for i, item := range items {
		result[i] = shareddomain.TypedSchemaFromMap(item)
	}
	return result
}

// unmarshalTypedCustomFieldsOrEmpty unmarshals a JSON string into TypedCustomFields.
// Returns empty TypedCustomFields on error or empty input.
func unmarshalTypedCustomFieldsOrEmpty(jsonStr string, table, field string) shareddomain.TypedCustomFields {
	result := shareddomain.TypedCustomFields{}
	if jsonStr == "" {
		return result
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log := logger.New()
		log.Warn("Failed to unmarshal typed custom fields",
			"table", table,
			"field", field,
			"error", err)
	}
	return result
}

// unmarshalStringArrayField converts a PostgreSQL text[] string into a string slice.
func unmarshalStringArrayField(arrayStr string, table, field string) []string {
	if strings.TrimSpace(arrayStr) == "" {
		return []string{}
	}

	result := []string{}
	if err := pq.Array(&result).Scan(arrayStr); err != nil {
		log := logger.New()
		log.Warn("Failed to unmarshal string array field",
			"table", table,
			"field", field,
			"error", err)
		return []string{}
	}
	return result
}

// BulkMarshaler helps marshal multiple fields and collect errors.
type BulkMarshaler struct {
	context string
	errors  []string
	results map[string]string
}

// NewBulkMarshaler creates a new BulkMarshaler.
func NewBulkMarshaler(context string) *BulkMarshaler {
	return &BulkMarshaler{
		context: context,
		results: make(map[string]string),
	}
}

// Add marshals a value and stores the result.
func (m *BulkMarshaler) Add(name string, value interface{}) *BulkMarshaler {
	result, err := marshalJSONString(value, name)
	if err != nil {
		m.errors = append(m.errors, fmt.Sprintf("%s: %v", name, err))
	} else {
		m.results[name] = result
	}
	return m
}

// Get returns the marshaled string for a field.
func (m *BulkMarshaler) Get(name string) string {
	return m.results[name]
}

// Error returns combined error if any marshaling failed.
func (m *BulkMarshaler) Error() error {
	if len(m.errors) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s", m.context, strings.Join(m.errors, "; "))
}

// buildInQuery expands a query containing IN (?) with the given slice of values.
// Uses sqlx.In to handle the placeholder expansion for SQLite.
func buildInQuery(query string, args interface{}) (string, []interface{}, error) {
	q, qArgs, err := sqlx.In(query, args)
	if err != nil {
		return "", nil, fmt.Errorf("build IN query: %w", err)
	}
	// SQLite uses ? placeholders (already the default from sqlx.In)
	return q, qArgs, nil
}
