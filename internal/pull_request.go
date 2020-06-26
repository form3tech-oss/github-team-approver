package internal

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/go-github/v28/github"
)

type event interface {
	GetAction() string
	GetPullRequest() *github.PullRequest
	GetRepo() *github.Repository
}

// handleEvent handles a GitHub event (which should be of type "pull_request" or "pull_request_review").
// It does so by computing the status and the final set of labels to apply to the PR, and reporting these.
func handleEvent(ctx context.Context, eventType string, event event) (finalStatus string, err error) {
	var (
		action         = event.GetAction()
		ownerLogin     = event.GetRepo().GetOwner().GetLogin()
		repoName       = event.GetRepo().GetName()
		prNumber       = event.GetPullRequest().GetNumber()
		prTargetBranch = event.GetPullRequest().GetBase().GetRef()
		prBody         = event.GetPullRequest().GetBody()
		prLabels       = getLabelNames(event.GetPullRequest().Labels)
		statusesURL    = event.GetPullRequest().GetStatusesURL()
	)

	// Make sure the combination of event type and action is supported.
	if !isSupportedAction(eventType, action) {
		getLogger(ctx).Warnf("ignoring action of type %q", action)
		return "", nil
	}

	// Compute the approval status.
	status, description, finalLabels, reviewsToRequest, err := computeApprovalStatus(ctx, getClient(), ownerLogin, repoName, prNumber, prTargetBranch, prBody, prLabels)
	if err != nil {
		if err == errNoConfigurationFile {
			return "", err
		}
		return "", fmt.Errorf("failed to compute status: %v", err)
	}

	// Report the approval status, request reviews from the approving teams, and update the PR's labels.
	ch := make(chan error, 3)
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()
		getLogger(ctx).Tracef("Reporting %q as the status", status)
		if err := getClient().reportStatus(ownerLogin, repoName, statusesURL, status, description); err != nil {
			getLogger(ctx).Errorf("Failed to report status: %v", err)
			ch <- err
		}
	}()
	go func() {
		defer wg.Done()
		getLogger(ctx).Tracef("Requesting reviews from %v", reviewsToRequest)
		if err := getClient().requestReviews(ownerLogin, repoName, prNumber, reviewsToRequest); err != nil {
			getLogger(ctx).Errorf("Failed to request reviews: %v", err)
			ch <- err
		}
	}()
	go func() {
		defer wg.Done()
		getLogger(ctx).Tracef("Updating labels to %v", finalLabels)
		if err := getClient().updateLabels(ownerLogin, repoName, prNumber, finalLabels); err != nil {
			getLogger(ctx).Errorf("Failed to update labels: %v", err)
			ch <- err
		}
	}()
	wg.Wait()

	// Propagate a single error, or return the computed state.
	select {
	case err := <-ch:
		return "", err
	default:
		return status, nil
	}
}

func toBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func isPrMergeEvent(event event) bool {
	return toBool(event.GetPullRequest().Merged) && event.GetAction() == pullRequestActionClosed
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
