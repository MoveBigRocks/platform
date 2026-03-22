package util

import (
	cryptoRand "crypto/rand"
	"fmt"
	mathRand "math/rand"
	"strings"
	"sync"
	"time"
)

// Base58 alphabet excludes confusing characters: 0, O, I, l
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var (
	fallbackRand     = mathRand.New(mathRand.NewSource(time.Now().UnixNano()))
	fallbackRandLock sync.Mutex
)

// GenerateBase58 generates a random Base58 string of the specified length.
// Uses crypto/rand for secure random generation.
func GenerateBase58(length int) string {
	if length <= 0 {
		return ""
	}

	const (
		alphabetLen = len(base58Alphabet)
		floorBytes  = 256 - (256 % alphabetLen)
	)

	result := make([]byte, length)
	randomBytes := make([]byte, 1)

	i := 0
	for i < length {
		_, err := cryptoRand.Read(randomBytes)
		if err != nil {
			break
		}

		if int(randomBytes[0]) >= floorBytes {
			continue
		}
		result[i] = base58Alphabet[int(randomBytes[0])%alphabetLen]
		i++
	}

	if i < length {
		fallbackRandLock.Lock()
		defer fallbackRandLock.Unlock()
		for ; i < length; i++ {
			result[i] = base58Alphabet[fallbackRand.Intn(len(base58Alphabet))]
		}
	}

	return string(result)
}

// GenerateHumanID generates a human-readable case ID in format: prefix-yymm-random
// Example: ac-2512-a3e9ef (Acme, December 2025, random)
func GenerateHumanID(prefix string, randomLength int) string {
	now := time.Now()
	yymm := fmt.Sprintf("%02d%02d", now.Year()%100, now.Month())
	random := strings.ToLower(GenerateBase58(randomLength))

	return fmt.Sprintf("%s-%s-%s", strings.ToLower(prefix), yymm, random)
}
