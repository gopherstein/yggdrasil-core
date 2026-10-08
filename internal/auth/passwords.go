package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Passwords (#206) are hashed with argon2id, OWASP's second recommended
// setting: 19 MiB, two passes, one lane.
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	// MinPassword is the shortest password Toskar accepts.
	MinPassword = 10
)

// ErrWeakPassword is a password too short to keep.
var ErrWeakPassword = fmt.Errorf("a password needs at least %d characters", MinPassword)

// HashPassword is a password's argon2id hash, in the PHC string format.
func HashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < MinPassword {
		return "", ErrWeakPassword
	}
	return HashSecret(password)
}

// HashSecret is a secret's argon2id hash, like a password's but of any
// length, such as a chat portal's shared passcode (#205), which its caller
// checks.
func HashSecret(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// CheckPassword reports whether password matches hash.
func CheckPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var memory, passes uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &passes, &threads); err != nil {
		return false
	}
	b64 := base64.RawStdEncoding
	salt, err1 := b64.DecodeString(parts[4])
	want, err2 := b64.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, passes, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is checked when a username doesn't exist, so a wrong username
// takes as long as a wrong password.
var dummyHash, _ = HashPassword("not-a-real-password-just-for-timing")

// ErrSignIn is a username and password that don't match, said the same way
// whichever was wrong.
var ErrSignIn = errors.New("that username and password don't match")
