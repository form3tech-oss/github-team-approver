package internal

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/form3tech-oss/go-pact-testing/pacttesting"
	"github.com/google/tcpproxy"
	"github.com/spf13/viper"
)

const (
	stablePactHostPort  = "localhost:18080"
	stableSlackHostPort = "localhost:18081"
)

var (
	proxyOnce  sync.Once
	proxySlack sync.Once
)

func Test_Handle(t *testing.T) {
	PactTest(t)
	tests := []struct {
		name string

		eventType      string
		eventBody      []byte
		eventSignature string
		pacts          []pacttesting.Pact

		expectedFinalStatus string

		slackAlerts []struct {
			webhookSecret string
			hookId        string
		}
	}{
		{
			name: `PR opened (requires approval from the "CAB" team)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_opened.json"),
			eventSignature: "sha1=f3a30cf3d5f785b779163dd04a20f87f9bce8aef",
			pacts:          []pacttesting.Pact{"pull_request_opened_pending"},

			expectedFinalStatus: statusEventStatusPending,
		},
		{
			name: `PR opened (no rules for branch)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_opened_no_rules_for_branch.json"),
			eventSignature: "sha1=668a5b79988a958c5535bc7f484384f956a71799",
			pacts:          []pacttesting.Pact{"pull_request_opened_no_rules_for_branch"},

			expectedFinalStatus: statusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha1=19206052dc16ae2f9a6c82df5d28fbc3b1eed0cd",
			pacts:          []pacttesting.Pact{"pull_request_review_submitted_approved"},

			expectedFinalStatus: statusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha1=19206052dc16ae2f9a6c82df5d28fbc3b1eed0cd",
			pacts:          []pacttesting.Pact{"pull_request_review_submitted_pending"},

			expectedFinalStatus: statusEventStatusPending,
		},
		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted_force_approval.json"),
			eventSignature: "sha1=c3850ad259e927948f20804f0128e692ae598a5a",
			pacts:          []pacttesting.Pact{"pull_request_review_submitted_force_approval"},

			expectedFinalStatus: statusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (no regular expressions matched)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted_no_regexes_matched.json"),
			eventSignature: "sha1=da2609f8738084d21d7b9390c23bcd6dd67adb5b",
			pacts:          []pacttesting.Pact{"pull_request_review_submitted_no_regexes_matched"},

			expectedFinalStatus: statusEventStatusPending,
		},
		{
			name: `PR review Submitted (requires approval from at least one of the "CAB - Foo" and "CAB - BAR" teams, as well as from the "CAB - Documentation" team)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha1=19206052dc16ae2f9a6c82df5d28fbc3b1eed0cd",
			pacts:          []pacttesting.Pact{"pull_request_review_submitted_approval_mode_require_any"},

			expectedFinalStatus: statusEventStatusSuccess,
		},
		{
			name: `PR Merged to master (matches slack alert - alert should fire)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_merged_to_master.json"),
			eventSignature: "sha1=12b9d49c35c1a11673d9287cda2a5b8f2b6b1b63",
			pacts:          []pacttesting.Pact{"pull_request_merged_single_alert", "slack_post_message_for_emergency_change"},

			slackAlerts: []struct {
				webhookSecret string
				hookId        string
			}{
				{
					webhookSecret: "slack_platform_team_secret",
					hookId:        "1234",
				},
			},
		},
		{
			name: `PR closed (matches slack alert - alert should not fire)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_closed.json"),
			eventSignature: "sha1=d2b6698e162d59d7e73d75900edf22bd903af731",
			pacts:          []pacttesting.Pact{},

			slackAlerts: []struct {
				webhookSecret string
				hookId        string
			}{
				{
					webhookSecret: "slack_platform_team_secret",
					hookId:        "5678",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pacttesting.IntegrationTest(
				tt.pacts,
				func() {
					// A simple proxy is used in order to make the Pact server available on a stable, well-known "host:port" combination.
					// This is required because pacts reference this "host:port" combination.
					// In its turn, this is required because the GitHub client's "DownloadContents" function will follow URLs returned in the responses themselves (bypassing the configured base URL).
					// NOTE: It is possible to initialise the proxy only once because "url" won't change between successive test cases.
					proxyOnce.Do(func() {
						url := viper.GetString("github-api")
						idx := strings.LastIndex(url, ":")
						prx := tcpproxy.Proxy{}
						prx.AddRoute(stablePactHostPort, tcpproxy.To(url[idx:]))
						go prx.Run()
						if err := os.Setenv(envGitHubBaseURL, url); err != nil {
							t.Fatal(err)
						}
					})

					slackUrl := viper.GetString("slack")
					if slackUrl != "" {
						proxySlack.Do(func() {
							idx := strings.LastIndex(slackUrl, ":")
							prx := tcpproxy.Proxy{}
							prx.AddRoute(stableSlackHostPort, tcpproxy.To(slackUrl[idx:]))
							go prx.Run()
						})
					}

					for _, slackAlert := range tt.slackAlerts {
						slackUrl := viper.GetString("slack")
						os.Setenv(slackAlert.webhookSecret, fmt.Sprintf("%s/%s", slackUrl, slackAlert.hookId))
					}
					// Call the handler and make sure the response matches our expectations.
					req := buildRequest(tt.eventType, tt.eventBody, tt.eventSignature)
					res := httptest.NewRecorder()

					// Act
					Handle(res, req)

					// Assertions
					finalStatus := res.Result().Header.Get(httpHeaderXFinalStatus)
					if finalStatus != tt.expectedFinalStatus {
						t.Errorf("handleEvent() returned %q (expected %q)", finalStatus, tt.expectedFinalStatus)
					}
				})
		})
	}
}

func buildRequest(eventType string, eventBody []byte, eventSignature string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(eventBody))
	r.Header.Set(httpHeaderXGithubEvent, eventType)
	r.Header.Set(httpHeaderXHubSignature, eventSignature)
	return r
}

func readGitHubExampleFile(file string) []byte {
	v := os.Getenv("EXAMPLES_DIR")
	if v == "" {
		panic(fmt.Errorf("EXAMPLES_DIR must be set"))
	}
	bytes, err := ioutil.ReadFile(path.Join(v, file))
	if err != nil {
		panic(err)
	}
	return bytes
}
