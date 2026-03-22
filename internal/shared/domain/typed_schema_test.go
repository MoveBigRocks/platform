package shareddomain

import (
	"encoding/json"
	"testing"
)

func TestTypedSchemaHelpers(t *testing.T) {
	t.Parallel()

	empty := NewTypedSchema()
	if !empty.IsEmpty() {
		t.Fatalf("expected new typed schema to be empty, got %#v", empty)
	}

	schema := TypedSchemaFromMap(map[string]interface{}{
		"type":        "object",
		"title":       "Contact Form",
		"description": "Collect contact details",
		"required":    []interface{}{"email", "name"},
		"properties": map[string]interface{}{
			"email": map[string]interface{}{
				"type":        "string",
				"title":       "Email",
				"description": "Primary email",
				"format":      "email",
				"minLength":   float64(5),
				"maxLength":   float64(255),
				"pattern":     ".+@.+",
				"default":     "ops@example.com",
				"enum":        []interface{}{"ops@example.com", "sales@example.com"},
				"readOnly":    true,
			},
			"priority": map[string]interface{}{
				"type":        "number",
				"title":       "Priority",
				"description": "Priority level",
				"minimum":     float64(1),
				"maximum":     float64(5),
			},
		},
	})

	if schema.Type() != "object" || schema.Title() != "Contact Form" || schema.Description() != "Collect contact details" {
		t.Fatalf("unexpected schema metadata: %#v", schema)
	}
	if len(schema.Required()) != 2 || !schema.IsRequired("email") || schema.IsRequired("priority") {
		t.Fatalf("unexpected required fields: %#v", schema.Required())
	}
	if len(schema.Properties()) != 2 || !schema.HasProperty("email") || schema.HasProperty("missing") {
		t.Fatalf("unexpected properties: %#v", schema.Properties())
	}

	field, ok := schema.GetField("email")
	if !ok {
		t.Fatal("expected email field to exist")
	}
	if field.Type() != "string" || field.Title() != "Email" || field.Description() != "Primary email" {
		t.Fatalf("unexpected field metadata: %#v", field)
	}
	if field.Default() != "ops@example.com" || field.Format() != "email" || field.MinLength() != 5 || field.MaxLength() != 255 {
		t.Fatalf("unexpected field defaults/lengths: %#v", field)
	}
	if field.Pattern() != ".+@.+" || len(field.Enum()) != 2 || len(field.EnumStrings()) != 2 || !field.IsReadOnly() {
		t.Fatalf("unexpected field enum/pattern/readonly state: %#v", field)
	}
	if len(field.ToMap()) == 0 {
		t.Fatalf("expected field map output: %#v", field)
	}

	numberField, ok := schema.GetField("priority")
	if !ok || numberField.Minimum() != 1 || numberField.Maximum() != 5 {
		t.Fatalf("unexpected numeric field data: %#v", numberField)
	}
	if _, ok := schema.GetField("missing"); ok {
		t.Fatal("expected missing field lookup to fail")
	}

	schema.Set("draft", true)
	schema.Set("slug", "contact-form")
	if raw, ok := schema.Get("draft"); !ok || raw != true {
		t.Fatalf("expected raw get to work, got %v ok=%v", raw, ok)
	}
	if schema.GetString("slug") != "contact-form" || !schema.GetBool("draft") {
		t.Fatalf("unexpected typed getters: %#v", schema)
	}
	if len(schema.ToMap()) == 0 {
		t.Fatalf("expected schema map output: %#v", schema)
	}
}

func TestTypedSchemaCloneAndJSONRoundTrip(t *testing.T) {
	t.Parallel()

	schema := NewTypedSchema()
	schema.Set("title", "Form Title")
	schema.Set("readOnly", false)
	schema.Set("properties", map[string]interface{}{
		"status": map[string]interface{}{
			"type":  "string",
			"title": "Status",
		},
	})

	clone := schema.Clone()
	clone.Set("title", "Changed")
	if schema.GetString("title") != "Form Title" || clone.GetString("title") != "Changed" {
		t.Fatalf("expected clone to be independent, original=%#v clone=%#v", schema, clone)
	}

	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal typed schema: %v", err)
	}

	var decoded TypedSchema
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal typed schema: %v", err)
	}
	if decoded.GetString("title") != "Form Title" || decoded.IsEmpty() {
		t.Fatalf("expected typed schema to round-trip, got %#v", decoded)
	}
}

func TestFieldSchemaFromNilMap(t *testing.T) {
	t.Parallel()

	field := FieldSchemaFromMap(nil)
	if field.Type() != "" || field.Title() != "" || field.Description() != "" {
		t.Fatalf("expected zero field schema from nil map, got %#v", field)
	}
	if field.Default() != nil || field.Format() != "" || field.MinLength() != 0 || field.MaxLength() != 0 || field.Minimum() != 0 || field.Maximum() != 0 {
		t.Fatalf("expected zero field defaults from nil map, got %#v", field)
	}
	if field.Pattern() != "" || field.Enum() != nil || field.EnumStrings() != nil || field.IsReadOnly() {
		t.Fatalf("expected zero field validation helpers from nil map, got %#v", field)
	}
	if len(field.ToMap()) != 0 {
		t.Fatalf("expected empty map view for zero field schema, got %#v", field.ToMap())
	}
}
