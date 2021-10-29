package fakegithub

import (
	"bytes"
	"fmt"
	"github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"
	"github.com/google/go-github/v28/github"
	"github.com/stretchr/testify/require"
	"testing"
)

// ReviewEvent Capturing the data we require in sending an event
type ReviewEvent struct {
	OwnerLogin     string
	RepoName       string
	PRNumber       int
	Action         string
	CommitSHA      string
	LabelNames     []string
	PRMerged       bool
	PRTargetBranch string

	// GitHubTeam Approver template filled by author
	PRCfg  *configuration.Configuration
}

func (r *ReviewEvent) Create(t *testing.T) *github.PullRequestReviewEvent {
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

func cfgString(t *testing.T, cfg *configuration.Configuration) string {
	buffer := bytes.Buffer{}

	err := cfg.Write(&buffer)
	require.NoError(t, err, "cfg.Write")

	return buffer.String()
}

