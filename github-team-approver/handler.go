package function

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/form3tech-oss/logrus-logzio-hook/pkg/hook"
	"github.com/google/go-github/github"
	"github.com/logzio/logzio-go"
	handler "github.com/openfaas-incubator/go-function-sdk"
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
func Handle(req handler.Request) (handler.Response, error) {
	// Make sure we're dealing with a POST request.
	if req.Method != http.MethodPost {
		return newHttpMethodNotAllowedResponse(fmt.Errorf("unsupported method %q", req.Method)), nil
	}

	var (
		ctx       = newRequestContext(req)
		eventType = req.Header.Get(httpHeaderXGithubEvent)
		event     event
		signature = req.Header.Get(httpHeaderXHubSignature)
	)

	// Validate the incoming payload if we're configured to do so.
	if len(githubWebhookSecretToken) != 0 {
		if err := github.ValidateSignature(signature, req.Body, githubWebhookSecretToken); err != nil {
			getLogger(ctx).Errorf("Failed to validate payload: %v", err)
			return newHttpBadRequestResponse(fmt.Errorf("failed to validate payload: %v", err)), nil
		}
	}

	// Unmarshal the incoming event.
	switch eventType {
	case eventTypePullRequest:
		var (
			e github.PullRequestEvent
		)
		if err := json.Unmarshal(req.Body, &e); err != nil {
			getLogger(ctx).Errorf("Failed to unmarshal request into PullRequestEvent: %v", err)
			return newHttpBadRequestResponse(fmt.Errorf("failed to unmarshal request into PullRequestEvent: %v", err)), nil
		} else {
			event = &e
		}
	case eventTypePullRequestReview:
		var (
			e github.PullRequestReviewEvent
		)
		if err := json.Unmarshal(req.Body, &e); err != nil {
			getLogger(ctx).Errorf("Failed to unmarshal request into PullRequestReviewEvent: %v", err)
			return newHttpBadRequestResponse(fmt.Errorf("failed to unmarshal request into PullRequestReviewEvent: %v", err)), nil
		} else {
			event = &e
		}
	default:
		getLogger(ctx).Warn("Ignoring event: unsupported type")
		return newHttpNoContentResponse(), nil
	}

	// Update the current request's context.
	ctx = updateRequestContext(ctx, eventType, event)

	// If the source repository is in the list of ignored repositories, stop processing.
	rfn := event.GetRepo().GetFullName()
	if indexOf(ignoredRepositories, rfn) >= 0 {
		getLogger(ctx).Warn("Ignoring event: ignored repository")
		return newHttpNoContentResponse(), nil
	}

	// Handle the incoming event.
	if r, err := handleEvent(ctx, eventType, event); err != nil {
		if err == errNoConfigurationFile {
			getLogger(ctx).Warnf("Ignoring event: %v", err)
			return newHttpNoContentResponse(), nil
		}
		getLogger(ctx).Errorf("Failed to handle event: %v", err)
		return newHttpInternalServerErrorResponse(fmt.Errorf("failed to handle event: %v", err)), nil
	} else {
		getLogger(ctx).Tracef("%q will be reported as the status", r)
		return newHttpOkResponse(r), nil
	}
}

func newHttpResponse(statusCode int, message string) handler.Response {
	return handler.Response{
		Body:       []byte(message),
		Header:     http.Header{},
		StatusCode: statusCode,
	}
}

func newHttpOkResponse(finalStatus string) handler.Response {
	r := newHttpResponse(http.StatusOK, "")
	r.Header.Set(httpHeaderXFinalStatus, finalStatus)
	return r
}

func newHttpBadRequestResponse(err error) handler.Response {
	return newHttpResponse(http.StatusBadRequest, err.Error())
}

func newHttpInternalServerErrorResponse(err error) handler.Response {
	return newHttpResponse(http.StatusInternalServerError, err.Error())
}

func newHttpMethodNotAllowedResponse(err error) handler.Response {
	return newHttpResponse(http.StatusMethodNotAllowed, err.Error())
}

func newHttpNoContentResponse() handler.Response {
	return newHttpResponse(http.StatusNoContent, "")
}
