package api

import (
	"io/ioutil"
	"os"
	"strings"
)

type SecretStore interface {
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
	return ioutil.ReadFile(os.Getenv(envVariable))
}

func NewSSMStore() *EnvSecretStore {
	return &EnvSecretStore{}
}

func NewEnvSecretStore() *FileSecretStore {
	return &FileSecretStore{}
}
