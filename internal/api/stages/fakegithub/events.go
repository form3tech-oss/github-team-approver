package fakegithub

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/form3tech-oss/github-team-approver-commons/v2/pkg/configuration"
	"github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/require"
)

// Event Capturing the data we require in sending an event
type Event struct {
	OwnerLogin     string
	RepoName       string
	PRNumber       int
	Action         string
	CommitSHA      string
	LabelNames     []string
	PRMerged       bool
	PRTargetBranch string

	// GitHubTeam Approver template filled by author
	PRCfg *configuration.Configuration
}

func (r *Event) CreatePullRequestReviewEvent(t *testing.T) *github.PullRequestReviewEvent {
	var labels []*github.Label
	for _, n := range r.LabelNames {
		labels = append(labels, &github.Label{Name: github.String(n)})
	}
	owner := &github.User{Login: github.String(r.OwnerLogin)}
	fullName := fmt.Sprintf("%s/%s", r.OwnerLogin, r.RepoName)

	return &github.PullRequestReviewEvent{
		Repo: &github.Repository{
			Owner:    owner,
			Name:     github.String(r.RepoName),
			FullName: github.String(fullName),
		},

		Action: github.String(r.Action),
		Review: &github.PullRequestReview{},
		PullRequest: &github.PullRequest{
			Number:      github.Int(r.PRNumber),
			Body:        github.String(cfgString(t, r.PRCfg)),
			Labels:      labels,
			CommitsURL:  github.String(fmt.Sprintf("repos/%s/commits{/sha}", fullName)),
			CommentsURL: github.String(fmt.Sprintf("repos/%s/comments{/number}", fullName)),
			StatusesURL: github.String(fmt.Sprintf("repos/%s/statuses/%s", fullName, r.CommitSHA)),
			Base: &github.PullRequestBranch{
				Ref: github.String(r.PRTargetBranch),
			},
			Merged: github.Bool(r.PRMerged),
		},
	}
}

func (e *Event) CreatePullRequestEvent(t *testing.T) *github.PullRequestEvent {
	var labels []*github.Label
	for _, n := range e.LabelNames {
		labels = append(labels, &github.Label{Name: github.String(n)})
	}
	owner := &github.User{Login: github.String(e.OwnerLogin)}
	fullName := fmt.Sprintf("%s/%s", e.OwnerLogin, e.RepoName)

	return &github.PullRequestEvent{
		Repo: &github.Repository{
			Owner:    owner,
			Name:     github.String(e.RepoName),
			FullName: github.String(fullName),
		},

		Action: github.String(e.Action),
		PullRequest: &github.PullRequest{
			Number:      github.Int(e.PRNumber),
			Body:        github.String(cfgString(t, e.PRCfg)),
			Labels:      labels,
			CommitsURL:  github.String(fmt.Sprintf("repos/%s/commits{/sha}", fullName)),
			CommentsURL: github.String(fmt.Sprintf("repos/%s/comments{/number}", fullName)),
			StatusesURL: github.String(fmt.Sprintf("repos/%s/statuses/%s", fullName, e.CommitSHA)),
			Base: &github.PullRequestBranch{
				Ref: github.String(e.PRTargetBranch),
			},
			Merged: github.Bool(e.PRMerged),
		},
	}
}

func cfgString(t *testing.T, cfg *configuration.Configuration) string {
	buffer := bytes.Buffer{}

	err := cfg.Write(&buffer)
	require.NoError(t, err, "cfg.Write")

	return buffer.String()
}
