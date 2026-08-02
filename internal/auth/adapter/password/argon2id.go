package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	timeParam    = 1
	memoryParam  = 64 * 1024
	threadsParam = 4
	keyLength    = 32
	saltLength   = 16
)

var ErrMismatch = errors.New("password mismatch")

type Argon2id struct{}

func NewArgon2id() *Argon2id { return &Argon2id{} }

func (a *Argon2id) Hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, timeParam, memoryParam, threadsParam, keyLength)
	// encoded: argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
	return fmt.Sprintf(
		"argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memoryParam,
		timeParam,
		threadsParam,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func (a *Argon2id) Compare(encoded, password string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return ErrMismatch
	}
	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return ErrMismatch
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return ErrMismatch
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrMismatch
	}
	got := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(want)))
	if subtle.ConstantTimeCompare(want, got) != 1 {
		return ErrMismatch
	}
	return nil
}
