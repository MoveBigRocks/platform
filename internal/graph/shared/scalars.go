package shared

import (
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// DateTime Scalar
// =============================================================================

// DateTime is a custom scalar for time values in GraphQL
type DateTime struct {
	time.Time
}

// ImplementsGraphQLType maps this Go type to the GraphQL scalar type
func (DateTime) ImplementsGraphQLType(name string) bool {
	return name == "DateTime"
}

// UnmarshalGraphQL parses the GraphQL scalar value
func (t *DateTime) UnmarshalGraphQL(input interface{}) error {
	switch input := input.(type) {
	case time.Time:
		t.Time = input
		return nil
	case string:
		var err error
		t.Time, err = time.Parse(time.RFC3339, input)
		if err != nil {
			return fmt.Errorf("DateTime must be RFC3339 formatted: %w", err)
		}
		return nil
	case int64:
		t.Time = time.Unix(input, 0)
		return nil
	case float64:
		t.Time = time.Unix(int64(input), 0)
		return nil
	default:
		return fmt.Errorf("wrong type for DateTime: %T", input)
	}
}

// MarshalJSON implements json.Marshaler
func (t DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Time.Format(time.RFC3339))
}

// =============================================================================
// JSON Scalar
// =============================================================================

// JSON is a custom scalar for arbitrary JSON data
type JSON map[string]interface{}

// ImplementsGraphQLType maps this Go type to the GraphQL scalar type
func (JSON) ImplementsGraphQLType(name string) bool {
	return name == "JSON"
}

// UnmarshalGraphQL parses the GraphQL scalar value
func (j *JSON) UnmarshalGraphQL(input interface{}) error {
	switch val := input.(type) {
	case map[string]interface{}:
		*j = JSON(val)
		return nil
	case string:
		var result JSON
		if err := json.Unmarshal([]byte(val), &result); err != nil {
			return fmt.Errorf("JSON must be a valid JSON object: %w", err)
		}
		*j = result
		return nil
	case nil:
		*j = nil
		return nil
	default:
		return fmt.Errorf("wrong type for JSON: %T", input)
	}
}

// MarshalJSON implements json.Marshaler
func (j JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]interface{}(j))
}

// ToMap converts JSON to a regular map
func (j JSON) ToMap() map[string]interface{} {
	return map[string]interface{}(j)
}
