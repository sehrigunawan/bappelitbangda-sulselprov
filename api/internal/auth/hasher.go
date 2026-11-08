// Package auth holds the account and credential logic shared by the HTTP server and the operator CLI.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Current argon2id parameters (OWASP minimum, docs/modules/auth.md "Passwords").
const (
	argonMemory  = 19456 // KiB
	argonTime    = 2
	argonThreads = 1
	argonSaltLen = 16
	argonKeyLen  = 32
	// maxConcurrentHashes bounds memory use: at most this many argon2id computations run at once.
	maxConcurrentHashes = 2
)

// dummyHash is a hash of a random password made with the current parameters. VerifyDummy checks
// against it so that an unknown username costs the same time as a known one.
const dummyHash = "$argon2id$v=19$m=19456,t=2,p=1$Qx8zu6/R1O29mHHWGSXkrw$Gjw9l3cPIkrDtBANFAf2aGvUt6KCa+T6QY+zd0YVFro"

var errMalformedHash = errors.New("auth: malformed password hash")

// Hasher computes and verifies argon2id password hashes. It bounds concurrent hashing so that
// login traffic cannot exhaust memory.
type Hasher struct {
	slots  chan struct{}
	onHash func() // test hook, runs while a slot is held
}

// NewHasher returns a Hasher with the concurrency limit used by the server.
func NewHasher() *Hasher {
	return &Hasher{slots: make(chan struct{}, maxConcurrentHashes)}
}

// acquire takes a hashing slot or fails when ctx ends first. The returned func releases the slot.
func (h *Hasher) acquire(ctx context.Context) (release func(), err error) {
	select {
	case h.slots <- struct{}{}:
		return func() { <-h.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Hash returns the PHC-encoded argon2id hash of password with the current parameters.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	release, err := h.acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("auth: wait for hash slot: %w", err)
	}
	defer release()
	if h.onHash != nil {
		h.onHash()
	}

	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return encodeHash(argonMemory, argonTime, argonThreads, salt, key), nil
}

// Verify reports whether password matches the encoded hash. rehash is true when the hash was made
// with parameters other than the current ones and should be replaced.
func (h *Hasher) Verify(ctx context.Context, encoded, password string) (ok, rehash bool, err error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, false, err
	}
	release, err := h.acquire(ctx)
	if err != nil {
		return false, false, fmt.Errorf("auth: wait for hash slot: %w", err)
	}
	defer release()
	if h.onHash != nil {
		h.onHash()
	}

	got := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, uint32(len(want))) //nolint:gosec // len(want) comes from a stored hash, far below 2^32
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	return true, p != currentParams, nil
}

// VerifyDummy spends the same time as Verify on a fixed hash. It is used when the user does not
// exist so that response timing does not reveal account existence.
func (h *Hasher) VerifyDummy(ctx context.Context, password string) error {
	_, _, err := h.Verify(ctx, dummyHash, password)
	return err
}

// params are the argon2id cost parameters stored in a PHC string.
type params struct {
	memory  uint32
	time    uint32
	threads uint8
}

var currentParams = params{memory: argonMemory, time: argonTime, threads: argonThreads}

func encodeHash(memory, time uint32, threads uint8, salt, key []byte) string {
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, time, threads, enc.EncodeToString(salt), enc.EncodeToString(key))
}

// decodeHash parses a PHC string "$argon2id$v=19$m=..,t=..,p=..$salt$key". The values come from our
// own database, but argon2.IDKey panics on zero time or threads, so they are checked anyway.
func decodeHash(encoded string) (p params, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return params{}, nil, nil, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return params{}, nil, nil, errMalformedHash
	}
	var threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &threads); err != nil {
		return params{}, nil, nil, errMalformedHash
	}
	if p.memory < 8 || p.time < 1 || threads < 1 || threads > 255 {
		return params{}, nil, nil, errMalformedHash
	}
	p.threads = uint8(threads)
	enc := base64.RawStdEncoding
	if salt, err = enc.DecodeString(parts[4]); err != nil || len(salt) == 0 {
		return params{}, nil, nil, errMalformedHash
	}
	if key, err = enc.DecodeString(parts[5]); err != nil || len(key) < 16 {
		return params{}, nil, nil, errMalformedHash
	}
	return p, salt, key, nil
}
