package secret

import (
	"os"
	"strings"
)

type Store interface {
	Get(envVariable string) ([]byte, error)
}

type EnvSecretStore struct {
}

func (s *EnvSecretStore) Get(envVariable string) ([]byte, error) {
	return []byte(strings.ReplaceAll(os.Getenv(envVariable), "\\n", "\n")), nil
}

type FileSecretStore struct {
}

func (s *FileSecretStore) Get(envVariable string) ([]byte, error) {
	return os.ReadFile(os.Getenv(envVariable))
}

func NewSSMStore() *EnvSecretStore {
	return &EnvSecretStore{}
}

func NewEnvSecretStore() *FileSecretStore {
	return &FileSecretStore{}
}
