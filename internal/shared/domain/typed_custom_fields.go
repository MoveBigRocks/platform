package shareddomain

import (
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// Typed Custom Fields System
// Replaces map[string]interface{} for CustomFields with type-safe access
// =============================================================================

// TypedCustomFields provides type-safe storage and access for custom fields.
// It keeps the JSON surface compact while providing compile-time type safety.
type TypedCustomFields struct {
	fields map[string]CustomFieldEntry
}

// CustomFieldEntry stores a custom field value with its type information
type CustomFieldEntry struct {
	Value    Value
	FieldID  string
	DataType DataType
}

// NewTypedCustomFields creates a new typed custom fields container
func NewTypedCustomFields() TypedCustomFields {
	return TypedCustomFields{
		fields: make(map[string]CustomFieldEntry),
	}
}

// SetString sets a string custom field
func (tcf *TypedCustomFields) SetString(name, value string) {
	tcf.ensureInit()
	tcf.fields[name] = CustomFieldEntry{
		Value:    StringValue(value),
		DataType: DataTypeString,
	}
}

// SetInt sets an integer custom field
func (tcf *TypedCustomFields) SetInt(name string, value int64) {
	tcf.ensureInit()
	tcf.fields[name] = CustomFieldEntry{
		Value:    IntValue(value),
		DataType: DataTypeNumber,
	}
}

// SetFloat sets a float custom field
func (tcf *TypedCustomFields) SetFloat(name string, value float64) {
	tcf.ensureInit()
	tcf.fields[name] = CustomFieldEntry{
		Value:    FloatValue(value),
		DataType: DataTypeNumber,
	}
}

// SetBool sets a boolean custom field
func (tcf *TypedCustomFields) SetBool(name string, value bool) {
	tcf.ensureInit()
	tcf.fields[name] = CustomFieldEntry{
		Value:    BoolValue(value),
		DataType: DataTypeBoolean,
	}
}

// SetTime sets a time custom field
func (tcf *TypedCustomFields) SetTime(name string, value time.Time) {
	tcf.ensureInit()
	tcf.fields[name] = CustomFieldEntry{
		Value:    TimeValue(value),
		DataType: DataTypeDate,
	}
}

// SetStrings sets a string array custom field
func (tcf *TypedCustomFields) SetStrings(name string, value []string) {
	tcf.ensureInit()
	tcf.fields[name] = CustomFieldEntry{
		Value:    StringsValue(value),
		DataType: DataTypeArray,
	}
}

// SetAny sets a custom field from any value.
// It infers the type from the value.
func (tcf *TypedCustomFields) SetAny(name string, value interface{}) {
	tcf.ensureInit()
	switch v := value.(type) {
	case string:
		tcf.SetString(name, v)
	case int:
		tcf.SetInt(name, int64(v))
	case int64:
		tcf.SetInt(name, v)
	case int32:
		tcf.SetInt(name, int64(v))
	case float64:
		tcf.SetFloat(name, v)
	case float32:
		tcf.SetFloat(name, float64(v))
	case bool:
		tcf.SetBool(name, v)
	case time.Time:
		tcf.SetTime(name, v)
	case []string:
		tcf.SetStrings(name, v)
	case nil:
		// Don't store nil values
		delete(tcf.fields, name)
	default:
		// For unknown types, convert to string
		tcf.SetString(name, fmt.Sprintf("%v", v))
	}
}

// GetString retrieves a string custom field
func (tcf TypedCustomFields) GetString(name string) (string, bool) {
	if entry, ok := tcf.fields[name]; ok {
		return entry.Value.AsString(), true
	}
	return "", false
}

// GetInt retrieves an integer custom field
func (tcf TypedCustomFields) GetInt(name string) (int64, bool) {
	if entry, ok := tcf.fields[name]; ok {
		return entry.Value.AsInt(), true
	}
	return 0, false
}

// GetFloat retrieves a float custom field
func (tcf TypedCustomFields) GetFloat(name string) (float64, bool) {
	if entry, ok := tcf.fields[name]; ok {
		return entry.Value.AsFloat(), true
	}
	return 0, false
}

// GetBool retrieves a boolean custom field
func (tcf TypedCustomFields) GetBool(name string) (bool, bool) {
	if entry, ok := tcf.fields[name]; ok {
		return entry.Value.AsBool(), true
	}
	return false, false
}

// GetTime retrieves a time custom field
func (tcf TypedCustomFields) GetTime(name string) (time.Time, bool) {
	if entry, ok := tcf.fields[name]; ok {
		return entry.Value.AsTime(), true
	}
	return time.Time{}, false
}

// GetStrings retrieves a string array custom field
func (tcf TypedCustomFields) GetStrings(name string) ([]string, bool) {
	if entry, ok := tcf.fields[name]; ok {
		return entry.Value.AsStrings(), true
	}
	return nil, false
}

// Get retrieves a custom field entry
func (tcf TypedCustomFields) Get(name string) (CustomFieldEntry, bool) {
	entry, ok := tcf.fields[name]
	return entry, ok
}

// Has checks if a custom field exists
func (tcf TypedCustomFields) Has(name string) bool {
	_, ok := tcf.fields[name]
	return ok
}

// Delete removes a custom field
func (tcf *TypedCustomFields) Delete(name string) {
	delete(tcf.fields, name)
}

// Keys returns all custom field names
func (tcf TypedCustomFields) Keys() []string {
	keys := make([]string, 0, len(tcf.fields))
	for k := range tcf.fields {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of custom fields
func (tcf TypedCustomFields) Len() int {
	return len(tcf.fields)
}

// IsEmpty returns true if no custom fields are set
func (tcf TypedCustomFields) IsEmpty() bool {
	return len(tcf.fields) == 0
}

// ToMap converts to a map[string]interface{} for map-oriented call sites.
func (tcf TypedCustomFields) ToMap() map[string]interface{} {
	result := make(map[string]interface{}, len(tcf.fields))
	for k, entry := range tcf.fields {
		switch entry.Value.Type() {
		case ValueTypeString:
			result[k] = entry.Value.AsString()
		case ValueTypeInt:
			result[k] = entry.Value.AsInt()
		case ValueTypeFloat:
			result[k] = entry.Value.AsFloat()
		case ValueTypeBool:
			result[k] = entry.Value.AsBool()
		case ValueTypeTime:
			result[k] = entry.Value.AsTime()
		case ValueTypeStrings:
			result[k] = entry.Value.AsStrings()
		default:
			result[k] = entry.Value.AsString()
		}
	}
	return result
}

func (tcf *TypedCustomFields) ensureInit() {
	if tcf.fields == nil {
		tcf.fields = make(map[string]CustomFieldEntry)
	}
}

// MarshalJSON implements json.Marshaler.
// It emits the compact object form used at integration boundaries.
func (tcf TypedCustomFields) MarshalJSON() ([]byte, error) {
	if len(tcf.fields) == 0 {
		return []byte("{}"), nil
	}

	// Marshal as a simple map for straightforward JSON consumers.
	simple := make(map[string]interface{}, len(tcf.fields))
	for k, entry := range tcf.fields {
		switch entry.Value.Type() {
		case ValueTypeString:
			simple[k] = entry.Value.stringVal
		case ValueTypeInt:
			simple[k] = entry.Value.intVal
		case ValueTypeFloat:
			simple[k] = entry.Value.floatVal
		case ValueTypeBool:
			simple[k] = entry.Value.boolVal
		case ValueTypeTime:
			simple[k] = entry.Value.timeVal.Format(time.RFC3339)
		case ValueTypeStrings:
			simple[k] = entry.Value.stringsVal
		case ValueTypeDuration:
			simple[k] = entry.Value.durationVal.String()
		default:
			simple[k] = nil
		}
	}
	return json.Marshal(simple)
}

// UnmarshalJSON implements json.Unmarshaler
// Handles both old map[string]interface{} format and new typed format
func (tcf *TypedCustomFields) UnmarshalJSON(data []byte) error {
	tcf.fields = make(map[string]CustomFieldEntry)

	// Try to unmarshal as a generic map and infer types
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for k, v := range raw {
		entry := tcf.inferType(v)
		tcf.fields[k] = entry
	}

	return nil
}

// inferType attempts to determine the type of a JSON value
func (tcf *TypedCustomFields) inferType(data json.RawMessage) CustomFieldEntry {
	// Try string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		// Check if it's a time string
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return CustomFieldEntry{Value: TimeValue(t), DataType: DataTypeDate}
		}
		return CustomFieldEntry{Value: StringValue(s), DataType: DataTypeString}
	}

	// Try bool (before number, since JSON bools are not numbers)
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		return CustomFieldEntry{Value: BoolValue(b), DataType: DataTypeBoolean}
	}

	// Try number
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		if f == float64(int64(f)) {
			return CustomFieldEntry{Value: IntValue(int64(f)), DataType: DataTypeNumber}
		}
		return CustomFieldEntry{Value: FloatValue(f), DataType: DataTypeNumber}
	}

	// Try string array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return CustomFieldEntry{Value: StringsValue(arr), DataType: DataTypeArray}
	}

	// Try interface array and convert to strings
	var iArr []interface{}
	if err := json.Unmarshal(data, &iArr); err == nil {
		strArr := make([]string, len(iArr))
		for i, v := range iArr {
			strArr[i] = fmt.Sprintf("%v", v)
		}
		return CustomFieldEntry{Value: StringsValue(strArr), DataType: DataTypeArray}
	}

	// Default to null
	return CustomFieldEntry{Value: NullValue()}
}

// =============================================================================
// Typed Context for Rules and Events
// =============================================================================

// TypedContext provides type-safe context storage for rules and event handlers
type TypedContext struct {
	// Predefined typed fields for common context values
	CaseID      string
	WorkspaceID string
	UserID      string
	EventType   string
	Timestamp   time.Time
	Source      string

	// Extension fields for additional context
	Extra Metadata
}

// NewTypedContext creates a new typed context
func NewTypedContext() TypedContext {
	return TypedContext{
		Timestamp: time.Now(),
		Extra:     NewMetadata(),
	}
}

// MarshalJSON preserves the existing snake_case wire format without tagging the domain type.
func (tc TypedContext) MarshalJSON() ([]byte, error) {
	payload := make(map[string]interface{})
	if tc.CaseID != "" {
		payload["case_id"] = tc.CaseID
	}
	if tc.WorkspaceID != "" {
		payload["workspace_id"] = tc.WorkspaceID
	}
	if tc.UserID != "" {
		payload["user_id"] = tc.UserID
	}
	if tc.EventType != "" {
		payload["event_type"] = tc.EventType
	}
	if !tc.Timestamp.IsZero() {
		payload["timestamp"] = tc.Timestamp
	}
	if tc.Source != "" {
		payload["source"] = tc.Source
	}
	if !tc.Extra.IsEmpty() {
		payload["extra"] = tc.Extra
	}
	return json.Marshal(payload)
}

// UnmarshalJSON preserves the existing snake_case wire format without tagging the domain type.
func (tc *TypedContext) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	tc.Extra = NewMetadata()
	_ = json.Unmarshal(raw["case_id"], &tc.CaseID)
	_ = json.Unmarshal(raw["workspace_id"], &tc.WorkspaceID)
	_ = json.Unmarshal(raw["user_id"], &tc.UserID)
	_ = json.Unmarshal(raw["event_type"], &tc.EventType)
	_ = json.Unmarshal(raw["timestamp"], &tc.Timestamp)
	_ = json.Unmarshal(raw["source"], &tc.Source)
	if extra, ok := raw["extra"]; ok {
		_ = json.Unmarshal(extra, &tc.Extra)
	}
	return nil
}

// WithCaseID sets the case ID
func (tc TypedContext) WithCaseID(id string) TypedContext {
	tc.CaseID = id
	return tc
}

// WithWorkspaceID sets the workspace ID
func (tc TypedContext) WithWorkspaceID(id string) TypedContext {
	tc.WorkspaceID = id
	return tc
}

// WithUserID sets the user ID
func (tc TypedContext) WithUserID(id string) TypedContext {
	tc.UserID = id
	return tc
}

// WithEventType sets the event type
func (tc TypedContext) WithEventType(eventType string) TypedContext {
	tc.EventType = eventType
	return tc
}

// WithSource sets the source
func (tc TypedContext) WithSource(source string) TypedContext {
	tc.Source = source
	return tc
}

// SetExtra sets an extra metadata field
func (tc *TypedContext) SetExtra(key string, value Value) {
	tc.Extra.Set(key, value)
}

// GetExtra retrieves an extra metadata field
func (tc TypedContext) GetExtra(key string) (Value, bool) {
	return tc.Extra.Get(key)
}
