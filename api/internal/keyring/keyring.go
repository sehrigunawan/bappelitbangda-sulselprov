// Package keyring implements envelope encryption: data keys (DEKs) are stored wrapped by a key
// encryption key (KEK) that lives in a file outside the database.
//
// Field ciphertext layout: 0x01 || uint16 big-endian data key id || 12-byte random nonce ||
// AES-256-GCM ciphertext and tag. The caller's AAD binds a value to its column and row.
// A wrapped data key is nonce(12) || AES-256-GCM(KEK, nonce, DEK, aad = "data_keys:" + kek_id).
package keyring

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
)

const (
	keySize       = 32 // AES-256 for both KEKs and DEKs
	nonceSize     = 12 // GCM standard nonce
	tagSize       = 16 // GCM tag
	versionByte   = 0x01
	headerSize    = 1 + 2 // version, data key id
	minCiphertext = headerSize + nonceSize + tagSize
)

// Keyring holds the unwrapped data keys and seals and opens values with them.
type Keyring struct {
	pool *pgxpool.Pool
	// keks maps a KEK id to its key. It is read-only after Load.
	keks map[string][]byte
	// activeKEK wraps new and rewrapped data keys.
	activeKEK string
	// activeDEK is the id of the data key used by Seal. It is fixed at Load.
	activeDEK int16

	mu   sync.RWMutex // guards deks
	deks map[int16][]byte
}

// Load reads the KEK file, creates the first data key if none exists, and unwraps every data key.
func Load(ctx context.Context, pool *pgxpool.Pool, kekFile, activeID string) (*Keyring, error) {
	keks, err := readKEKFile(kekFile)
	if err != nil {
		return nil, err
	}
	if _, ok := keks[activeID]; !ok {
		return nil, fmt.Errorf("keyring: KEK_ACTIVE_ID %q is not in the KEK file", activeID)
	}
	k := &Keyring{pool: pool, keks: keks, activeKEK: activeID, deks: map[int16][]byte{}}

	q := dbgen.New(pool)
	rows, err := q.ListDataKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("keyring: list data keys: %w", err)
	}
	if len(rows) == 0 {
		if err := k.createFirstDEK(ctx); err != nil {
			return nil, err
		}
		if rows, err = q.ListDataKeys(ctx); err != nil {
			return nil, fmt.Errorf("keyring: list data keys: %w", err)
		}
	}
	deks, err := k.unwrapAll(rows)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("keyring: no data key after creating the first one")
	}
	k.deks = deks
	k.activeDEK = rows[len(rows)-1].ID // rows are ordered by id; the active key is the highest
	return k, nil
}

// createFirstDEK inserts a data key when the table is empty. The advisory lock and the second count
// keep the server and the CLI from both creating the first key.
func (k *Keyring) createFirstDEK(ctx context.Context) error {
	err := pgx.BeginTxFunc(ctx, k.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := q.LockDataKeys(ctx); err != nil {
			return err
		}
		n, err := q.CountDataKeys(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		_, err = k.insertNewDEK(ctx, q)
		return err
	})
	if err != nil {
		return fmt.Errorf("keyring: create first data key: %w", err)
	}
	return nil
}

// insertNewDEK generates a data key, wraps it with the active KEK and stores it.
func (k *Keyring) insertNewDEK(ctx context.Context, q *dbgen.Queries) (int16, error) {
	dek := make([]byte, keySize)
	if _, err := rand.Read(dek); err != nil {
		return 0, fmt.Errorf("read random data key: %w", err)
	}
	wrapped, err := wrapKey(k.keks[k.activeKEK], k.activeKEK, dek)
	if err != nil {
		return 0, err
	}
	return q.InsertDataKey(ctx, dbgen.InsertDataKeyParams{KekID: k.activeKEK, WrappedKey: wrapped})
}

// Seal encrypts plaintext with the active data key, binding it to aad. The output is versioned and
// carries the data key id.
func (k *Keyring) Seal(plaintext, aad []byte) ([]byte, error) {
	k.mu.RLock()
	dek := k.deks[k.activeDEK]
	k.mu.RUnlock()
	gcm, err := newGCM(dek)
	if err != nil {
		return nil, fmt.Errorf("keyring: seal: %w", err)
	}

	out := make([]byte, headerSize+nonceSize, headerSize+nonceSize+len(plaintext)+tagSize)
	out[0] = versionByte
	binary.BigEndian.PutUint16(out[1:headerSize], uint16(k.activeDEK)) //nolint:gosec // data key ids come from an identity column and are positive
	nonce := out[headerSize : headerSize+nonceSize]
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("keyring: read nonce: %w", err)
	}
	return gcm.Seal(out, nonce, plaintext, aad), nil
}

// Open decrypts ciphertext produced by Seal. It fails when aad does not match or the data is altered.
// An unknown data key id triggers one reload of the data keys, so a rotation by another process
// does not need a restart.
func (k *Keyring) Open(ciphertext, aad []byte) ([]byte, error) {
	if len(ciphertext) < minCiphertext || ciphertext[0] != versionByte {
		return nil, errors.New("keyring: not a valid ciphertext")
	}
	id := int16(binary.BigEndian.Uint16(ciphertext[1:headerSize])) //nolint:gosec // an id above 32767 cannot match any data key and fails as unknown
	dek, err := k.dek(id)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(dek)
	if err != nil {
		return nil, fmt.Errorf("keyring: open: %w", err)
	}
	nonce := ciphertext[headerSize : headerSize+nonceSize]
	plain, err := gcm.Open(nil, nonce, ciphertext[headerSize+nonceSize:], aad)
	if err != nil {
		return nil, errors.New("keyring: ciphertext does not authenticate")
	}
	return plain, nil
}

// dek returns the data key with the given id, reloading the table once when it is unknown.
func (k *Keyring) dek(id int16) ([]byte, error) {
	k.mu.RLock()
	dek, ok := k.deks[id]
	k.mu.RUnlock()
	if ok {
		return dek, nil
	}

	rows, err := dbgen.New(k.pool).ListDataKeys(context.Background())
	if err != nil {
		return nil, fmt.Errorf("keyring: reload data keys: %w", err)
	}
	deks, err := k.unwrapAll(rows)
	if err != nil {
		return nil, err
	}
	k.mu.Lock()
	k.deks = deks
	k.mu.Unlock()
	if dek, ok = deks[id]; ok {
		return dek, nil
	}
	return nil, fmt.Errorf("keyring: unknown data key %d", id)
}

// RotateDEK creates a new data key wrapped with the active KEK and returns its id. The active data
// key of this instance does not change.
func (k *Keyring) RotateDEK(ctx context.Context) (int16, error) {
	id, err := k.insertNewDEK(ctx, dbgen.New(k.pool))
	if err != nil {
		return 0, fmt.Errorf("keyring: rotate data key: %w", err)
	}
	return id, nil
}

// Rewrap wraps every data key with the active KEK and returns how many were rewrapped.
func (k *Keyring) Rewrap(ctx context.Context) (int, error) {
	var n int
	err := pgx.BeginTxFunc(ctx, k.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		rows, err := q.ListDataKeys(ctx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			dek, err := k.unwrapRow(row)
			if err != nil {
				return err
			}
			wrapped, err := wrapKey(k.keks[k.activeKEK], k.activeKEK, dek)
			if err != nil {
				return err
			}
			if err := q.UpdateDataKeyWrap(ctx, dbgen.UpdateDataKeyWrapParams{ID: row.ID, KekID: k.activeKEK, WrappedKey: wrapped}); err != nil {
				return err
			}
		}
		n = len(rows)
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("keyring: rewrap data keys: %w", err)
	}
	return n, nil
}

func (k *Keyring) unwrapAll(rows []dbgen.ListDataKeysRow) (map[int16][]byte, error) {
	deks := make(map[int16][]byte, len(rows))
	for _, row := range rows {
		dek, err := k.unwrapRow(row)
		if err != nil {
			return nil, err
		}
		deks[row.ID] = dek
	}
	return deks, nil
}

// unwrapRow decrypts one data key. Errors name the KEK id and never include key bytes.
func (k *Keyring) unwrapRow(row dbgen.ListDataKeysRow) ([]byte, error) {
	kek, ok := k.keks[row.KekID]
	if !ok {
		return nil, fmt.Errorf("keyring: data key %d is wrapped with KEK %q, which is not in the KEK file", row.ID, row.KekID)
	}
	gcm, err := newGCM(kek)
	if err != nil {
		return nil, fmt.Errorf("keyring: KEK %q: %w", row.KekID, err)
	}
	if len(row.WrappedKey) < nonceSize+tagSize {
		return nil, fmt.Errorf("keyring: data key %d is too short", row.ID)
	}
	nonce, ct := row.WrappedKey[:nonceSize], row.WrappedKey[nonceSize:]
	dek, err := gcm.Open(nil, nonce, ct, wrapAAD(row.KekID))
	if err != nil {
		return nil, fmt.Errorf("keyring: cannot unwrap data key %d with KEK %q", row.ID, row.KekID)
	}
	if len(dek) != keySize {
		return nil, fmt.Errorf("keyring: data key %d has the wrong length", row.ID)
	}
	return dek, nil
}

func wrapAAD(kekID string) []byte { return []byte("data_keys:" + kekID) }

// wrapKey returns nonce || AES-256-GCM(kek, nonce, dek, aad = "data_keys:" + kekID).
func wrapKey(kek []byte, kekID string, dek []byte) ([]byte, error) {
	gcm, err := newGCM(kek)
	if err != nil {
		return nil, fmt.Errorf("KEK %q: %w", kekID, err)
	}
	nonce := make([]byte, nonceSize, nonceSize+len(dek)+tagSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, dek, wrapAAD(kekID)), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != keySize {
		return nil, errors.New("key has the wrong length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// readKEKFile parses lines of "<kek-id> <base64 std of 32 bytes>". Blank lines and lines starting
// with # are skipped. Errors name the line number only, never the content.
func readKEKFile(path string) (map[string][]byte, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the KEK file path is operator configuration (KEK_FILE), not request input
	if err != nil {
		return nil, fmt.Errorf("keyring: read KEK file: %w", err)
	}
	keks := map[string][]byte{}
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("keyring: KEK file line %d is invalid", i+1)
		}
		key, err := base64.StdEncoding.DecodeString(fields[1])
		if err != nil || len(key) != keySize {
			return nil, fmt.Errorf("keyring: KEK file line %d is invalid", i+1)
		}
		if _, dup := keks[fields[0]]; dup {
			return nil, fmt.Errorf("keyring: KEK file line %d repeats an id", i+1)
		}
		keks[fields[0]] = key
	}
	return keks, nil
}
