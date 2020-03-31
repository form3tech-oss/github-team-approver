package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/form3tech-oss/logrus-logzio-hook/pkg/hook"
	"github.com/google/go-github/v28/github"
	"github.com/logzio/logzio-go"
	log "github.com/sirupsen/logrus"
)

var (
	// githubWebhookSecretToken contains the secret token used to validate incoming payloads.
	githubWebhookSecretToken []byte
	// ignoredRepositories is the list of repositories for which events will be ignored.
	ignoredRepositories []string
)

func init() {
	// Configure the log level.
	if v, err := log.ParseLevel(os.Getenv(envLogLevel)); err == nil {
		log.SetLevel(v)
	} else {
		log.Warnf("Failed to parse log level, falling back to %q: %v", log.GetLevel().String(), err)
	}
	// Configure log shipping to Logz.io.
	if t, err := ioutil.ReadFile(os.Getenv(envLogzioTokenPath)); err != nil {
		log.Warnf("Failed to configure the logz.io logrus hook: %v", err)
	} else {
		if c, err := logzio.New(string(t), logzio.SetUrl(logzioListenerURL)); err != nil {
			log.Warnf("Failed to configure the logz.io logrus hook: %v", err)
		} else {
			log.AddHook(hook.NewLogzioHook(c))
		}
	}
	// Read the webhook secret token.
	b, err := ioutil.ReadFile(os.Getenv(envGitHubAppWebhookSecretTokenPath))
	if err != nil {
		// Warn but do not fail, making all requests be rejected.
		log.Warnf("Failed to read webhook secret token: %v", err)
	}
	githubWebhookSecretToken = b
	// Parse the list of ignored repositories.
	ignoredRepositories = strings.Split(os.Getenv(envIgnoredRepositories), ",")
}

// Handle handles an HTTP request.
func Handle(res http.ResponseWriter, req *http.Request) {
	// Make sure we're dealing with a POST request.
	if req.Method != http.MethodPost {
		sendHttpMethodNotAllowedResponse(res, fmt.Errorf("unsupported method %q", req.Method))
		return
	}

	var (
		ctx       = newRequestContext(req)
		eventType = req.Header.Get(httpHeaderXGithubEvent)
		event     event
		signature = req.Header.Get(httpHeaderXHubSignature)
	)

	// Read the request's body.
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		getLogger(ctx).Errorf("Failed to validate payload: %v", err)
		sendHttpBadRequestResponse(res, fmt.Errorf("failed to validate payload: %v", err))
		return
	}

	// Validate the incoming payload if we're configured to do so.
	if len(githubWebhookSecretToken) != 0 {
		if err := github.ValidateSignature(signature, body, githubWebhookSecretToken); err != nil {
			getLogger(ctx).Errorf("Failed to validate payload: %v", err)
			sendHttpBadRequestResponse(res, fmt.Errorf("failed to validate payload: %v", err))
			return
		}
	}

	// Unmarshal the incoming event.
	switch eventType {
	case eventTypePullRequest:
		var (
			e github.PullRequestEvent
		)

		if err := json.Unmarshal(body, &e); err != nil {
			getLogger(ctx).Errorf("Failed to unmarshal request into PullRequestEvent: %v", err)
			sendHttpBadRequestResponse(res, fmt.Errorf("failed to unmarshal request into PullRequestEvent: %v", err))
			return
		} else {
			event = &e
		}
	case eventTypePullRequestReview:
		var (
			e github.PullRequestReviewEvent
		)
		if err := json.Unmarshal(body, &e); err != nil {
			getLogger(ctx).Errorf("Failed to unmarshal request into PullRequestReviewEvent: %v", err)
			sendHttpBadRequestResponse(res, fmt.Errorf("failed to unmarshal request into PullRequestReviewEvent: %v", err))
			return
		} else {
			event = &e
		}
	default:
		getLogger(ctx).Warn("Ignoring event: unsupported type")
		sendHttpNoContentResponse(res)
		return
	}

	// Update the current request's context.
	ctx = updateRequestContext(ctx, eventType, event)

	// If the source repository is in the list of ignored repositories, stop processing.
	rfn := event.GetRepo().GetFullName()
	if indexOf(ignoredRepositories, rfn) >= 0 {
		getLogger(ctx).Warn("Ignoring event: ignored repository")
		sendHttpNoContentResponse(res)
		return
	}

	// Handle the incoming event.
	if r, err := handleEvent(ctx, eventType, event); err != nil {
		if err == errNoConfigurationFile {
			getLogger(ctx).Warnf("Ignoring event: %v", err)
			sendHttpNoContentResponse(res)
			return
		}
		getLogger(ctx).Errorf("Failed to handle event: %v", err)
		sendHttpInternalServerErrorResponse(res, fmt.Errorf("failed to handle event: %v", err))
		return
	} else {
		getLogger(ctx).Tracef("%q will be reported as the status", r)
		sendHttpOkResponse(res, r)
		return
	}
}

func sendHttpResponse(res http.ResponseWriter, statusCode int, message string) {
	res.WriteHeader(statusCode)
	res.Write([]byte(message))
}

func sendHttpOkResponse(res http.ResponseWriter, finalStatus string) {
	res.Header().Set(httpHeaderXFinalStatus, finalStatus)
	res.WriteHeader(http.StatusOK)
}

func sendHttpBadRequestResponse(res http.ResponseWriter, err error) {
	sendHttpResponse(res, http.StatusBadRequest, err.Error())
}

func sendHttpInternalServerErrorResponse(res http.ResponseWriter, err error) {
	sendHttpResponse(res, http.StatusInternalServerError, err.Error())
}

func sendHttpMethodNotAllowedResponse(res http.ResponseWriter, err error) {
	sendHttpResponse(res, http.StatusMethodNotAllowed, err.Error())
}

func sendHttpNoContentResponse(res http.ResponseWriter) {
	sendHttpResponse(res, http.StatusNoContent, "")
}
