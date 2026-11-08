package keyring_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/dbtest"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/keyring"
)

// A35: Keyring Load creates the first DEK if empty, fails if active KEK is missing or if a wrapped key's KEK is missing.
func TestLoad(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	ctx := context.Background()

	kek1 := make([]byte, 32)
	if _, err := rand.Read(kek1); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	kekFile1 := filepath.Join(t.TempDir(), "kek")
	kekContent := fmt.Sprintf("k1 %s\n", base64.StdEncoding.EncodeToString(kek1))
	if err := os.WriteFile(kekFile1, []byte(kekContent), 0600); err != nil {
		t.Fatalf("write kek file 1: %v", err)
	}

	// 1. Empty table creates DEK 1
	kr, err := keyring.Load(ctx, db.App, kekFile1, "k1")
	if err != nil {
		t.Fatalf("Load on empty table: %v", err)
	}
	if kr == nil {
		t.Fatal("Load returned nil keyring")
	}

	var count int
	var minID, maxID int16
	var kekID string
	err = db.Super.QueryRow(ctx, "SELECT count(*), coalesce(min(id), 0), coalesce(max(id), 0), coalesce(min(kek_id), '') FROM data_keys").Scan(&count, &minID, &maxID, &kekID)
	if err != nil {
		t.Fatalf("query data_keys: %v", err)
	}
	if count != 1 {
		t.Fatalf("data_keys count = %d, want 1", count)
	}
	if minID != 1 || maxID != 1 {
		t.Fatalf("data_keys id = %d..%d, want 1", minID, maxID)
	}
	if kekID != "k1" {
		t.Fatalf("data_keys kek_id = %q, want %q", kekID, "k1")
	}

	// 2. KEK_ACTIVE_ID absent from the file fails naming the id
	_, err = keyring.Load(ctx, db.App, kekFile1, "missing-kek")
	if err == nil {
		t.Fatal("Load with missing active KEK succeeded, want error")
	}
	if !strings.Contains(err.Error(), "missing-kek") {
		t.Fatalf("error %q does not contain missing active KEK id %q", err.Error(), "missing-kek")
	}

	// 3. A file missing k1 while a row uses it fails naming "k1" without leaking base64 key material
	kek2 := make([]byte, 32)
	if _, err := rand.Read(kek2); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	kek2B64 := base64.StdEncoding.EncodeToString(kek2)
	kekFile2 := filepath.Join(t.TempDir(), "kek2")
	if err := os.WriteFile(kekFile2, []byte(fmt.Sprintf("k2 %s\n", kek2B64)), 0600); err != nil {
		t.Fatalf("write kek file 2: %v", err)
	}

	_, err = keyring.Load(ctx, db.App, kekFile2, "k2")
	if err == nil {
		t.Fatal("Load with missing unwrapping KEK succeeded, want error")
	}
	if !strings.Contains(err.Error(), "k1") {
		t.Fatalf("error %q does not contain missing KEK id %q", err.Error(), "k1")
	}
	if strings.Contains(err.Error(), kek2B64) {
		t.Fatalf("error %q leaks base64 key material", err.Error())
	}
}

// A17, A35: Seal encrypts with active DEK (version 0x01, BE uint16 DEK id, 12-byte nonce, GCM tag), Open decrypts and binds AAD, reloads on unknown DEK id.
func TestSealOpen(t *testing.T) {
	t.Parallel()
	db := dbtest.New(t)
	ctx := context.Background()

	kek1 := make([]byte, 32)
	if _, err := rand.Read(kek1); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	kekFile := filepath.Join(t.TempDir(), "kek")
	if err := os.WriteFile(kekFile, []byte(fmt.Sprintf("k1 %s\n", base64.StdEncoding.EncodeToString(kek1))), 0600); err != nil {
		t.Fatalf("write kek file: %v", err)
	}

	kr, err := keyring.Load(ctx, db.App, kekFile, "k1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	pt := []byte("confidential-totp-secret-data")
	aad := []byte("users.totp_secret:42")
	ct, err := kr.Seal(pt, aad)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	wantLen := 3 + 12 + len(pt) + 16
	if len(ct) != wantLen {
		t.Fatalf("ciphertext len = %d, want %d", len(ct), wantLen)
	}
	if ct[0] != 0x01 {
		t.Fatalf("ciphertext version byte = 0x%02x, want 0x01", ct[0])
	}
	activeDEK := binary.BigEndian.Uint16(ct[1:3])
	if activeDEK != 1 {
		t.Fatalf("ciphertext DEK id = %d, want 1", activeDEK)
	}

	got, err := kr.Open(ct, aad)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("Open = %q, want %q", got, pt)
	}

	// Wrong AAD fails
	_, err = kr.Open(ct, []byte("users.totp_secret:43"))
	if err == nil {
		t.Fatal("Open with wrong AAD succeeded, want error")
	}

	// Flipped byte fails
	flipped := slices.Clone(ct)
	flipped[len(flipped)-1] ^= 0x01
	_, err = kr.Open(flipped, aad)
	if err == nil {
		t.Fatal("Open with flipped byte succeeded, want error")
	}

	// Version 0x02 fails
	badVer := slices.Clone(ct)
	badVer[0] = 0x02
	_, err = kr.Open(badVer, aad)
	if err == nil {
		t.Fatal("Open with version 0x02 succeeded, want error")
	}

	// Unknown DEK id inserted after Load succeeds after reload
	dek2 := make([]byte, 32)
	if _, err := rand.Read(dek2); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	block, err := aes.NewCipher(kek1)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("rand.Read nonce: %v", err)
	}
	wrappedKey := gcm.Seal(nonce, nonce, dek2, []byte("data_keys:k1"))

	var dek2ID int16
	err = db.Super.QueryRow(ctx, "INSERT INTO data_keys (kek_id, wrapped_key) VALUES ($1, $2) RETURNING id", "k1", wrappedKey).Scan(&dek2ID)
	if err != nil {
		t.Fatalf("insert DEK 2: %v", err)
	}

	pt2 := []byte("reload-test-payload")
	aad2 := []byte("users.totp_secret:99")
	block2, err := aes.NewCipher(dek2)
	if err != nil {
		t.Fatalf("aes.NewCipher dek2: %v", err)
	}
	gcm2, err := cipher.NewGCM(block2)
	if err != nil {
		t.Fatalf("cipher.NewGCM dek2: %v", err)
	}
	nonce2 := make([]byte, 12)
	if _, err := rand.Read(nonce2); err != nil {
		t.Fatalf("rand.Read nonce2: %v", err)
	}
	ctPayload2 := gcm2.Seal(nil, nonce2, pt2, aad2)

	ct2 := make([]byte, 0, 3+len(nonce2)+len(ctPayload2))
	ct2 = append(ct2, 0x01)
	idBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(idBuf, uint16(dek2ID)) //nolint:gosec // a small positive id
	ct2 = append(ct2, idBuf...)
	ct2 = append(ct2, nonce2...)
	ct2 = append(ct2, ctPayload2...)

	got2, err := kr.Open(ct2, aad2)
	if err != nil {
		t.Fatalf("Open after DEK inserted (reload): %v", err)
	}
	if !bytes.Equal(got2, pt2) {
		t.Fatalf("Open got %q, want %q", got2, pt2)
	}
}
