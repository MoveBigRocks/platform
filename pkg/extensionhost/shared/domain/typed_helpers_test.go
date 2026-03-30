package shareddomain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestValueConversionsAndPredicates(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)
	values := []Value{
		StringValue("42"),
		IntValue(42),
		FloatValue(42.5),
		BoolValue(true),
		TimeValue(now),
		StringsValue([]string{"a", "b"}),
		DurationValue(2 * time.Minute),
		NullValue(),
	}

	if values[0].Type() != ValueTypeString || values[7].Type() != ValueTypeNull {
		t.Fatalf("unexpected value types: %#v", values)
	}
	if values[0].IsZero() || !values[7].IsZero() {
		t.Fatalf("unexpected zero-value semantics: %#v", values)
	}
	if values[1].AsString() != "42" || values[0].AsInt() != 42 || values[1].AsFloat() != 42 || !values[3].AsBool() {
		t.Fatalf("unexpected primitive conversions: %#v", values)
	}
	if !values[4].AsTime().Equal(now) || len(values[5].AsStrings()) != 2 || values[6].AsDuration() != 2*time.Minute {
		t.Fatalf("unexpected complex conversions: %#v", values)
	}

	if _, err := StringValue("abc").TryAsInt(); err == nil {
		t.Fatal("expected invalid int conversion to fail")
	}
	if _, err := StringValue("abc").TryAsFloat(); err == nil {
		t.Fatal("expected invalid float conversion to fail")
	}
	if _, err := StringValue("not-bool").TryAsBool(); err == nil {
		t.Fatal("expected invalid bool conversion to fail")
	}
	if _, err := StringValue("not-time").TryAsTime(); err == nil {
		t.Fatal("expected invalid time conversion to fail")
	}
	if _, err := StringValue("not-duration").TryAsDuration(); err == nil {
		t.Fatal("expected invalid duration conversion to fail")
	}

	if !values[0].IsString() || !values[1].IsInt() || !values[2].IsFloat() || !values[3].IsBool() || !values[4].IsTime() || !values[5].IsStrings() || !values[6].IsDuration() || !values[7].IsNull() || !values[1].IsNumeric() {
		t.Fatalf("unexpected value predicates: %#v", values)
	}
	if !StringValue("42").Equals(IntValue(42)) || !StringsValue([]string{"a", "b"}).Equals(StringsValue([]string{"a", "b"})) {
		t.Fatal("expected equality helpers to succeed")
	}
	if BoolValue(true).ToInterface() != true || NullValue().ToInterface() != nil {
		t.Fatal("expected interface conversion to preserve primitive values")
	}
}

func TestValueAndMetadataJSONHelpers(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	var fromPrimitive Value
	if err := json.Unmarshal([]byte(`123`), &fromPrimitive); err != nil {
		t.Fatalf("unmarshal primitive value: %v", err)
	}
	if fromPrimitive.Type() != ValueTypeInt || fromPrimitive.AsInt() != 123 {
		t.Fatalf("unexpected primitive value: %#v", fromPrimitive)
	}

	value := TimeValue(now)
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal typed value: %v", err)
	}
	var decoded Value
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal typed value: %v", err)
	}
	if !decoded.AsTime().Equal(now) {
		t.Fatalf("expected time value to round-trip, got %#v", decoded)
	}

	meta := MetadataFromMap(map[string]interface{}{
		"name":    "Ada",
		"enabled": true,
		"count":   int64(3),
		"tags":    []interface{}{"one", "two"},
	})
	meta.SetString("role", "admin")
	meta.SetInt("score", 7)
	meta.SetBool("active", true)
	if meta.GetString("role") != "admin" || meta.GetInt("score") != 7 || !meta.GetBool("active") || !meta.Has("name") {
		t.Fatalf("unexpected metadata helpers: %#v", meta)
	}
	if _, ok := meta.Get("enabled"); !ok {
		t.Fatalf("expected metadata Get to return value")
	}
	if len(meta.Keys()) == 0 || meta.Len() == 0 || meta.IsEmpty() {
		t.Fatalf("expected metadata to contain entries: %#v", meta)
	}

	clone := meta.Clone()
	clone.Delete("role")
	if !meta.Has("role") || clone.Has("role") {
		t.Fatalf("expected clone/delete behavior to isolate copies")
	}

	merged := NewMetadata()
	merged.Merge(meta)
	merged.Merge(MetadataFromMap(map[string]interface{}{"name": "Grace"}))
	if merged.GetString("name") != "Grace" {
		t.Fatalf("expected merge precedence to apply, got %#v", merged)
	}
	if len(merged.ToInterfaceMap()) == 0 {
		t.Fatalf("expected interface map output")
	}

	encoded, err := json.Marshal(merged)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	var roundTrip Metadata
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if roundTrip.GetString("name") != "Grace" {
		t.Fatalf("expected metadata JSON round-trip, got %#v", roundTrip)
	}
}

func TestChangeSetAndTypedCustomFieldsHelpers(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	changeSet := NewChangeSet()
	changeSet.RecordString("status", "new", "open")
	changeSet.RecordInt("count", 1, 2)
	changeSet.RecordBool("active", false, true)
	changeSet.RecordTime("updated_at", now, now.Add(time.Hour))
	if !changeSet.HasChanges() || len(changeSet.Changes()) != 4 {
		t.Fatalf("expected recorded changes, got %#v", changeSet)
	}
	if change, ok := changeSet.GetChange("count"); !ok || change.NewValue.AsInt() != 2 {
		t.Fatalf("expected typed change lookup, got %#v", change)
	}
	data, err := json.Marshal(changeSet)
	if err != nil {
		t.Fatalf("marshal change set: %v", err)
	}
	var decodedChangeSet ChangeSet
	if err := json.Unmarshal(data, &decodedChangeSet); err != nil {
		t.Fatalf("unmarshal change set: %v", err)
	}
	if !decodedChangeSet.HasChanges() {
		t.Fatalf("expected change set JSON round-trip, got %#v", decodedChangeSet)
	}

	fields := NewTypedCustomFields()
	fields.SetString("name", "Ada")
	fields.SetInt("age", 42)
	fields.SetFloat("score", 9.5)
	fields.SetBool("enabled", true)
	fields.SetTime("seen_at", now)
	fields.SetStrings("tags", []string{"ops", "triage"})
	fields.SetAny("nickname", "ace")
	fields.SetAny("attempts", int32(3))
	fields.SetAny("ratio", float32(1.5))
	fields.SetAny("disabled", nil)
	if name, ok := fields.GetString("name"); !ok || name != "Ada" {
		t.Fatalf("expected typed string field, got %#v", fields)
	}
	if age, ok := fields.GetInt("age"); !ok || age != 42 {
		t.Fatalf("expected typed int field, got %#v", fields)
	}
	if score, ok := fields.GetFloat("score"); !ok || score != 9.5 {
		t.Fatalf("expected typed float field, got %#v", fields)
	}
	if enabled, ok := fields.GetBool("enabled"); !ok || !enabled {
		t.Fatalf("expected typed bool field, got %#v", fields)
	}
	if seenAt, ok := fields.GetTime("seen_at"); !ok || !seenAt.Equal(now) {
		t.Fatalf("expected typed time field, got %#v", fields)
	}
	if tags, ok := fields.GetStrings("tags"); !ok || len(tags) != 2 {
		t.Fatalf("expected typed strings field, got %#v", fields)
	}
	if !fields.Has("nickname") || fields.Has("disabled") {
		t.Fatalf("expected SetAny/delete behavior, got %#v", fields)
	}

	fields.Delete("nickname")
	if fields.Has("nickname") || fields.Len() == 0 || fields.IsEmpty() {
		t.Fatalf("expected delete/len helpers to behave, got %#v", fields)
	}
	if len(fields.Keys()) == 0 || len(fields.ToMap()) == 0 {
		t.Fatalf("expected keys and map conversion, got %#v", fields)
	}

	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal custom fields: %v", err)
	}
	var roundTrip TypedCustomFields
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("unmarshal custom fields: %v", err)
	}
	if restored, ok := roundTrip.GetString("name"); !ok || restored != "Ada" {
		t.Fatalf("expected custom fields JSON round-trip, got %#v", roundTrip)
	}
	var inferred TypedCustomFields
	if err := json.Unmarshal([]byte(`{"mixed":["a",1]}`), &inferred); err != nil {
		t.Fatalf("unmarshal inferred custom fields: %v", err)
	}
	if tags, ok := inferred.GetStrings("mixed"); !ok || len(tags) != 2 || tags[1] != "1" {
		t.Fatalf("expected mixed array inference to stringify entries, got %#v", inferred)
	}
}

func TestValueConversionEdgeCasesAndInterfaceConversion(t *testing.T) {
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	if got := FloatValue(42.5).AsString(); got != "42.5" {
		t.Fatalf("expected float string conversion, got %q", got)
	}
	if got := BoolValue(true).AsString(); got != "true" {
		t.Fatalf("expected bool string conversion, got %q", got)
	}
	if got := TimeValue(now).AsString(); got != now.Format(time.RFC3339) {
		t.Fatalf("expected time string conversion, got %q", got)
	}
	if got := DurationValue(90 * time.Second).AsString(); got != "1m30s" {
		t.Fatalf("expected duration string conversion, got %q", got)
	}

	if got := FloatValue(8.9).AsInt(); got != 8 {
		t.Fatalf("expected float to int conversion, got %d", got)
	}
	if got := StringValue("9").AsInt(); got != 9 {
		t.Fatalf("expected string to int conversion, got %d", got)
	}
	if got := BoolValue(true).AsInt(); got != 1 {
		t.Fatalf("expected bool true to int conversion, got %d", got)
	}

	if got := IntValue(7).AsFloat(); got != 7 {
		t.Fatalf("expected int to float conversion, got %f", got)
	}
	if got := StringValue("7.5").AsFloat(); got != 7.5 {
		t.Fatalf("expected string to float conversion, got %f", got)
	}

	if !StringValue("true").AsBool() || !IntValue(1).AsBool() || !FloatValue(2.5).AsBool() {
		t.Fatal("expected bool coercion to treat non-zero values as true")
	}

	if !StringValue(now.Format(time.RFC3339)).AsTime().Equal(now) {
		t.Fatal("expected RFC3339 string to convert to time")
	}
	if !IntValue(10).AsTime().Equal(time.Unix(10, 0)) {
		t.Fatal("expected unix seconds int to convert to time")
	}

	if got := StringValue("vip").AsStrings(); len(got) != 1 || got[0] != "vip" {
		t.Fatalf("expected singleton strings conversion, got %#v", got)
	}
	if got := StringValue("").AsStrings(); got != nil {
		t.Fatalf("expected empty string to map to nil slice, got %#v", got)
	}

	if got := IntValue(120).AsDuration(); got != 120*time.Second {
		t.Fatalf("expected int to duration conversion, got %s", got)
	}
	if got := StringValue("2m").AsDuration(); got != 2*time.Minute {
		t.Fatalf("expected string to duration conversion, got %s", got)
	}

	if got, err := BoolValue(true).TryAsInt(); err != nil || got != 1 {
		t.Fatalf("expected bool TryAsInt to succeed, got %d, %v", got, err)
	}
	if got, err := FloatValue(8.9).TryAsInt(); err != nil || got != 8 {
		t.Fatalf("expected float TryAsInt to succeed, got %d, %v", got, err)
	}
	if _, err := NullValue().TryAsInt(); err == nil {
		t.Fatal("expected null TryAsInt to fail")
	}
	if _, err := StringsValue([]string{"a"}).TryAsInt(); err == nil {
		t.Fatal("expected strings TryAsInt to fail")
	}

	if got, err := IntValue(7).TryAsFloat(); err != nil || got != 7 {
		t.Fatalf("expected int TryAsFloat to succeed, got %f, %v", got, err)
	}
	if _, err := BoolValue(true).TryAsFloat(); err == nil {
		t.Fatal("expected bool TryAsFloat to fail")
	}
	if _, err := NullValue().TryAsFloat(); err == nil {
		t.Fatal("expected null TryAsFloat to fail")
	}

	if got, err := IntValue(1).TryAsBool(); err != nil || !got {
		t.Fatalf("expected int TryAsBool to succeed, got %v, %v", got, err)
	}
	if got, err := FloatValue(2.0).TryAsBool(); err != nil || !got {
		t.Fatalf("expected float TryAsBool to succeed, got %v, %v", got, err)
	}
	if _, err := NullValue().TryAsBool(); err == nil {
		t.Fatal("expected null TryAsBool to fail")
	}
	if _, err := StringsValue([]string{"a"}).TryAsBool(); err == nil {
		t.Fatal("expected strings TryAsBool to fail")
	}

	if got, err := IntValue(10).TryAsTime(); err != nil || !got.Equal(time.Unix(10, 0)) {
		t.Fatalf("expected int TryAsTime to succeed, got %v, %v", got, err)
	}
	if _, err := NullValue().TryAsTime(); err == nil {
		t.Fatal("expected null TryAsTime to fail")
	}
	if _, err := BoolValue(true).TryAsTime(); err == nil {
		t.Fatal("expected bool TryAsTime to fail")
	}

	if got, err := IntValue(5).TryAsDuration(); err != nil || got != 5*time.Second {
		t.Fatalf("expected int TryAsDuration to succeed, got %v, %v", got, err)
	}
	if _, err := NullValue().TryAsDuration(); err == nil {
		t.Fatal("expected null TryAsDuration to fail")
	}
	if _, err := BoolValue(true).TryAsDuration(); err == nil {
		t.Fatal("expected bool TryAsDuration to fail")
	}

	if !FloatValue(1.5).Equals(FloatValue(1.5)) {
		t.Fatal("expected float equality")
	}
	if !TimeValue(now).Equals(TimeValue(now)) {
		t.Fatal("expected time equality")
	}
	if !DurationValue(time.Minute).Equals(DurationValue(time.Minute)) {
		t.Fatal("expected duration equality")
	}
	if StringsValue([]string{"a", "b"}).Equals(StringsValue([]string{"a"})) {
		t.Fatal("expected differing string slices not to be equal")
	}
	if StringsValue([]string{"a", "b"}).Equals(StringsValue([]string{"a", "c"})) {
		t.Fatal("expected differing string slice contents not to be equal")
	}
	if StringValue("text").Equals(BoolValue(true)) {
		t.Fatal("expected differing mixed-type string coercion not to match")
	}

	if got := ValueFromInterface(int32(4)); got.AsInt() != 4 {
		t.Fatalf("expected int32 conversion, got %#v", got)
	}
	if got := ValueFromInterface(float32(1.25)); got.AsFloat() != 1.25 {
		t.Fatalf("expected float32 conversion, got %#v", got)
	}
	if got := ValueFromInterface([]interface{}{"a", 1, true}); len(got.AsStrings()) != 1 || got.AsStrings()[0] != "a" {
		t.Fatalf("expected []interface{} conversion to keep string entries, got %#v", got)
	}
	if got := ValueFromInterface(now); !got.AsTime().Equal(now) {
		t.Fatalf("expected time conversion, got %#v", got)
	}
	if got := ValueFromInterface(3 * time.Minute); got.AsDuration() != 3*time.Minute {
		t.Fatalf("expected duration conversion, got %#v", got)
	}
	if got := ValueFromInterface(struct{ Name string }{Name: "Ada"}); got.AsString() != "{Ada}" {
		t.Fatalf("expected fallback string conversion, got %#v", got)
	}
}

func TestValuePrimitiveUnmarshalAndTypedCustomFieldGet(t *testing.T) {
	var boolValue Value
	if err := json.Unmarshal([]byte(`true`), &boolValue); err != nil {
		t.Fatalf("unmarshal bool primitive: %v", err)
	}
	if !boolValue.AsBool() {
		t.Fatalf("expected bool primitive to unmarshal, got %#v", boolValue)
	}

	var listValue Value
	if err := json.Unmarshal([]byte(`["a","b"]`), &listValue); err != nil {
		t.Fatalf("unmarshal string array primitive: %v", err)
	}
	if got := listValue.AsStrings(); len(got) != 2 || got[1] != "b" {
		t.Fatalf("expected array primitive to unmarshal, got %#v", listValue)
	}

	var fallbackNull Value
	if err := json.Unmarshal([]byte(`{}`), &fallbackNull); err != nil {
		t.Fatalf("unmarshal fallback null primitive: %v", err)
	}
	if !fallbackNull.IsNull() {
		t.Fatalf("expected fallback object to unmarshal to null value, got %#v", fallbackNull)
	}

	var fields TypedCustomFields
	fields.SetAny("struct_value", struct{ Code int }{Code: 7})
	entry, ok := fields.Get("struct_value")
	if !ok {
		t.Fatalf("expected raw custom field entry")
	}
	if entry.Value.AsString() != "{7}" {
		t.Fatalf("expected unknown SetAny value to stringify, got %#v", entry)
	}
	if entry.DataType != DataTypeString {
		t.Fatalf("expected unknown SetAny value to use string data type, got %#v", entry)
	}
}
