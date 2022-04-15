package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/form3tech-oss/github-team-approver/internal/api/aes"
	"github.com/form3tech-oss/github-team-approver/internal/api/github"
	"github.com/form3tech-oss/github-team-approver/internal/api/secret"
	log "github.com/sirupsen/logrus"
)

const (
	defaultAppName = "github-team-approver"

	logTimeFormat = "2006-01-02T15:04:05.000Z07:00"

	logzioListenerURL = "https://listener-eu.logz.io:8071"

	envAppName                         = "APP_NAME"
	envGitHubAppWebhookSecretTokenPath = "GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH"
	envIgnoredRepositories             = "IGNORED_REPOSITORIES"
	envLogLevel                        = "LOG_LEVEL"
	envLogFormat                       = "LOG_FORMAT"
	envSecretStoreType                 = "SECRET_STORE_TYPE" // Set to AWS_SSM for the ability to run in ECS using SSM. Empty, not set or anything else for default K8s secret
	envEncryptionKeyPath               = "ENCRYPTION_KEY_PATH"
)

type API struct {
	AppName                  string
	SecretStore              secret.Store
	cipher                   aes.Cipher
	githubWebhookSecretToken []byte
	ignoredRepositories      []string
}

func newApi() *API {
	return &API{}
}

func Start(address string, shutdown <-chan os.Signal, ready chan<- struct{}) {
	api := newApi()

	api.init()
	api.startServer(address, shutdown, ready)
}

func (api *API) init() {
	api.setAppName()
	api.initSecretStore(os.Getenv(envSecretStoreType))
	api.configureLogger()
	api.setGitHubAppSecret()
	api.setIgnoredRepositories()
	api.initAES()
}

func (api *API) setAppName() {
	api.AppName = getAppNameOrDefault()
}

func (api *API) initSecretStore(env string) {
	api.SecretStore = getSecretStore(env)
	log.WithField("env", env).Info("Configured Secret Store")
}

func (api *API) configureLogger() {
	if v, err := log.ParseLevel(os.Getenv(envLogLevel)); err == nil {
		log.SetLevel(v)
		log.WithField("log_level", v).
			Info("log level")
	} else {
		log.WithError(err).Warnf("failed to parse log level, falling back to %q", log.GetLevel().String())
	}

	logFormat := os.Getenv(envLogFormat)
	if strings.EqualFold(logFormat, "json") {
		log.SetFormatter(
			&log.JSONFormatter{
				FieldMap: log.FieldMap{
					log.FieldKeyMsg:  "message",
					log.FieldKeyTime: "@timestamp",
				},
				TimestampFormat: logTimeFormat,
			})
	}
	log.Info("Configured logger")
}

func (api *API) setGitHubAppSecret() {
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
	log.Info("Configured GitHub App Secret")
}

func (api *API) setIgnoredRepositories() {
	v, ok := os.LookupEnv(envIgnoredRepositories)
	if !ok {
		api.ignoredRepositories = []string{}
		return
	}

	ignored := strings.Split(v, ",")
	api.ignoredRepositories = ignored
	log.Info("Configured Ignored repositories")
}

func (api *API) initAES() {
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
	log.Info("Configured cipher")
}

func (api *API) GetCipher() (aes.Cipher, error) {
	if api.cipher == nil {
		return nil, fmt.Errorf("AES cipher not initialized")
	}
	return api.cipher, nil
}

func (api *API) startServer(address string, shutdown <-chan os.Signal, ready chan<- struct{}) {

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
