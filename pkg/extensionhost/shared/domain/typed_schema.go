package shareddomain

import (
	"encoding/json"
)

// =============================================================================
// TypedSchema - Type-safe wrapper for JSON Schema data
// =============================================================================

// TypedSchema wraps JSON Schema data with type-safe access.
// The underlying data is still dynamic (JSON Schema spec requires flexibility),
// but access is controlled through typed methods.
type TypedSchema struct {
	data map[string]interface{}
}

// NewTypedSchema creates a new empty TypedSchema
func NewTypedSchema() TypedSchema {
	return TypedSchema{data: make(map[string]interface{})}
}

// TypedSchemaFromMap creates a TypedSchema from an existing map
func TypedSchemaFromMap(m map[string]interface{}) TypedSchema {
	if m == nil {
		return NewTypedSchema()
	}
	return TypedSchema{data: m}
}

// =============================================================================
// JSON Schema Navigation Methods
// =============================================================================

// GetField returns schema definition for a specific field from "properties"
func (s TypedSchema) GetField(name string) (FieldSchema, bool) {
	props, ok := s.data["properties"].(map[string]interface{})
	if !ok {
		return FieldSchema{}, false
	}
	field, ok := props[name].(map[string]interface{})
	if !ok {
		return FieldSchema{}, false
	}
	return FieldSchemaFromMap(field), true
}

// Required returns the list of required field names
func (s TypedSchema) Required() []string {
	required, ok := s.data["required"].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(required))
	for _, r := range required {
		if str, ok := r.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

// Type returns the schema type (object, array, string, etc.)
func (s TypedSchema) Type() string {
	if t, ok := s.data["type"].(string); ok {
		return t
	}
	return ""
}

// Title returns the schema title
func (s TypedSchema) Title() string {
	if t, ok := s.data["title"].(string); ok {
		return t
	}
	return ""
}

// Description returns the schema description
func (s TypedSchema) Description() string {
	if d, ok := s.data["description"].(string); ok {
		return d
	}
	return ""
}

// Properties returns all field names in the schema
func (s TypedSchema) Properties() []string {
	props, ok := s.data["properties"].(map[string]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(props))
	for k := range props {
		result = append(result, k)
	}
	return result
}

// HasProperty checks if a property exists in the schema
func (s TypedSchema) HasProperty(name string) bool {
	props, ok := s.data["properties"].(map[string]interface{})
	if !ok {
		return false
	}
	_, exists := props[name]
	return exists
}

// IsRequired checks if a field is in the required list
func (s TypedSchema) IsRequired(name string) bool {
	for _, r := range s.Required() {
		if r == name {
			return true
		}
	}
	return false
}

// =============================================================================
// Modification Methods
// =============================================================================

// Set stores a value at the given key
func (s *TypedSchema) Set(key string, value interface{}) {
	if s.data == nil {
		s.data = make(map[string]interface{})
	}
	s.data[key] = value
}

// Get retrieves a raw value by key
func (s TypedSchema) Get(key string) (interface{}, bool) {
	v, ok := s.data[key]
	return v, ok
}

// GetString retrieves a string value by key
func (s TypedSchema) GetString(key string) string {
	if v, ok := s.data[key].(string); ok {
		return v
	}
	return ""
}

// GetBool retrieves a boolean value by key
func (s TypedSchema) GetBool(key string) bool {
	if v, ok := s.data[key].(bool); ok {
		return v
	}
	return false
}

// =============================================================================
// Interop Methods
// =============================================================================

// ToMap returns the underlying map for map-oriented call sites.
// Use sparingly - prefer typed access methods.
func (s TypedSchema) ToMap() map[string]interface{} {
	if s.data == nil {
		return make(map[string]interface{})
	}
	return s.data
}

// IsEmpty returns true if the schema has no data
func (s TypedSchema) IsEmpty() bool {
	return len(s.data) == 0
}

// Clone creates a deep copy of the schema
func (s TypedSchema) Clone() TypedSchema {
	if s.data == nil {
		return NewTypedSchema()
	}
	// Deep copy via JSON round-trip
	bytes, err := json.Marshal(s.data)
	if err != nil {
		return NewTypedSchema()
	}
	var copied map[string]interface{}
	if err := json.Unmarshal(bytes, &copied); err != nil {
		return NewTypedSchema()
	}
	return TypedSchema{data: copied}
}

// =============================================================================
// JSON Serialization
// =============================================================================

// MarshalJSON implements json.Marshaler
func (s TypedSchema) MarshalJSON() ([]byte, error) {
	if s.data == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(s.data)
}

// UnmarshalJSON implements json.Unmarshaler
func (s *TypedSchema) UnmarshalJSON(data []byte) error {
	if s.data == nil {
		s.data = make(map[string]interface{})
	}
	return json.Unmarshal(data, &s.data)
}

// =============================================================================
// FieldSchema - Schema definition for a single field
// =============================================================================

// FieldSchema represents the schema for a single form field
type FieldSchema struct {
	data map[string]interface{}
}

// FieldSchemaFromMap creates a FieldSchema from a map
func FieldSchemaFromMap(m map[string]interface{}) FieldSchema {
	if m == nil {
		return FieldSchema{data: make(map[string]interface{})}
	}
	return FieldSchema{data: m}
}

// Type returns the field type (string, number, boolean, array, object)
func (f FieldSchema) Type() string {
	if t, ok := f.data["type"].(string); ok {
		return t
	}
	return ""
}

// Title returns the field title
func (f FieldSchema) Title() string {
	if t, ok := f.data["title"].(string); ok {
		return t
	}
	return ""
}

// Description returns the field description
func (f FieldSchema) Description() string {
	if d, ok := f.data["description"].(string); ok {
		return d
	}
	return ""
}

// Default returns the default value
func (f FieldSchema) Default() interface{} {
	return f.data["default"]
}

// Format returns the format hint (email, date, uri, etc.)
func (f FieldSchema) Format() string {
	if fmt, ok := f.data["format"].(string); ok {
		return fmt
	}
	return ""
}

// MinLength returns minimum string length (for string type)
func (f FieldSchema) MinLength() int {
	if v, ok := f.data["minLength"].(float64); ok {
		return int(v)
	}
	return 0
}

// MaxLength returns maximum string length (for string type)
func (f FieldSchema) MaxLength() int {
	if v, ok := f.data["maxLength"].(float64); ok {
		return int(v)
	}
	return 0
}

// Minimum returns minimum value (for number type)
func (f FieldSchema) Minimum() float64 {
	if v, ok := f.data["minimum"].(float64); ok {
		return v
	}
	return 0
}

// Maximum returns maximum value (for number type)
func (f FieldSchema) Maximum() float64 {
	if v, ok := f.data["maximum"].(float64); ok {
		return v
	}
	return 0
}

// Pattern returns the regex pattern (for string type)
func (f FieldSchema) Pattern() string {
	if p, ok := f.data["pattern"].(string); ok {
		return p
	}
	return ""
}

// Enum returns the list of allowed values
func (f FieldSchema) Enum() []interface{} {
	if e, ok := f.data["enum"].([]interface{}); ok {
		return e
	}
	return nil
}

// EnumStrings returns enum values as strings (convenience method)
func (f FieldSchema) EnumStrings() []string {
	enum := f.Enum()
	if enum == nil {
		return nil
	}
	result := make([]string, 0, len(enum))
	for _, e := range enum {
		if str, ok := e.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

// IsReadOnly returns true if the field is read-only
func (f FieldSchema) IsReadOnly() bool {
	if v, ok := f.data["readOnly"].(bool); ok {
		return v
	}
	return false
}

// ToMap returns the underlying map
func (f FieldSchema) ToMap() map[string]interface{} {
	return f.data
}
