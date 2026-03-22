package id

import (
	"crypto/rand"
	"encoding/binary"
	"time"

	"github.com/google/uuid"
	"github.com/mr-tron/base58"
)

// NewUUIDv7 generates a new UUIDv7 with timestamp ordering
func NewUUIDv7() (uuid.UUID, error) {
	// UUIDv7 format:
	// - 48 bits: Unix timestamp in milliseconds
	// - 4 bits: version (0111 = 7)
	// - 12 bits: random
	// - 2 bits: variant (10)
	// - 62 bits: random

	var id uuid.UUID

	// Get current timestamp in milliseconds
	now := time.Now().UnixMilli()

	// Set timestamp (48 bits)
	binary.BigEndian.PutUint32(id[0:4], uint32(now>>16))
	binary.BigEndian.PutUint16(id[4:6], uint16(now))

	// Set random bits
	_, err := rand.Read(id[6:])
	if err != nil {
		return uuid.Nil, err
	}

	// Set version (0111 = 7)
	id[6] = (id[6] & 0x0f) | 0x70

	// Set variant (10)
	id[8] = (id[8] & 0x3f) | 0x80

	return id, nil
}

// ToBase58 converts a UUID to Base58 encoding (for public IDs)
func ToBase58(id uuid.UUID) string {
	return base58.Encode(id[:])
}

// NewPublicID generates a new Base58-encoded public ID
func NewPublicID() string {
	id, err := NewUUIDv7()
	if err != nil {
		// Fall back to standard UUID if crypto fails
		return ToBase58(uuid.New())
	}
	return ToBase58(id)
}

// New generates a new UUIDv7 string (standard UUID format with timestamp ordering)
// This is the preferred ID generation method for all entities.
func New() string {
	id, err := NewUUIDv7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}
