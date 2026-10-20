package domain

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
)

var idPattern = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)

// NewID returns a time-prefixed 24-character lowercase hexadecimal identifier.
// The shape intentionally matches the former BSON ObjectID contract used by the API.
func NewID() (string, error) {
	var raw [12]byte
	binary.BigEndian.PutUint32(raw[:4], uint32(time.Now().UTC().Unix()))
	if _, err := rand.Read(raw[4:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

// MustID is intended for tests and static fixtures where randomness failure is unrecoverable.
func MustID() string {
	id, err := NewID()
	if err != nil {
		panic(err)
	}
	return id
}

// ParseID validates an externally supplied 24-character lowercase hexadecimal identifier.
func ParseID(raw string) (string, error) {
	if !idPattern.MatchString(raw) {
		return "", errors.New("invalid id")
	}
	return strings.ToLower(raw), nil
}
