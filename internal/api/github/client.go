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
	"github.com/google/go-github/v42/github"
	"github.com/gregjones/httpcache"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultGitHubOperationTimeout is the maximum duration of requests against the GitHub API.
	DefaultGitHubOperationTimeout = 15 * time.Second

	// defaultListOptionsPerPage is the number of items per page that we request by default from the GitHub API.
	defaultListOptionsPerPage = 100

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

func (c *Client) AddPRComment(ctx context.Context, ownerLogin string, repoName string, prNumber int, commentBody *string) error {
	comment := &github.PullRequestComment{
		Body: commentBody,
	}

	_, res, err := c.githubClient.PullRequests.CreateComment(ctx, ownerLogin, repoName, prNumber, comment)
	if err != nil {
		log.WithError(err).Warn("error from GitHub: ", res.StatusCode)
		return err
	}

	return nil
}

func (c *Client) GetConfiguration(ctx context.Context, ownerLogin, repoName string) (*configuration.Configuration, error) {
	// Try to download the contents of the configuration file.
	ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
	defer fn()
	r, res, err := c.githubClient.Repositories.DownloadContents(ctxTimeout, ownerLogin, repoName, configuration.ConfigurationFilePath, nil)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no file named") { // fixme: look at status code instead of body
			return nil, ErrNoConfigurationFile
		}
		return nil, fmt.Errorf("error downloading configuration: %w", err)
	}
	defer res.Body.Close()
	defer r.Close()
	// Parse the configuration file.
	v, err := configuration.ReadConfiguration(r)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c *Client) GetPullRequestReviews(ctx context.Context, ownerLogin, repoName string, prNumber int) ([]*github.PullRequestReview, error) {
	reviews := make([]*github.PullRequestReview, 0, 0)

	opts := &github.ListOptions{
		Page:    1,
		PerPage: defaultListOptionsPerPage,
	}

	logger := log.WithFields(
		log.Fields{
			"pr":       prNumber,
			"repo":     fmt.Sprintf("%s/%s", ownerLogin, repoName),
			"api":      "PullRequests.ListReviews",
			"per_page": opts.PerPage,
		})

	for {
		logger.WithFields(log.Fields{"page": opts.Page}).Tracef("requesting")

		ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
		r, res, err := c.githubClient.PullRequests.ListReviews(ctxTimeout, ownerLogin, repoName, prNumber, opts)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing pull request reviews: %w", err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing pull request reviews (status: %d): %s", res.StatusCode, readAllClose(res.Body))
		}
		fn()
		reviews = append(reviews, r...)
		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
	}
	return reviews, nil
}

// https://docs.github.com/en/rest/reference/pulls#list-pull-requests-files
func (c *Client) GetPullRequestCommitFiles(ctx context.Context, ownerLogin, repoName string, prNumber int) ([]*github.CommitFile, error) {

	commitFiles := make([]*github.CommitFile, 0, 0)

	opts := &github.ListOptions{
		Page:    1,
		PerPage: defaultListOptionsPerPage,
	}

	logger := log.WithFields(
		log.Fields{
			"pr":       prNumber,
			"repo":     fmt.Sprintf("%s/%s", ownerLogin, repoName),
			"api":      "PullRequests.ListFiles",
			"per_page": opts.PerPage,
		})

	for {
		logger.WithFields(log.Fields{"page": opts.Page}).Tracef("requesting")

		ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
		r, res, err := c.githubClient.PullRequests.ListFiles(ctxTimeout, ownerLogin, repoName, prNumber, opts)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing pull request files: %w", err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing pull request files (status: %d): %s", res.StatusCode, readAllClose(res.Body))
		}
		fn()
		commitFiles = append(commitFiles, r...)
		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
	}
	return commitFiles, nil
}

func (c *Client) GetTeams(ctx context.Context, organisation string) ([]*github.Team, error) {
	// Grab a list of all the teams in the organization.
	teams := make([]*github.Team, 0, 0)

	opts := &github.ListOptions{
		Page:    1,
		PerPage: defaultListOptionsPerPage,
	}

	logger := log.WithFields(
		log.Fields{
			"org":      organisation,
			"api":      "Teams.ListTeams",
			"per_page": opts.PerPage,
		})

	for {
		logger.WithFields(log.Fields{"page": opts.Page}).Tracef("requesting")

		ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
		t, res, err := c.githubClient.Teams.ListTeams(ctxTimeout, organisation, opts)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing teams for organisation %q: %w", organisation, err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing teams for organisation %q (status: %d): %s", organisation, res.StatusCode, readAllClose(res.Body))
		}
		fn()
		teams = append(teams, t...)
		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
	}
	return teams, nil
}

func (c *Client) GetPRCommits(ctx context.Context, owner, repo string, prNumber int) ([]*github.RepositoryCommit, error) {
	var commits []*github.RepositoryCommit
	ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
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
	ctxTimeout, cancel := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
	defer cancel()

	opts := &github.ListOptions{
		Page:    page,
		PerPage: defaultListOptionsPerPage,
	}

	log.WithFields(
		log.Fields{
			"pr":       prNumber,
			"repo":     fmt.Sprintf("%s/%s", owner, repo),
			"api":      "PullRequests.ListCommits",
			"per_page": opts.PerPage,
			"page":     opts.Page,
		}).Tracef("requesting")

	commits, resp, err := c.githubClient.PullRequests.ListCommits(
		ctxTimeout, owner, repo, prNumber, opts)
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
	users := make([]*github.User, 0, 0)

	opts := &github.TeamListTeamMembersOptions{
		ListOptions: github.ListOptions{
			Page:    1,
			PerPage: defaultListOptionsPerPage,
		},
	}

	logger := log.WithFields(
		log.Fields{
			"org":      organisation,
			"name":     name,
			"api":      "Teams.ListTeamMembers",
			"per_page": opts.PerPage,
		})

	for {
		logger.WithFields(log.Fields{"page": opts.Page}).Tracef("requesting")

		ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
		org, resorg, err := c.githubClient.Organizations.Get(ctx, organisation)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error getting an organisation %q: %w", organisation, err)
		}
		if resorg.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error getting an organisation organisation %q (status: %d): %s", organisation, resorg.StatusCode, readAllClose(resorg.Body))
		}
		fn()
		defer resorg.Body.Close()

		ctxTimeout, fn = context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
		m, resteam, err := c.githubClient.Teams.ListTeamMembersByID(ctxTimeout, org.GetID(), team.GetID(), opts)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing members for team %q in organisation %q: %w", name, organisation, err)
		}
		if resteam.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing members for team %q in organisation %q (status: %d): %s", name, organisation, resteam.StatusCode, readAllClose(resteam.Body))
		}
		fn()
		defer resteam.Body.Close()

		users = append(users, m...)
		if resteam.NextPage == 0 {
			break
		}
		opts.Page = resteam.NextPage
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
	ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
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

func (c *Client) ReportIgnoredReviews(ctx context.Context, owner, repo string, prNumber int, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}

	title := "Following reviewers have been ignored as they are also authors in the PR:\n"

	err := c.removeOldBotComments(ctx, owner, repo, prNumber, title)
	if err != nil {
		return err
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
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
		return fmt.Errorf("reportIgnoredReviews: %w", err)
	}

	return nil
}

func (c *Client) removeOldBotComments(ctx context.Context, owner, repo string, prNumber int, title string) error {
	comments, err := c.getPRComments(ctx, owner, repo, prNumber)
	if err != nil {
		return err
	}

	for _, comment := range comments {
		if strings.Contains(comment.GetBody(), title) {
			_, err = c.DeletePRComment(ctx, owner, repo, comment.GetID())
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Client) DeletePRComment(ctx context.Context, owner, repo string, commentID int64) (*github.Response, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
	defer cancel()

	resp, err := c.githubClient.Issues.DeleteComment(ctxTimeout, owner, repo, commentID)
	if err != nil {
		// we treat 404 as successful, as the comment no longer exists
		if resp.StatusCode == http.StatusNotFound {
			return resp, nil
		}

		return nil, fmt.Errorf("DeletePRComment: %w", err)
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
	ctxTimeout, cancel := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
	defer cancel()

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: defaultListOptionsPerPage,
		},
	}

	log.WithFields(
		log.Fields{
			"pr":       prNumber,
			"repo":     fmt.Sprintf("%s/%s", owner, repo),
			"api":      "Issues.ListComments",
			"per_page": opts.PerPage,
			"page":     opts.Page,
		}).Tracef("requesting")

	comments, resp, err := c.githubClient.Issues.ListComments(ctxTimeout, owner, repo, prNumber, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("getPRComments: %w", err)
	}

	return comments, resp.NextPage, nil
}

func (c *Client) RequestReviews(ctx context.Context, ownerLogin, repoName string, prNumber int, reviewsToRequest []string) error {
	if len(reviewsToRequest) == 0 {
		return nil
	}
	ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
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
	ctx, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
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

	labels := make([]string, 0, 0)
	opts := &github.ListOptions{
		Page:    1,
		PerPage: defaultListOptionsPerPage,
	}

	logger := log.WithFields(
		log.Fields{
			"pr":       prNumber,
			"repo":     fmt.Sprintf("%s/%s", ownerLogin, repoName),
			"api":      "Issues.ListLabelsByIssue",
			"per_page": opts.PerPage,
		})

	for {
		logger.WithFields(log.Fields{"page": opts.Page}).Tracef("requesting")

		ctxTimeout, fn := context.WithTimeout(ctx, DefaultGitHubOperationTimeout)
		m, res, err := c.githubClient.Issues.ListLabelsByIssue(ctxTimeout, ownerLogin, repoName, prNumber, opts)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing PR labels : %w", err)
		}
		if res.StatusCode >= 300 {
			fn()
			return nil, fmt.Errorf("error listing PR labels (status: %d): %s", res.StatusCode, readAllClose(res.Body))
		}
		fn()

		labels = append(labels, githubLabelsToLabels(m)...)
		if res.NextPage == 0 {
			break
		}
		opts.Page = res.NextPage
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
