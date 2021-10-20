package github

import (
	"context"
	"errors"
	"fmt"
	"github.com/form3tech-oss/github-team-approver/internal/api/secret"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"
	"github.com/google/go-github/v28/github"
	"github.com/gregjones/httpcache"
	log "github.com/sirupsen/logrus"
)

const (
	// defaultListOptionsPerPage is the number of items per page that we request by default from the GitHub API.
	defaultListOptionsPerPage = 100
	// defaultGitHubOperationTimeout is the maximum duration of requests against the GitHub API.
	defaultGitHubOperationTimeout = 15 * time.Second

	envGitHubStatusName        = "GITHUB_STATUS_NAME"
	envUseCachingTransport     = "USE_CACHING_TRANSPORT"
	envGitHubBaseURL           = "GITHUB_BASE_URL"
	envGitHubAppId             = "GITHUB_APP_ID"
	envGitHubAppInstallationId = "GITHUB_APP_INSTALLATION_ID"
	envGitHubAppPrivateKeyPath = "GITHUB_APP_PRIVATE_KEY_PATH"
)

var (
	ErrNoConfigurationFile = errors.New("no configuration file exists in the source repository")
)

type Client struct {
	githubClient *github.Client
}

func (c *Client) GetConfiguration(ctx context.Context, ownerLogin, repoName string) (*configuration.Configuration, error) {
	// Try to download the contents of the configuration file.
	ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer fn()
	r, err := c.githubClient.Repositories.DownloadContents(ctxTimeout, ownerLogin, repoName, configuration.ConfigurationFilePath, nil)
	if err != nil {
		if strings.Contains(err.Error(), "No file named") { // No better way of distinguishing between errors.
			return nil, ErrNoConfigurationFile
		}
		return nil, fmt.Errorf("error downloading configuration: %w", err)
	}
	defer r.Close()
	// Parse the configuration file.
	v, err := configuration.ReadConfiguration(r)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c *Client) GetPullRequestReviews(ctx context.Context, ownerLogin, repoName string, prNumber int) ([]*github.PullRequestReview, error) {
	reviews, nextPage := make([]*github.PullRequestReview, 0, 0), 1
	for nextPage != 0 {
		ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
		r, res, err := c.githubClient.PullRequests.ListReviews(ctxTimeout, ownerLogin, repoName, prNumber, &github.ListOptions{
			Page:    nextPage,
			PerPage: defaultListOptionsPerPage,
		})
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing pull request reviews: %w", err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing pull request reviews (status: %d): %s", res.StatusCode, readAllClose(res.Body))
		}
		fn()
		reviews, nextPage = append(reviews, r...), res.NextPage
	}
	return reviews, nil
}

// https://docs.github.com/en/rest/reference/pulls#list-pull-requests-files
func (c *Client) GetPullRequestCommitFiles(ctx context.Context, ownerLogin, repoName string, prNumber int) ([]*github.CommitFile, error) {

	commitFiles, nextPage := make([]*github.CommitFile, 0, 0), 1
	for nextPage != 0 {
		ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
		r, res, err := c.githubClient.PullRequests.ListFiles(ctxTimeout, ownerLogin, repoName, prNumber, &github.ListOptions{
			Page:    nextPage,
			PerPage: defaultListOptionsPerPage,
		})
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing pull request files: %w", err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing pull request files (status: %d): %s", res.StatusCode, readAllClose(res.Body))
		}
		fn()
		commitFiles, nextPage = append(commitFiles, r...), res.NextPage
	}
	return commitFiles, nil
}

func (c *Client) GetTeams(ctx context.Context, organisation string) ([]*github.Team, error) {
	// Grab a list of all the teams in the organization.
	teams, nextPage := make([]*github.Team, 0, 0), 1
	for nextPage != 0 {
		ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
		t, res, err := c.githubClient.Teams.ListTeams(ctxTimeout, organisation, &github.ListOptions{
			Page:    nextPage,
			PerPage: defaultListOptionsPerPage,
		})
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing teams for organisation %q: %w", organisation, err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing teams for organisation %q (status: %d): %s", organisation, res.StatusCode, readAllClose(res.Body))
		}
		fn()
		teams, nextPage = append(teams, t...), res.NextPage
	}
	return teams, nil
}

func (c *Client) GetPRCommits(ctx context.Context, owner, repo string, prNumber int) ([]*github.RepositoryCommit, error) {
	var commits []*github.RepositoryCommit
	ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer fn()

	nextPage := 1
	for nextPage != 0 {
		commitsPage, next, err := c.getPRCommitsPage(ctxTimeout, owner, repo, prNumber, nextPage)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commitsPage...)
		nextPage = next
	}

	return commits, nil
}

func (c *Client) getPRCommitsPage(ctx context.Context, owner, repo string, prNumber, page int) ([]*github.RepositoryCommit, int, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer cancel()

	commits, resp, err := c.githubClient.PullRequests.ListCommits(
		ctxTimeout, owner, repo, prNumber, &github.ListOptions{Page: page})
	if err != nil {
		return nil, 0, fmt.Errorf("getPRCommits: %w", err)
	}

	return commits, resp.NextPage, nil
}

func (c *Client) GetTeamMembers(ctx context.Context, teams []*github.Team, organisation, name string) ([]*github.User, error) {
	// Return immediately if the request team is not found.
	var (
		team *github.Team
	)
	for _, v := range teams {
		if v.GetName() == name {
			team = v
			break
		}
	}
	if team == nil {
		return nil, fmt.Errorf("could not find team %q in organisation %q", name, organisation)
	}
	// Grab a list of all the users in the target team.
	users, nextPage := make([]*github.User, 0, 0), 1
	for nextPage != 0 {
		ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
		m, res, err := c.githubClient.Teams.ListTeamMembers(ctxTimeout, team.GetID(), nil)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing members for team %q in organisation %q: %w", name, organisation, err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing members for team %q in organisation %q (status: %d): %s", name, organisation, res.StatusCode, readAllClose(res.Body))
		}
		fn()
		users, nextPage = append(users, m...), res.NextPage
	}
	return users, nil
}

func (c *Client) ReportStatus(ctx context.Context, ownerLogin, repoName, statusesURL, status, description string) error {
	n := os.Getenv(envGitHubStatusName)
	v := &github.RepoStatus{
		State:       &status,
		Description: &description,
		Context:     &n,
	}
	ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer fn()
	_, res, err := c.githubClient.Repositories.CreateStatus(ctxTimeout, ownerLogin, repoName, readStatusSHAFromStatusURL(statusesURL), v)
	if err != nil {
		return fmt.Errorf("error reporting status: %w", err)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("error reporting status (status: %d): %s", res.StatusCode, readAllClose(res.Body))
	}
	return nil
}

func (c *Client) ReportDismissedReviews(ctx context.Context, owner, repo string, prNumber int, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}

	title := "Following reviewers have been dismissed as they are also authors in the PR:\n"

	err := c.removeOldDismissedComments(ctx, owner, repo, prNumber, title)
	if err != nil {
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer cancel()

	msg := title
	for _, r := range reviewers {
		msg += fmt.Sprintf("- @%s\n", r)
	}

	payload := &github.IssueComment{
		Body: github.String(msg),
	}

	// using Issues API over PullRequests as we only have an interest in commenting on the PR
	// not commenting on a given line in a specific commit
	_, _, err = c.githubClient.Issues.CreateComment(ctxTimeout, owner, repo, prNumber, payload)
	if err != nil {
		return fmt.Errorf("reportDismissedReviews: %w", err)
	}

	return nil
}

func (c *Client) removeOldDismissedComments(ctx context.Context, owner, repo string, prNumber int, title string) error {
	comments, err := c.getPRComments(ctx, owner, repo, prNumber)
	if err != nil {
		return err
	}

	for _, comment := range comments {
		if strings.Contains(comment.GetBody(), title) {
			_, err = c.deletePRComment(ctx, owner, repo, comment.GetID())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) deletePRComment(ctx context.Context, owner, repo string, commentID int64) (*github.Response, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer cancel()

	resp, err := c.githubClient.Issues.DeleteComment(ctxTimeout, owner, repo, commentID)
	if err != nil {
		return nil, fmt.Errorf("deletePRComment: %w", err)
	}

	return resp, nil
}

func (c *Client) getPRComments(ctx context.Context, owner, repo string, prNumber int) ([]*github.IssueComment, error) {
	var comments []*github.IssueComment

	nextPage := 1
	for nextPage != 0 {
		commentsPage, next, err := c.getPRCommentsPage(ctx, owner, repo, prNumber, nextPage)
		if err != nil {
			return nil, err
		}
		comments = append(comments, commentsPage...)
		nextPage = next
	}

	return comments, nil
}

func (c *Client) getPRCommentsPage(ctx context.Context, owner, repo string, prNumber, page int) ([]*github.IssueComment, int, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer cancel()

	listOpts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: defaultListOptionsPerPage,
		},
	}

	comments, resp, err := c.githubClient.Issues.ListComments(ctxTimeout, owner, repo, prNumber, listOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("getPRComments: %w", err)
	}

	return comments, resp.NextPage, nil
}

func (c *Client) RequestReviews(ctx context.Context, ownerLogin, repoName string, prNumber int, reviewsToRequest []string) error {
	if len(reviewsToRequest) == 0 {
		return nil
	}
	ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer fn()
	_, res, err := c.githubClient.PullRequests.RequestReviewers(ctxTimeout, ownerLogin, repoName, prNumber, github.ReviewersRequest{
		TeamReviewers: reviewsToRequest,
	})
	if err != nil {
		return fmt.Errorf("error requesting reviews: %w", err)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("error requesting reviews (status: %d): %s", res.StatusCode, readAllClose(res.Body))
	}
	return nil
}

func (c *Client) UpdateLabels(ctx context.Context, ownerLogin, repoName string, prNumber int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	ctx, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
	defer fn()
	_, res, err := c.githubClient.Issues.ReplaceLabelsForIssue(ctx, ownerLogin, repoName, prNumber, labels)
	if err != nil {
		return fmt.Errorf("error updating labels: %w", err)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("error updating labels (status: %d): %s", res.StatusCode, readAllClose(res.Body))
	}
	return nil
}

func (c *Client) GetLabels(ctx context.Context, ownerLogin, repoName string, prNumber int) ([]string, error) {

	labels, nextPage := make([]string, 0, 0), 1
	for nextPage != 0 {
		ctxTimeout, fn := context.WithTimeout(ctx, defaultGitHubOperationTimeout)
		m, res, err := c.githubClient.Issues.ListLabelsByIssue(ctxTimeout, ownerLogin, repoName, prNumber, nil)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing PR labels : %w", err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing PR labels (status: %d): %s", res.StatusCode, readAllClose(res.Body))
		}
		fn()

		labels, nextPage = append(labels, githubLabelsToLabels(m)...), res.NextPage
	}
	return labels, nil
}

func githubLabelsToLabels(githubLabels []*github.Label) []string {

	var out []string
	for _, githubLabel := range githubLabels {
		if githubLabel == nil || githubLabel.Name == nil {
			continue
		}
		out = append(out, *githubLabel.Name)
	}
	return out
}

func alwaysStale(_ http.Request, _ http.Response) httpcache.Freshness {
	return httpcache.Stale
}

func New(store secret.Store) *Client {
	var baseTransport http.RoundTripper

	if v, err := strconv.ParseBool(os.Getenv(envUseCachingTransport)); err == nil && v {
		t := httpcache.NewMemoryCacheTransport()
		t.FreshnessFunc = alwaysStale
		baseTransport = t
	} else {
		baseTransport = http.DefaultTransport
	}
	client := &Client{
		githubClient: github.NewClient(&http.Client{
			Transport: maybeWrapInAuthenticatingTransport(baseTransport, store),
		}),
	}
	if v := os.Getenv(envGitHubBaseURL); v != "" {
		client.githubClient.BaseURL = mustParseURL(v)
	}
	return client
}

func maybeWrapInAuthenticatingTransport(baseTransport http.RoundTripper, store secret.Store) http.RoundTripper {
	// Grab our GitHub application ID.
	applicationId, err := strconv.Atoi(os.Getenv(envGitHubAppId))
	if err != nil {
		log.WithError(err).Warn("failed to parse application id")
		return baseTransport
	}
	// Grab our GitHub installation ID.
	installationId, err := strconv.Atoi(os.Getenv(envGitHubAppInstallationId))
	if err != nil {
		log.WithError(err).Warn("failed to parse installation id")
		return baseTransport
	}
	// Use a transport that authenticates us as an installation.

	privateKey, err := store.Get(envGitHubAppPrivateKeyPath)
	if err != nil {
		log.WithError(err).Warn("Failed to read secret key from configured path")
	}
	authenticatingTransport, err := ghinstallation.New(baseTransport, int64(applicationId), int64(installationId), privateKey)
	if err != nil {
		log.WithError(err).Warn("failed to create authenticating transport")
		return baseTransport
	}
	return authenticatingTransport
}

func mustParseURL(v string) *url.URL {
	if !strings.HasSuffix(v, "/") {
		v += "/"
	}
	u, err := url.Parse(v)
	if err != nil {
		log.WithError(err).Fatalf("Failed to parse %q as a url", v)
	}
	return u
}

func readAllClose(r io.ReadCloser) []byte {
	if r == nil {
		log.Warn("Failed to read from nil ReadCloser")
		return []byte{}
	}
	v, err := ioutil.ReadAll(r)
	if err != nil {
		log.WithError(err).Warn("Failed to read from ReadCloser")
		return []byte{}
	}
	if err := r.Close(); err != nil {
		log.WithError(err).Warn("Failed to close ReadCloser")
		return v
	}
	return v
}

func readStatusSHAFromStatusURL(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
