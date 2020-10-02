package internal

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
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
)

var (
	clientInstance *client
	once           sync.Once
)

type client struct {
	githubClient *github.Client
}

func (c *client) getConfiguration(ownerLogin, repoName string) (*configuration.Configuration, error) {
	// Try to download the contents of the configuration file.
	ctx, fn := context.WithTimeout(context.Background(), defaultGitHubOperationTimeout)
	defer fn()
	r, err := c.githubClient.Repositories.DownloadContents(ctx, ownerLogin, repoName, configuration.ConfigurationFilePath, nil)
	if err != nil {
		if strings.Contains(err.Error(), "No file named") { // No better way of distinguishing between errors.
			return nil, errNoConfigurationFile
		}
		return nil, fmt.Errorf("error downloading configuration: %v", err)
	}
	defer r.Close()
	// Parse the configuration file.
	v, err := configuration.ReadConfiguration(r)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c *client) getPullRequestReviews(ownerLogin, repoName string, prNumber int) ([]*github.PullRequestReview, error) {
	reviews, nextPage := make([]*github.PullRequestReview, 0, 0), 1
	for nextPage != 0 {
		ctx, fn := context.WithTimeout(context.Background(), defaultGitHubOperationTimeout)
		r, res, err := c.githubClient.PullRequests.ListReviews(ctx, ownerLogin, repoName, prNumber, &github.ListOptions{
			Page:    nextPage,
			PerPage: defaultListOptionsPerPage,
		})
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing pull request reviews: %v", err)
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

func (c *client) getTeams(organisation string) ([]*github.Team, error) {
	// Grab a list of all the teams in the organization.
	teams, nextPage := make([]*github.Team, 0, 0), 1
	for nextPage != 0 {
		ctx, fn := context.WithTimeout(context.Background(), defaultGitHubOperationTimeout)
		t, res, err := c.githubClient.Teams.ListTeams(ctx, organisation, &github.ListOptions{
			Page:    nextPage,
			PerPage: defaultListOptionsPerPage,
		})
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing teams for organisation %q: %v", organisation, err)
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

func (c *client) getTeamMembers(teams []*github.Team, organisation, name string) ([]*github.User, error) {
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
		ctx, fn := context.WithTimeout(context.Background(), defaultGitHubOperationTimeout)
		m, res, err := c.githubClient.Teams.ListTeamMembers(ctx, team.GetID(), nil)
		if err != nil {
			fn()
			return nil, fmt.Errorf("error listing members for team %q in organisation %q: %v", name, organisation, err)
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

func (c *client) reportStatus(ownerLogin, repoName, statusesURL, status, description string) error {
	n := os.Getenv(envGitHubStatusName)
	v := &github.RepoStatus{
		State:       &status,
		Description: &description,
		Context:     &n,
	}
	ctx, fn := context.WithTimeout(context.Background(), defaultGitHubOperationTimeout)
	defer fn()
	_, res, err := c.githubClient.Repositories.CreateStatus(ctx, ownerLogin, repoName, readStatusSHAFromStatusURL(statusesURL), v)
	if err != nil {
		return fmt.Errorf("error reporting status: %v", err)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("error reporting status (status: %d): %s", res.StatusCode, readAllClose(res.Body))
	}
	return nil
}

func (c *client) requestReviews(ownerLogin, repoName string, prNumber int, reviewsToRequest []string) error {
	if len(reviewsToRequest) == 0 {
		return nil
	}
	ctx, fn := context.WithTimeout(context.Background(), defaultGitHubOperationTimeout)
	defer fn()
	_, res, err := c.githubClient.PullRequests.RequestReviewers(ctx, ownerLogin, repoName, prNumber, github.ReviewersRequest{
		TeamReviewers: reviewsToRequest,
	})
	if err != nil {
		return fmt.Errorf("error requesting reviews: %v", err)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("error requesting reviews (status: %d): %s", res.StatusCode, readAllClose(res.Body))
	}
	return nil
}

func (c *client) updateLabels(ownerLogin, repoName string, prNumber int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	ctx, fn := context.WithTimeout(context.Background(), defaultGitHubOperationTimeout)
	defer fn()
	_, res, err := c.githubClient.Issues.ReplaceLabelsForIssue(ctx, ownerLogin, repoName, prNumber, labels)
	if err != nil {
		return fmt.Errorf("error updating labels: %v", err)
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("error updating labels (status: %d): %s", res.StatusCode, readAllClose(res.Body))
	}
	return nil
}

func alwaysStale(_ http.Request, _ http.Response) httpcache.Freshness {
	return httpcache.Stale
}

func getClient() *client {
	once.Do(func() {
		var (
			baseTransport http.RoundTripper
		)
		if v, err := strconv.ParseBool(os.Getenv(envUseCachingTransport)); err == nil && v {
			t := httpcache.NewMemoryCacheTransport()
			t.FreshnessFunc = alwaysStale
			baseTransport = t
		} else {
			baseTransport = http.DefaultTransport
		}
		clientInstance = &client{
			githubClient: github.NewClient(&http.Client{
				Transport: maybeWrapInAuthenticatingTransport(baseTransport),
			}),
		}
		if v := os.Getenv(envGitHubBaseURL); v != "" {
			clientInstance.githubClient.BaseURL = mustParseURL(v)
		}
	})
	return clientInstance
}

func maybeWrapInAuthenticatingTransport(baseTransport http.RoundTripper) http.RoundTripper {
	// Grab our GitHub application ID.
	applicationId, err := strconv.Atoi(os.Getenv(envGitHubAppId))
	if err != nil {
		log.Warnf("failed to parse application id: %v", err)
		return baseTransport
	}
	// Grab our GitHub installation ID.
	installationId, err := strconv.Atoi(os.Getenv(envGitHubAppInstallationId))
	if err != nil {
		log.Warnf("failed to parse installation id: %v", err)
		return baseTransport
	}
	// Use a transport that authenticates us as an installation.

	// Read the Secret Key
	fmt.Printf("%v", secretStore)
	privateKey, err := secretStore.Get(envGitHubAppPrivateKeyPath)
	if err != nil {
		log.Warnf("Failed to read secret key from configured path: %v", err)
	}
	authenticatingTransport, err := ghinstallation.New(baseTransport, int64(applicationId), int64(installationId), privateKey)
	if err != nil {
		log.Warnf("failed to create authenticating transport: %v", err)
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
		log.Fatalf("Failed to parse %q as a url: %v", v, err)
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
		log.Warnf("Failed to read from ReadCloser: %v", err)
		return []byte{}
	}
	if err := r.Close(); err != nil {
		log.Warnf("Failed to close ReadCloser: %v", err)
		return v
	}
	return v
}

func readStatusSHAFromStatusURL(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}
