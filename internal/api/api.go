package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/form3tech-oss/github-team-approver/internal/api/aes"
	"github.com/form3tech-oss/github-team-approver/internal/api/github"
	"github.com/form3tech-oss/github-team-approver/internal/api/secret"
	"github.com/form3tech-oss/logrus-logzio-hook/pkg/hook"
	"github.com/logzio/logzio-go"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"strings"
)

const (
	defaultAppName = "github-team-approver"

	logzioListenerURL = "https://listener-eu.logz.io:8071"

	envAppName                         = "APP_NAME"
	envGitHubAppWebhookSecretTokenPath = "GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH"
	envIgnoredRepositories             = "IGNORED_REPOSITORIES"
	envLogLevel                        = "LOG_LEVEL"
	envSecretStoreType                 = "SECRET_STORE_TYPE" // Set to AWS_SSM for the ability to run in ECS using SSM. Empty, not set or anything else for default K8s secret
	envLogzioTokenPath                 = "LOGZIO_TOKEN_PATH"
	envEncryptionKeyPath               = "ENCRYPTION_KEY_PATH"
)

type Api struct {
	AppName                  string
	SecretStore              secret.Store
	cipher                   aes.Cipher
	githubWebhookSecretToken []byte
	ignoredRepositories      []string
}

func newApi() *Api {
	return &Api{}
}

func Start(address string, shutdown <-chan os.Signal, ready chan<- struct{}) {
	api := newApi()

	api.init()
	api.startServer(address, shutdown, ready)
}

func (api *Api) init() {
	api.setAppName()
	api.initSecretStore(os.Getenv(envSecretStoreType))
	api.configureLogger()
	api.setGitHubAppSecret()
	api.setIgnoredRepositories()
	api.initAES()
}

func (api *Api) setAppName() {
	api.AppName = getAppNameOrDefault()
}

func (api *Api) initSecretStore(env string) {
	api.SecretStore = getSecretStore(env)
}

func (api *Api) configureLogger() {
	if v, err := log.ParseLevel(os.Getenv(envLogLevel)); err == nil {
		log.SetLevel(v)
		log.WithField("log_level", v).
			Info("log level")
	} else {
		log.WithError(err).Warnf("failed to parse log level, falling back to %q", log.GetLevel().String())
	}

	// Configure log shipping to Logz.io.
	if t, err := api.SecretStore.Get(envLogzioTokenPath); err != nil {
		log.WithError(err).Warn("failed to read the logz.io token from the configured path")
	} else {
		if c, err := logzio.New(string(t), logzio.SetUrl(logzioListenerURL)); err != nil {
			log.WithError(err).Warn("failed to configure the logz.io logrus hook")
		} else {
			log.AddHook(hook.NewLogzioHook(c))
		}
	}
}

func (api *Api) setGitHubAppSecret() {
	// Read the webhook secret token.
	token, err := api.SecretStore.Get(envGitHubAppWebhookSecretTokenPath)
	if err != nil {
		// TODO make this more explicit, if we want to check signatures, it should fail rather than silently not
		// verifying signatures.
		// Configuration error shouldn't be treated as an option
		// Warn but do not fail, making all requests be rejected.
		log.WithError(err).Warn("GitHub Event signatures won't be checked: failed to read webhook secret token")
		// TODO we should handle this differently, making it explicit here as to what we're setting
		api.githubWebhookSecretToken = nil
		return
	}
	api.githubWebhookSecretToken = token
}

func (api *Api) setIgnoredRepositories() {
	v, ok := os.LookupEnv(envIgnoredRepositories)
	if !ok {
		api.ignoredRepositories = []string{}
		return
	}

	ignored := strings.Split(v, ",")
	api.ignoredRepositories = ignored
}

func (api *Api) initAES() {
	// Read the encryption key for slack web hooks
	k, err := api.SecretStore.Get(envEncryptionKeyPath)
	if err != nil {
		// Warn but do not fail, meaning we will not be able to decrypt slack hooks
		log.WithError(err).Warn("Failed to read decryption key")
	}

	key, err := hex.DecodeString(string(k))
	if err != nil {
		// Warn but do not fail, meaning we will not be able to decrypt slack hooks
		log.WithError(err).Warn("Failed to read decryption key")
	}

	cipher, err := aes.NewCipher(key)
	if err != nil {
		log.WithError(err).Warn("Failed to create AES cipher for decrypting")
		return
	}
	api.cipher = cipher
}

func (api *Api) GetCipher() (aes.Cipher, error) {
	if api.cipher == nil {
		return nil, fmt.Errorf("AES cipher not initialized")
	}
	return api.cipher, nil
}

func (api *Api) startServer(address string, shutdown <-chan os.Signal, ready chan<- struct{}) {

	m := http.NewServeMux()
	m.HandleFunc("/health", api.HandleHealth)
	m.HandleFunc("/events", api.Handle)
	m.HandleFunc("/function/github-team-approver", api.Handle) // Keep backwards-compatibility.
	srv := &http.Server{Addr: address, Handler: m}

	go func() {
		log.Infof("listening on: %s", address)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("failed to serve HTTP")
		}
	}()
	close(ready)

	<-shutdown
	log.Infof("SIGINT or SIGTERM received, shutting server down")
	ctx, cancel := context.WithTimeout(context.Background(), github.DefaultGitHubOperationTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("context forced to shutdown")
	}
	log.Info("server shutdown")
}

func getSecretStore(env string) secret.Store {
	if env == "AWS_SSM" {
		return secret.NewSSMStore()
	}
	return secret.NewEnvSecretStore()
}

func getAppNameOrDefault() string {
	if v := os.Getenv(envAppName); v != "" {
		return v
	}
	return defaultAppName
}
