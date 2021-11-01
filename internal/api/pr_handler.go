package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/form3tech-oss/github-team-approver/internal/api/approval"
	ghclient "github.com/form3tech-oss/github-team-approver/internal/api/github"
	"github.com/google/go-github/v28/github"
	"github.com/sirupsen/logrus"
	"sync"
)

const (
	eventTypePullRequest       = "pull_request"
	eventTypePullRequestReview = "pull_request_review"

	pullRequestActionEdited      = "edited"
	pullRequestActionOpened      = "opened"
	pullRequestActionReopened    = "reopened"
	pullRequestActionSynchronize = "synchronize"

	pullRequestReviewActionDismissed = "dismissed"
	pullRequestReviewActionEdited    = pullRequestActionEdited
	pullRequestReviewActionSubmitted = "submitted"
)

type PullRequestEventHandler struct {
	api    *API
	log    *logrus.Entry
	client *ghclient.Client
}

func NewPullRequestEventHandler(api *API, log *logrus.Entry, client *ghclient.Client) *PullRequestEventHandler {
	return &PullRequestEventHandler{
		api:    api,
		log:    log,
		client: client,
	}
}

// handleEvent handles a GitHub event (which should be of type "pull_request" or "pull_request_review").
// It does so by computing the status and the final set of labels to apply to the PR, and reporting these.
func (handler *PullRequestEventHandler) handleEvent(ctx context.Context, eventType string, event event) (finalStatus string, err error) {
	var (
		ownerLogin = event.GetRepo().GetOwner().GetLogin()
		repoName   = event.GetRepo().GetName()
		prNumber   = event.GetPullRequest().GetNumber()
	)

	// Make sure the combination of event type and action is supported.
	action := event.GetAction()
	if !isSupportedAction(eventType, action) {
		handler.log.Warnf("ignoring action of type %q", action)
		return "", nil
	}

	prTargetBranch := event.GetPullRequest().GetBase().GetRef()
	prBody := event.GetPullRequest().GetBody()
	prLabels := getLabelNames(event.GetPullRequest().Labels)

	pr := approval.NewPR(ownerLogin, repoName, prTargetBranch, prBody, prNumber, prLabels)
	app := approval.NewApproval(handler.log, handler.client)
	result, err := app.ComputeApprovalStatus(ctx, pr)
	if errors.Is(err, ghclient.ErrNoConfigurationFile) {
		return "", err
	}

	if err != nil {
		return "", fmt.Errorf("failed to compute status: %w", err)
	}

	// Report the approval status, request reviews from the approving teams, and update the PR's labels.
	ch := make(chan error, 3)
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		handler.log.Tracef("Reporting %q as the status", result.Status())
		statusesURL := event.GetPullRequest().GetStatusesURL()
		if err := handler.client.ReportStatus(ctx, ownerLogin, repoName, statusesURL, result.Status(), result.Description()); err != nil {
			handler.log.WithError(err).Error("Failed to report status")
			ch <- err
		}
	}()
	go func() {
		defer wg.Done()
		handler.log.Tracef("Requesting reviews from %v", result.ReviewsToRequest())
		if err := handler.client.RequestReviews(ctx, ownerLogin, repoName, prNumber, result.ReviewsToRequest()); err != nil {
			handler.log.WithError(err).Error("Failed to request reviews")
			ch <- err
		}
	}()
	go func() {
		defer wg.Done()
		handler.log.Tracef("Updating labels to %v", result.FinalLabels())
		if err := handler.client.UpdateLabels(ctx, ownerLogin, repoName, prNumber, result.FinalLabels()); err != nil {
			handler.log.WithError(err).Error("Failed to update labels")
			ch <- err
		}
	}()
	wg.Wait()

	// Propagate a single error, or return the computed state.
	select {
	case err := <-ch:
		return "", err
	default:
		return result.Status(), nil
	}
}

func isSupportedAction(eventType, action string) bool {
	switch {
	case eventType == eventTypePullRequest:
		return action == pullRequestActionEdited || action == pullRequestActionOpened || action == pullRequestActionReopened || action == pullRequestActionSynchronize
	case eventType == eventTypePullRequestReview:
		return action == pullRequestReviewActionDismissed || action == pullRequestReviewActionEdited || action == pullRequestReviewActionSubmitted
	default:
		return false
	}
}

func getLabelNames(labels []*github.Label) []string {
	if labels == nil {
		return make([]string, 0, 0)
	}
	r := make([]string, 0, len(labels))
	for _, l := range labels {
		r = append(r, l.GetName())
	}
	return r
}
