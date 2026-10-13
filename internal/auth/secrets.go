package auth

import (
	"fmt"
	"os"
	"path/filepath"
)

// SecretStore persists sensitive values outside SQLite.
type SecretStore struct {
	dir string
}

func NewSecretStore(dataDir string) *SecretStore {
	return &SecretStore{dir: filepath.Join(dataDir, "secrets")}
}

func (s *SecretStore) Write(name, value string) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(s.dir, name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(value), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Create writes name only when it doesn't exist yet, and returns what it
// holds afterwards: value when this call made it, or what another caller
// wrote first. Two callers making a value at once, in one process or two,
// end up with the same one (#602).
func (s *SecretStore) Create(name, value string) (string, error) {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(s.dir, name+".*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	// A link fails when the name exists, so only the first caller's value
	// lands, complete.
	if err := os.Link(tmp.Name(), filepath.Join(s.dir, name)); err != nil {
		if !os.IsExist(err) {
			return "", err
		}
		return s.Read(name)
	}
	return value, nil
}

func (s *SecretStore) Read(name string) (string, error) {
	path := filepath.Join(s.dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SecretStore) Delete(name string) error {
	path := filepath.Join(s.dir, name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *SecretStore) Path(name string) string {
	return filepath.Join(s.dir, name)
}

func (s *SecretStore) EnsureDir() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create secrets dir: %w", err)
	}
	return nil
}
