package api

import (
	"fmt"
	"github.com/google/go-github/v28/github"
)
const (
	pullRequestActionClosed      = "closed"
)

type event interface {
	GetAction() string
	GetPullRequest() *github.PullRequest
	GetRepo() *github.Repository
}

func isPrMergeEvent(event event) bool {
	return event.GetPullRequest().GetMerged() && event.GetAction() == pullRequestActionClosed
}

func getSupportedEvent(eventType string) (event, error) {
	if eventType == eventTypePullRequest {
		return &github.PullRequestEvent{}, nil
	}

	if eventType == eventTypePullRequestReview {
		return &github.PullRequestReviewEvent{}, nil
	}

	return nil, fmt.Errorf("%s: %w", eventType, errIgnoredEvent)
}