package api

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/form3tech-oss/github-team-approver/internal/api/github"
	"github.com/form3tech-oss/github-team-approver/internal/api/secret"
	log "github.com/sirupsen/logrus"
)

const (
	defaultAppName = "github-team-approver"

	logTimeFormat = "2006-01-02T15:04:05.000Z07:00"

	envAppName                         = "APP_NAME"
	envGitHubAppWebhookSecretTokenPath = "GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH"
	envIgnoredRepositories             = "IGNORED_REPOSITORIES"
	envLogLevel                        = "LOG_LEVEL"
	envLogFormat                       = "LOG_FORMAT"
	envSecretStoreType                 = "SECRET_STORE_TYPE" // Set to AWS_SSM for the ability to run in ECS using SSM. Empty, not set or anything else for default K8s secret
	envSlackWebhookSecret              = "SLACK_WEBHOOK_SECRET"
)

type API struct {
	AppName                  string
	SecretStore              secret.Store
	githubWebhookSecretToken []byte
	ignoredRepositories      []string
	slackWebhookSecret       string
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
	api.setSlackWebhookSecret()
	api.setIgnoredRepositories()
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

func (api *API) setSlackWebhookSecret() {
	webhook, err := api.SecretStore.Get(envSlackWebhookSecret)
	if err != nil {
		log.WithError(err).Warn("Slack notification won't be send: failed to read slack webhook secret")
		return
	}
	api.slackWebhookSecret = string(webhook)
	log.Info("Configured Slack Webhook Secret")
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
