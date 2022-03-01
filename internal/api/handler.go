package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	ghclient "github.com/form3tech-oss/github-team-approver/internal/api/github"
	"github.com/google/go-github/v42/github"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

const (
	logFieldDeliveryID  = "delivery_id"
	logFieldEventType   = "event_type"
	logFieldPR          = "pr"
	logFieldRepo        = "repo"
	logFieldServiceName = "service_name"

	httpHeaderXFinalStatus    = "X-Final-Status"
	httpHeaderXGithubDelivery = "X-GitHub-Delivery"
	httpHeaderXGithubEvent    = "X-GitHub-Event"
	httpHeaderXHubSignature   = "X-Hub-Signature"
)

var (
	errIgnoredEvent = fmt.Errorf("ignoring event: unsupported type")
)

func (api *API) HandleHealth(w http.ResponseWriter, req *http.Request) {
	sendHttpOkResponse(w)
}
func (api *API) Handle(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		sendHttpMethodNotAllowedResponse(w, fmt.Errorf("unsupported method %q", req.Method))
		return
	}

	eventType := req.Header.Get(httpHeaderXGithubEvent)
	deliveryID := req.Header.Get(httpHeaderXGithubDelivery)

	fields := logrus.Fields{
		logFieldServiceName: api.AppName,
		logFieldDeliveryID:  deliveryID,
		logFieldEventType:   eventType,
	}

	log := logrus.NewEntry(logrus.StandardLogger()).WithFields(fields)

	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		log.WithError(err).Error("Failed to validate payload")
		sendHttpBadRequestResponse(w, fmt.Errorf("failed to validate payload: %w", err))
		return
	}
	signature := req.Header.Get(httpHeaderXHubSignature)

	err = api.validateSignature(signature, body)
	if err != nil {
		log.WithError(err).Error("Failed to validate payload")
		sendHttpBadRequestResponse(w, err)
		return
	}

	event, err := getSupportedEvent(eventType)
	if err != nil {
		log.WithError(err).Warn("not handled")
		sendHttpNoContentResponse(w)
		return
	}

	err = unmarshalEvent(body, event)
	if err != nil {
		log.WithError(err).Error("unmarshal request body")
		sendHttpBadRequestResponse(w, fmt.Errorf("unmarshal request body: %w", err))
	}

	repoName := event.GetRepo().GetFullName()
	log = log.WithFields(
		logrus.Fields{
			logFieldRepo: repoName,
			logFieldPR:   event.GetPullRequest().GetNumber(),
		})

	if isMember(api.ignoredRepositories, repoName) {
		log.Warn("ignoring event: ignored repository")
		sendHttpNoContentResponse(w)
		return
	}
	client := ghclient.New(api.SecretStore)

	ctx := context.Background()
	if isPrMergeEvent(event) {
		mergeHandler := NewMergeEventHandler(api, log, client)
		if err := mergeHandler.handlePrMergeEvent(ctx, event); err != nil {
			sendHttpInternalServerErrorResponse(w, fmt.Errorf("failed to handle event: %w", err))
			return
		}
		sendHttpOkResponse(w)
		return
	}

	handler := NewPullRequestEventHandler(api, log, client)

	status, err := handler.handleEvent(ctx, eventType, event)
	if errors.Is(err, ghclient.ErrNoConfigurationFile) {
		log.WithError(err).Warn("ignoring event")
		sendHttpNoContentResponse(w)
		return
	}
	if err != nil {
		log.WithField("event", event).
			WithError(err).
			Warn("failed to handle event")
		sendHttpInternalServerErrorResponse(w, fmt.Errorf("failed to handle event: %w", err))
		return
	}

	sendHttpOkWithStatusResponse(w, status)
	return
}

func (api *API) validateSignature(signature string, body []byte) error {
	if api.githubWebhookSecretToken == nil {
		// TODO we should make this more clear, following what we had from before now
		// see setGitHubAppSecret api comments
		return nil
	}

	if err := github.ValidateSignature(signature, body, api.githubWebhookSecretToken); err != nil {
		return fmt.Errorf("failed to validate payload: %w", err)
	}
	return nil
}

func sendHttpResponse(res http.ResponseWriter, statusCode int, message string) {
	res.WriteHeader(statusCode)
	res.Write([]byte(message))
}

func sendHttpOkWithStatusResponse(res http.ResponseWriter, finalStatus string) {
	res.Header().Set(httpHeaderXFinalStatus, finalStatus)
	res.WriteHeader(http.StatusOK)
}

func sendHttpOkResponse(res http.ResponseWriter) {
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

func unmarshalEvent(body []byte, v interface{}) error {
	err := json.Unmarshal(body, v)
	if err != nil {
		return err
	}
	return nil
}

func isMember(items []string, v string) bool {
	for _, i := range items {
		if i == v {
			return true
		}
	}
	return false
}
