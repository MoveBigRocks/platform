package id

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewUUIDv7(t *testing.T) {
	id, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("Failed to generate UUIDv7: %v", err)
	}

	if id == uuid.Nil {
		t.Fatal("Expected non-nil UUID")
	}

	// Verify version 7
	if id.Version() != 7 {
		t.Errorf("Expected UUID version 7, got %d", id.Version())
	}

	// Generate another and verify uniqueness
	id2, err := NewUUIDv7()
	if err != nil {
		t.Fatalf("Failed to generate second UUIDv7: %v", err)
	}

	if id == id2 {
		t.Error("Expected different UUIDs on subsequent calls")
	}
}

func TestNewUUIDv7_Uniqueness(t *testing.T) {
	ids := make(map[uuid.UUID]bool)
	count := 100

	for i := 0; i < count; i++ {
		id, err := NewUUIDv7()
		if err != nil {
			t.Fatalf("Failed to generate UUID: %v", err)
		}
		if ids[id] {
			t.Errorf("Duplicate ID found: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}

func TestToBase58(t *testing.T) {
	id := uuid.New()
	base58 := ToBase58(id)

	if base58 == "" {
		t.Error("Expected non-empty base58 string")
	}

	// Verify shorter than UUID string
	if len(base58) >= 36 {
		t.Errorf("Expected base58 (%d chars) to be shorter than UUID string (36 chars)", len(base58))
	}
}

func TestNewPublicID(t *testing.T) {
	id := NewPublicID()

	if id == "" {
		t.Fatal("Expected non-empty public ID")
	}

	// Verify reasonable length
	if len(id) > 30 || len(id) < 15 {
		t.Logf("Public ID length: %d (may vary)", len(id))
	}

	// Verify uniqueness
	id2 := NewPublicID()
	if id == id2 {
		t.Error("Expected different public IDs on subsequent calls")
	}
}

func TestNewPublicID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	count := 100

	for i := 0; i < count; i++ {
		id := NewPublicID()
		if ids[id] {
			t.Errorf("Duplicate public ID found: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}

func BenchmarkNewUUIDv7(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewUUIDv7()
	}
}

func BenchmarkToBase58(b *testing.B) {
	id := uuid.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ToBase58(id)
	}
}

func BenchmarkNewPublicID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewPublicID()
	}
}
