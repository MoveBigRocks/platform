package shareddomain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// =============================================================================
// Type-Safe Value (for rule conditions, custom fields, etc.)
// =============================================================================

// ValueType represents the type of a Value
type ValueType string

const (
	ValueTypeString   ValueType = "string"
	ValueTypeInt      ValueType = "int"
	ValueTypeFloat    ValueType = "float"
	ValueTypeBool     ValueType = "bool"
	ValueTypeTime     ValueType = "time"
	ValueTypeStrings  ValueType = "strings"
	ValueTypeDuration ValueType = "duration"
	ValueTypeNull     ValueType = "null"
)

// Value is a type-safe union type that can hold different value types
// without using interface{}. It's JSON-serializable and provides
// type-checked access to the underlying value.
type Value struct {
	typ ValueType

	// Storage fields - only one is used based on typ
	stringVal   string
	intVal      int64
	floatVal    float64
	boolVal     bool
	timeVal     time.Time
	stringsVal  []string
	durationVal time.Duration
}

// Type returns the value type
func (v Value) Type() ValueType {
	return v.typ
}

// IsZero returns true if the value is unset
func (v Value) IsZero() bool {
	return v.typ == "" || v.typ == ValueTypeNull
}

// Constructors for Value

// StringValue creates a string Value
func StringValue(s string) Value {
	return Value{typ: ValueTypeString, stringVal: s}
}

// IntValue creates an integer Value
func IntValue(i int64) Value {
	return Value{typ: ValueTypeInt, intVal: i}
}

// FloatValue creates a float Value
func FloatValue(f float64) Value {
	return Value{typ: ValueTypeFloat, floatVal: f}
}

// BoolValue creates a boolean Value
func BoolValue(b bool) Value {
	return Value{typ: ValueTypeBool, boolVal: b}
}

// TimeValue creates a time Value
func TimeValue(t time.Time) Value {
	return Value{typ: ValueTypeTime, timeVal: t}
}

// StringsValue creates a string slice Value
func StringsValue(s []string) Value {
	return Value{typ: ValueTypeStrings, stringsVal: s}
}

// DurationValue creates a duration Value
func DurationValue(d time.Duration) Value {
	return Value{typ: ValueTypeDuration, durationVal: d}
}

// NullValue creates a null Value
func NullValue() Value {
	return Value{typ: ValueTypeNull}
}

// Type-safe getters

// AsString returns the string value or empty string
func (v Value) AsString() string {
	switch v.typ {
	case ValueTypeString:
		return v.stringVal
	case ValueTypeInt:
		return strconv.FormatInt(v.intVal, 10)
	case ValueTypeFloat:
		return strconv.FormatFloat(v.floatVal, 'f', -1, 64)
	case ValueTypeBool:
		return strconv.FormatBool(v.boolVal)
	case ValueTypeTime:
		return v.timeVal.Format(time.RFC3339)
	case ValueTypeDuration:
		return v.durationVal.String()
	default:
		return ""
	}
}

// AsInt returns the int value or 0
func (v Value) AsInt() int64 {
	switch v.typ {
	case ValueTypeInt:
		return v.intVal
	case ValueTypeFloat:
		return int64(v.floatVal)
	case ValueTypeString:
		i, _ := strconv.ParseInt(v.stringVal, 10, 64)
		return i
	case ValueTypeBool:
		if v.boolVal {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// AsFloat returns the float value or 0
func (v Value) AsFloat() float64 {
	switch v.typ {
	case ValueTypeFloat:
		return v.floatVal
	case ValueTypeInt:
		return float64(v.intVal)
	case ValueTypeString:
		f, _ := strconv.ParseFloat(v.stringVal, 64)
		return f
	default:
		return 0
	}
}

// AsBool returns the bool value or false
func (v Value) AsBool() bool {
	switch v.typ {
	case ValueTypeBool:
		return v.boolVal
	case ValueTypeString:
		b, _ := strconv.ParseBool(v.stringVal)
		return b
	case ValueTypeInt:
		return v.intVal != 0
	case ValueTypeFloat:
		return v.floatVal != 0
	default:
		return false
	}
}

// AsTime returns the time value or zero time
func (v Value) AsTime() time.Time {
	switch v.typ {
	case ValueTypeTime:
		return v.timeVal
	case ValueTypeString:
		t, _ := time.Parse(time.RFC3339, v.stringVal)
		return t
	case ValueTypeInt:
		return time.Unix(v.intVal, 0)
	default:
		return time.Time{}
	}
}

// AsStrings returns the string slice value or nil
func (v Value) AsStrings() []string {
	switch v.typ {
	case ValueTypeStrings:
		return v.stringsVal
	case ValueTypeString:
		if v.stringVal == "" {
			return nil
		}
		return []string{v.stringVal}
	default:
		return nil
	}
}

// AsDuration returns the duration value or 0
func (v Value) AsDuration() time.Duration {
	switch v.typ {
	case ValueTypeDuration:
		return v.durationVal
	case ValueTypeInt:
		return time.Duration(v.intVal) * time.Second
	case ValueTypeString:
		d, _ := time.ParseDuration(v.stringVal)
		return d
	default:
		return 0
	}
}

// ToInterface converts Value back to interface{} for external interop.
// Use this when you need to pass typed values to APIs that require interface{}.
func (v Value) ToInterface() interface{} {
	switch v.typ {
	case ValueTypeString:
		return v.stringVal
	case ValueTypeInt:
		return v.intVal
	case ValueTypeFloat:
		return v.floatVal
	case ValueTypeBool:
		return v.boolVal
	case ValueTypeTime:
		return v.timeVal
	case ValueTypeStrings:
		return v.stringsVal
	case ValueTypeDuration:
		return v.durationVal
	case ValueTypeNull:
		return nil
	default:
		return nil
	}
}

// =============================================================================
// TryAs* Methods - Error-Aware Type Conversions
// =============================================================================
//
// These methods provide explicit error handling for type conversions, unlike
// the As* methods which silently return zero values on failure.
//
// Use TryAs* when:
//   - You need to distinguish between "value is zero" and "conversion failed"
//   - Validating user input or external data where errors should be surfaced
//   - Building rule conditions where misconfigured values should be detected
//
// Use As* when:
//   - Zero values are acceptable defaults
//   - The value type is already known (e.g., from IsString() check)
//   - Performance is critical and error handling overhead isn't needed

// TryAsInt attempts to convert the Value to int64.
// Returns an error if the conversion fails (e.g., non-numeric string).
func (v Value) TryAsInt() (int64, error) {
	switch v.typ {
	case ValueTypeInt:
		return v.intVal, nil
	case ValueTypeFloat:
		return int64(v.floatVal), nil
	case ValueTypeString:
		i, err := strconv.ParseInt(v.stringVal, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string %q to int: %w", v.stringVal, err)
		}
		return i, nil
	case ValueTypeBool:
		if v.boolVal {
			return 1, nil
		}
		return 0, nil
	case ValueTypeNull:
		return 0, fmt.Errorf("cannot convert null to int")
	default:
		return 0, fmt.Errorf("cannot convert %s to int", v.typ)
	}
}

// TryAsFloat attempts to convert the Value to float64.
// Returns an error if the conversion fails (e.g., non-numeric string).
func (v Value) TryAsFloat() (float64, error) {
	switch v.typ {
	case ValueTypeFloat:
		return v.floatVal, nil
	case ValueTypeInt:
		return float64(v.intVal), nil
	case ValueTypeString:
		f, err := strconv.ParseFloat(v.stringVal, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string %q to float: %w", v.stringVal, err)
		}
		return f, nil
	case ValueTypeNull:
		return 0, fmt.Errorf("cannot convert null to float")
	default:
		return 0, fmt.Errorf("cannot convert %s to float", v.typ)
	}
}

// TryAsBool attempts to convert the Value to bool.
// Returns an error if the conversion fails (e.g., non-boolean string other than "true"/"false").
func (v Value) TryAsBool() (bool, error) {
	switch v.typ {
	case ValueTypeBool:
		return v.boolVal, nil
	case ValueTypeString:
		b, err := strconv.ParseBool(v.stringVal)
		if err != nil {
			return false, fmt.Errorf("cannot convert string %q to bool: %w", v.stringVal, err)
		}
		return b, nil
	case ValueTypeInt:
		return v.intVal != 0, nil
	case ValueTypeFloat:
		return v.floatVal != 0, nil
	case ValueTypeNull:
		return false, fmt.Errorf("cannot convert null to bool")
	default:
		return false, fmt.Errorf("cannot convert %s to bool", v.typ)
	}
}

// TryAsTime attempts to convert the Value to time.Time.
// Returns an error if the conversion fails (e.g., invalid time format).
func (v Value) TryAsTime() (time.Time, error) {
	switch v.typ {
	case ValueTypeTime:
		return v.timeVal, nil
	case ValueTypeString:
		t, err := time.Parse(time.RFC3339, v.stringVal)
		if err != nil {
			return time.Time{}, fmt.Errorf("cannot convert string %q to time (expected RFC3339): %w", v.stringVal, err)
		}
		return t, nil
	case ValueTypeInt:
		return time.Unix(v.intVal, 0), nil
	case ValueTypeNull:
		return time.Time{}, fmt.Errorf("cannot convert null to time")
	default:
		return time.Time{}, fmt.Errorf("cannot convert %s to time", v.typ)
	}
}

// TryAsDuration attempts to convert the Value to time.Duration.
// Returns an error if the conversion fails (e.g., invalid duration string).
func (v Value) TryAsDuration() (time.Duration, error) {
	switch v.typ {
	case ValueTypeDuration:
		return v.durationVal, nil
	case ValueTypeInt:
		return time.Duration(v.intVal) * time.Second, nil
	case ValueTypeString:
		d, err := time.ParseDuration(v.stringVal)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string %q to duration: %w", v.stringVal, err)
		}
		return d, nil
	case ValueTypeNull:
		return 0, fmt.Errorf("cannot convert null to duration")
	default:
		return 0, fmt.Errorf("cannot convert %s to duration", v.typ)
	}
}

// Type predicates for checking Value type without conversion

// IsString returns true if the Value holds a string
func (v Value) IsString() bool {
	return v.typ == ValueTypeString
}

// IsInt returns true if the Value holds an integer
func (v Value) IsInt() bool {
	return v.typ == ValueTypeInt
}

// IsFloat returns true if the Value holds a float
func (v Value) IsFloat() bool {
	return v.typ == ValueTypeFloat
}

// IsBool returns true if the Value holds a boolean
func (v Value) IsBool() bool {
	return v.typ == ValueTypeBool
}

// IsTime returns true if the Value holds a time
func (v Value) IsTime() bool {
	return v.typ == ValueTypeTime
}

// IsStrings returns true if the Value holds a string slice
func (v Value) IsStrings() bool {
	return v.typ == ValueTypeStrings
}

// IsDuration returns true if the Value holds a duration
func (v Value) IsDuration() bool {
	return v.typ == ValueTypeDuration
}

// IsNull returns true if the Value is null/unset
func (v Value) IsNull() bool {
	return v.typ == ValueTypeNull || v.typ == ""
}

// IsNumeric returns true if the Value holds a numeric type (int or float)
func (v Value) IsNumeric() bool {
	return v.typ == ValueTypeInt || v.typ == ValueTypeFloat
}

// Equals compares two Values for equality
func (v Value) Equals(other Value) bool {
	if v.typ != other.typ {
		// Try string comparison for mixed types
		return v.AsString() == other.AsString()
	}

	switch v.typ {
	case ValueTypeString:
		return v.stringVal == other.stringVal
	case ValueTypeInt:
		return v.intVal == other.intVal
	case ValueTypeFloat:
		return v.floatVal == other.floatVal
	case ValueTypeBool:
		return v.boolVal == other.boolVal
	case ValueTypeTime:
		return v.timeVal.Equal(other.timeVal)
	case ValueTypeDuration:
		return v.durationVal == other.durationVal
	case ValueTypeStrings:
		if len(v.stringsVal) != len(other.stringsVal) {
			return false
		}
		for i, s := range v.stringsVal {
			if s != other.stringsVal[i] {
				return false
			}
		}
		return true
	case ValueTypeNull:
		return true
	default:
		return false
	}
}

// MarshalJSON implements json.Marshaler
func (v Value) MarshalJSON() ([]byte, error) {
	var val interface{}
	switch v.typ {
	case ValueTypeString:
		val = v.stringVal
	case ValueTypeInt:
		val = v.intVal
	case ValueTypeFloat:
		val = v.floatVal
	case ValueTypeBool:
		val = v.boolVal
	case ValueTypeTime:
		val = v.timeVal.Format(time.RFC3339)
	case ValueTypeStrings:
		val = v.stringsVal
	case ValueTypeDuration:
		val = v.durationVal.String()
	case ValueTypeNull:
		val = nil
	}
	return json.Marshal(map[string]interface{}{
		"type":  v.typ,
		"value": val,
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (v *Value) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Accept primitive JSON values in addition to the tagged object form.
		return v.unmarshalPrimitive(data)
	}

	typeData, ok := raw["type"]
	if !ok {
		return v.unmarshalPrimitive(data)
	}

	var typ ValueType
	if err := json.Unmarshal(typeData, &typ); err != nil {
		return v.unmarshalPrimitive(data)
	}

	v.typ = typ
	valueData := raw["value"]

	switch typ {
	case ValueTypeString:
		_ = json.Unmarshal(valueData, &v.stringVal)
	case ValueTypeInt:
		var n json.Number
		if err := json.Unmarshal(valueData, &n); err == nil {
			v.intVal, _ = n.Int64()
		}
	case ValueTypeFloat:
		var n json.Number
		if err := json.Unmarshal(valueData, &n); err == nil {
			v.floatVal, _ = n.Float64()
		}
	case ValueTypeBool:
		_ = json.Unmarshal(valueData, &v.boolVal)
	case ValueTypeTime:
		var s string
		if err := json.Unmarshal(valueData, &s); err == nil {
			v.timeVal, _ = time.Parse(time.RFC3339, s)
		}
	case ValueTypeStrings:
		var arr []string
		if err := json.Unmarshal(valueData, &arr); err == nil {
			v.stringsVal = arr
			break
		}
		var anyArr []interface{}
		if err := json.Unmarshal(valueData, &anyArr); err == nil {
			v.stringsVal = make([]string, len(anyArr))
			for i, item := range anyArr {
				v.stringsVal[i] = fmt.Sprintf("%v", item)
			}
		}
	case ValueTypeDuration:
		var s string
		if err := json.Unmarshal(valueData, &s); err == nil {
			v.durationVal, _ = time.ParseDuration(s)
		}
	}
	return nil
}

// unmarshalPrimitive handles primitive JSON values in addition to the tagged form.
func (v *Value) unmarshalPrimitive(data []byte) error {
	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*v = StringValue(s)
		return nil
	}

	// Try number
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		if f == float64(int64(f)) {
			*v = IntValue(int64(f))
		} else {
			*v = FloatValue(f)
		}
		return nil
	}

	// Try bool
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*v = BoolValue(b)
		return nil
	}

	// Try string array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*v = StringsValue(arr)
		return nil
	}

	// Null or unknown
	*v = NullValue()
	return nil
}

// =============================================================================
// Interface Conversion Utilities
// =============================================================================

// ValueFromInterface converts any interface{} to a type-safe Value.
// This is the single source of truth for interface{} → Value conversion.
// Use this instead of writing custom switch statements on interface{}.
func ValueFromInterface(v interface{}) Value {
	if v == nil {
		return NullValue()
	}
	switch val := v.(type) {
	case string:
		return StringValue(val)
	case int:
		return IntValue(int64(val))
	case int64:
		return IntValue(val)
	case int32:
		return IntValue(int64(val))
	case float64:
		return FloatValue(val)
	case float32:
		return FloatValue(float64(val))
	case bool:
		return BoolValue(val)
	case []string:
		return StringsValue(val)
	case []interface{}:
		strs := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				strs = append(strs, s)
			}
		}
		return StringsValue(strs)
	case Value:
		return val
	case time.Time:
		return TimeValue(val)
	case time.Duration:
		return DurationValue(val)
	default:
		// Fallback: convert to string representation
		return StringValue(fmt.Sprintf("%v", v))
	}
}

// MetadataFromMap converts map[string]interface{} to type-safe Metadata.
// Use this when receiving untyped maps from JSON or external sources.
func MetadataFromMap(m map[string]interface{}) Metadata {
	result := NewMetadata()
	for k, v := range m {
		result.Set(k, ValueFromInterface(v))
	}
	return result
}

// =============================================================================
// Type-Safe Change Tracking
// =============================================================================

// Change represents a type-safe field change
type Change[T comparable] struct {
	Field    string
	OldValue T
	NewValue T
}

// ChangeSet tracks multiple typed changes
type ChangeSet struct {
	changes []FieldChange
}

// FieldChange is a change to a specific field with type-safe values
type FieldChange struct {
	Field    string
	OldValue Value
	NewValue Value
}

// NewChangeSet creates a new change set
func NewChangeSet() *ChangeSet {
	return &ChangeSet{changes: make([]FieldChange, 0)}
}

// RecordString records a string field change
func (cs *ChangeSet) RecordString(field, oldVal, newVal string) {
	cs.changes = append(cs.changes, FieldChange{
		Field:    field,
		OldValue: StringValue(oldVal),
		NewValue: StringValue(newVal),
	})
}

// RecordInt records an int field change
func (cs *ChangeSet) RecordInt(field string, oldVal, newVal int64) {
	cs.changes = append(cs.changes, FieldChange{
		Field:    field,
		OldValue: IntValue(oldVal),
		NewValue: IntValue(newVal),
	})
}

// RecordBool records a bool field change
func (cs *ChangeSet) RecordBool(field string, oldVal, newVal bool) {
	cs.changes = append(cs.changes, FieldChange{
		Field:    field,
		OldValue: BoolValue(oldVal),
		NewValue: BoolValue(newVal),
	})
}

// RecordTime records a time field change
func (cs *ChangeSet) RecordTime(field string, oldVal, newVal time.Time) {
	cs.changes = append(cs.changes, FieldChange{
		Field:    field,
		OldValue: TimeValue(oldVal),
		NewValue: TimeValue(newVal),
	})
}

// Changes returns all recorded changes
func (cs *ChangeSet) Changes() []FieldChange {
	return cs.changes
}

// HasChanges returns true if any changes were recorded
func (cs *ChangeSet) HasChanges() bool {
	return len(cs.changes) > 0
}

// GetChange returns the change for a specific field
func (cs *ChangeSet) GetChange(field string) (FieldChange, bool) {
	for _, c := range cs.changes {
		if c.Field == field {
			return c, true
		}
	}
	return FieldChange{}, false
}

// MarshalJSON implements json.Marshaler
func (cs ChangeSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(cs.changes)
}

// UnmarshalJSON implements json.Unmarshaler
func (cs *ChangeSet) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &cs.changes)
}

// =============================================================================
// Typed Metadata (replaces map[string]interface{} for metadata)
// =============================================================================

// Metadata is a type-safe metadata container
type Metadata struct {
	data map[string]Value
}

// NewMetadata creates a new Metadata container
func NewMetadata() Metadata {
	return Metadata{data: make(map[string]Value)}
}

// Set stores a value
func (m *Metadata) Set(key string, value Value) {
	if m.data == nil {
		m.data = make(map[string]Value)
	}
	m.data[key] = value
}

// SetString stores a string value
func (m *Metadata) SetString(key, value string) {
	m.Set(key, StringValue(value))
}

// SetInt stores an int value
func (m *Metadata) SetInt(key string, value int64) {
	m.Set(key, IntValue(value))
}

// SetBool stores a bool value
func (m *Metadata) SetBool(key string, value bool) {
	m.Set(key, BoolValue(value))
}

// Get retrieves a value
func (m Metadata) Get(key string) (Value, bool) {
	v, ok := m.data[key]
	return v, ok
}

// GetString retrieves a string value
func (m Metadata) GetString(key string) string {
	if v, ok := m.data[key]; ok {
		return v.AsString()
	}
	return ""
}

// GetInt retrieves an int value
func (m Metadata) GetInt(key string) int64 {
	if v, ok := m.data[key]; ok {
		return v.AsInt()
	}
	return 0
}

// GetBool retrieves a bool value
func (m Metadata) GetBool(key string) bool {
	if v, ok := m.data[key]; ok {
		return v.AsBool()
	}
	return false
}

// Has checks if key exists
func (m Metadata) Has(key string) bool {
	_, ok := m.data[key]
	return ok
}

// Delete removes a key
func (m *Metadata) Delete(key string) {
	delete(m.data, key)
}

// Keys returns all keys
func (m Metadata) Keys() []string {
	keys := make([]string, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of entries
func (m Metadata) Len() int {
	return len(m.data)
}

// IsEmpty returns true if the metadata has no entries
func (m Metadata) IsEmpty() bool {
	return len(m.data) == 0
}

// ToInterfaceMap converts Metadata back to map[string]interface{} for external interop.
// Use this when you need to pass typed metadata to APIs that require map[string]interface{}.
func (m Metadata) ToInterfaceMap() map[string]interface{} {
	if m.data == nil {
		return make(map[string]interface{})
	}
	result := make(map[string]interface{}, len(m.data))
	for k, v := range m.data {
		result[k] = v.ToInterface()
	}
	return result
}

// Clone creates a deep copy of the Metadata
func (m Metadata) Clone() Metadata {
	if m.data == nil {
		return NewMetadata()
	}
	result := NewMetadata()
	for k, v := range m.data {
		result.data[k] = v
	}
	return result
}

// Merge combines another Metadata into this one (other takes precedence)
func (m *Metadata) Merge(other Metadata) {
	if m.data == nil {
		m.data = make(map[string]Value)
	}
	for k, v := range other.data {
		m.data[k] = v
	}
}

// MarshalJSON implements json.Marshaler
func (m Metadata) MarshalJSON() ([]byte, error) {
	if m.data == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(m.data)
}

// UnmarshalJSON implements json.Unmarshaler
func (m *Metadata) UnmarshalJSON(data []byte) error {
	if m.data == nil {
		m.data = make(map[string]Value)
	}
	return json.Unmarshal(data, &m.data)
}
