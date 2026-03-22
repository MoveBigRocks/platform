package analyticsdomain

import (
	"encoding/binary"

	"github.com/dchest/siphash"
)

// GenerateVisitorID creates a cookieless visitor identifier using SipHash.
// The salt rotates daily, so the same visitor gets a different ID each day.
// IP and User-Agent are used in-memory only, never persisted.
// Returns int64 (reinterpreted from uint64) in a SQLite-friendly form.
func GenerateVisitorID(salt []byte, userAgent, remoteIP, domain string) int64 {
	if len(salt) < 16 {
		return 0
	}
	data := []byte(userAgent + remoteIP + domain)
	h := siphash.Hash(
		binary.LittleEndian.Uint64(salt[:8]),
		binary.LittleEndian.Uint64(salt[8:16]),
		data,
	)
	return int64(h)
}
