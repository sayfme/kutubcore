// Package id generates UUIDv4 identifiers (Blueprint §6).
package id

import (
	"crypto/rand"
	"fmt"
)

// New returns a random RFC 4122 v4 UUID string.
func New() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func mustNew() string {
	id, err := New()
	if err != nil {
		panic(err)
	}
	return id
}

// Must returns a new UUID or panics (use in seed/test code paths).
func Must() string { return mustNew() }
