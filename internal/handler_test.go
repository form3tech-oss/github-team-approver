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
	stablePactHostPort = "localhost:18080"
)

var (
	proxyOnce sync.Once
)

func Test_Handle(t *testing.T) {
	PactTest(t)
	tests := []struct {
		name string

		eventType      string
		eventBody      []byte
		eventSignature string
		pactFileName   string

		expectedFinalStatus string

		slackAlerts []struct {
			webhookSecret string
			expectedText string
			hookId string
		}
	}{
		{
			name: `PR opened (requires approval from the "CAB" team)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_opened.json"),
			eventSignature: "sha1=f3a30cf3d5f785b779163dd04a20f87f9bce8aef",
			pactFileName:   "pull_request_opened_pending.json",

			expectedFinalStatus: statusEventStatusPending,
		},
		{
			name: `PR opened (no rules for branch)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_opened_no_rules_for_branch.json"),
			eventSignature: "sha1=668a5b79988a958c5535bc7f484384f956a71799",
			pactFileName:   "pull_request_opened_no_rules_for_branch.json",

			expectedFinalStatus: statusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha1=19206052dc16ae2f9a6c82df5d28fbc3b1eed0cd",
			pactFileName:   "pull_request_review_submitted_approved.json",

			expectedFinalStatus: statusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha1=19206052dc16ae2f9a6c82df5d28fbc3b1eed0cd",
			pactFileName:   "pull_request_review_submitted_pending.json",

			expectedFinalStatus: statusEventStatusPending,
		},
		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted_force_approval.json"),
			eventSignature: "sha1=c3850ad259e927948f20804f0128e692ae598a5a",
			pactFileName:   "pull_request_review_submitted_force_approval.json",

			expectedFinalStatus: statusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (no regular expressions matched)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted_no_regexes_matched.json"),
			eventSignature: "sha1=da2609f8738084d21d7b9390c23bcd6dd67adb5b",
			pactFileName:   "pull_request_review_submitted_no_regexes_matched.json",

			expectedFinalStatus: statusEventStatusPending,
		},
		{
			name: `PR review Submitted (requires approval from at least one of the "CAB - Foo" and "CAB - BAR" teams, as well as from the "CAB - Documentation" team)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha1=19206052dc16ae2f9a6c82df5d28fbc3b1eed0cd",
			pactFileName:   "pull_request_review_submitted_approval_mode_require_any.json",

			expectedFinalStatus: statusEventStatusSuccess,
		},

		{
			name: `PR review Merged to master (matches slack alert - alert should fire)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_merged_to_master.json"),
			eventSignature: "sha1=12b9d49c35c1a11673d9287cda2a5b8f2b6b1b63",
			pactFileName:   "pull_request_merged_single_alert.json",

			slackAlerts: []struct {
				webhookSecret string
				expectedText  string
				hookId        string
			}{
				{
					webhookSecret: "slack_platform_team_secret",
					expectedText:  "emergency change merged https://github.com/form3tech/github-team-approver-test/pull/86",
					hookId:        "1234",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pacttesting.IntegrationTest([]pacttesting.Pact{
				tt.pactFileName,
			}, func() {
				var fakeSlackServer *FakeServer
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

					var err error
					if fakeSlackServer, err = NewFakeSlackServer(); err != nil {
						t.Fatal(err)
					}
					if err := fakeSlackServer.Start(); err != nil {
						t.Fatal(err)
					}
				})

				for _, slackAlert := range tt.slackAlerts {
					os.Setenv(slackAlert.webhookSecret, fakeSlackServer.AddHookEndpoint(slackAlert.hookId))
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
				for _, slackAlert := range tt.slackAlerts {
					webhook := fakeSlackServer.GetHookRequest(slackAlert.hookId)
					if slackAlert.expectedText != webhook.Text {
						t.Errorf("slack message not equal, expected: %s got: %s", slackAlert.expectedText, webhook.Text)
					}
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


