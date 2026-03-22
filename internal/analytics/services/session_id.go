package analyticsservices

import (
	"crypto/rand"
	"encoding/binary"
)

func generateSessionID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return int64(binary.BigEndian.Uint64(b[:]))
}
