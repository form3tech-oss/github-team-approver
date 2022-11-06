package api

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

	"github.com/form3tech-oss/github-team-approver/internal/api/approval"
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
	}{
		{
			name: `PR opened (requires approval from the "CAB" team)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_opened.json"),
			eventSignature: "sha256=6d4d96d879720606802102a5892b51634c25d52f7827d2d9d0113cef17709c0e",
			pacts: []pacttesting.Pact{
				"pull_request_opened_pending",
			},

			expectedFinalStatus: approval.StatusEventStatusPending,
		},
		{
			name: `PR opened (no rules for branch)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_opened_no_rules_for_branch.json"),
			eventSignature: "sha256=f91b8ed784708050a1c332b07376a014a1f1e4ef1f94fe37d9532a37417c5bf6",
			pacts:          []pacttesting.Pact{"pull_request_opened_no_rules_for_branch"},

			expectedFinalStatus: approval.StatusEventStatusSuccess,
		},

		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha256=802a01c378001fbbbea8f59e7d5eab688550bcbd097491abc907d8850cef6e17",
			pacts: []pacttesting.Pact{
				"pull_request_review_submitted_approved",
			},

			expectedFinalStatus: approval.StatusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha256=802a01c378001fbbbea8f59e7d5eab688550bcbd097491abc907d8850cef6e17",
			pacts: []pacttesting.Pact{
				"pull_request_review_submitted_pending",
			},

			expectedFinalStatus: approval.StatusEventStatusPending,
		},

		{
			name: `PR review Submitted (requires approval from the "CAB" and "Documentation" teams)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted_force_approval.json"),
			eventSignature: "sha256=c4d9e2a311de0322c4b7c09c1a2239d23668542c9caf187be03c7acb62f3ca5b",
			pacts: []pacttesting.Pact{
				"pull_request_review_submitted_force_approval",
			},

			expectedFinalStatus: approval.StatusEventStatusSuccess,
		},
		{
			name: `PR review Submitted (no regular expressions matched)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted_no_regexes_matched.json"),
			eventSignature: "sha256=9b5e234c6deff549b631d7e08363e9e90e0bdf635e3a440e2b40cef5fab3205a",
			pacts: []pacttesting.Pact{
				"pull_request_review_submitted_no_regexes_matched",
			},

			expectedFinalStatus: approval.StatusEventStatusPending,
		},
		{
			name: `PR review Submitted (requires approval from at least one of the "CAB - Foo" and "CAB - BAR" teams, as well as from the "CAB - Documentation" team)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha256=802a01c378001fbbbea8f59e7d5eab688550bcbd097491abc907d8850cef6e17",
			pacts: []pacttesting.Pact{
				"pull_request_review_submitted_approval_mode_require_any",
			},

			expectedFinalStatus: approval.StatusEventStatusSuccess,
		},
		{
			name: `PR Merged to master (matches slack alert - alert should fire)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_merged_to_master.json"),
			eventSignature: "sha256=2324407137f738fc9e5e335e5ed6d52ab5d8a8b33705937d04463d7b9c678fcd",
			pacts: []pacttesting.Pact{
				"pull_request_merged_single_alert",
				"slack_post_message_for_emergency_change",
			},
		},
		{
			name: `PR closed (matches slack alert - alert should not fire)`,

			eventType:      eventTypePullRequest,
			eventBody:      readGitHubExampleFile("pull_request_closed.json"),
			eventSignature: "sha256=5d681e510b19e1a5e3588839f541615eded29c0c955cd795efcc56450dbad8c2",
			pacts:          []pacttesting.Pact{},
		},

		// below are good
		{
			name: `PR review Submitted (requires approval from CAB - FOO, a member of CAB - FOO contributed to the PR thus PR review isn't accepted)`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha256=802a01c378001fbbbea8f59e7d5eab688550bcbd097491abc907d8850cef6e17",
			pacts: []pacttesting.Pact{
				"pull_request_commits_alice_contributed",
				"pull_request_get_comments_pr_7",
				"pull_request_post_comment_pr_7",
				"pull_request_review_submitted_alice_approved",
			},

			expectedFinalStatus: approval.StatusEventStatusPending,
		},
		{
			name: `PR review Submitted (requires approval from CAB - FOO, Alice and Bob are members of CAB - FOO, Alice is a contributor to PR, her review is ignored. Bob's review is accepted.')`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha256=802a01c378001fbbbea8f59e7d5eab688550bcbd097491abc907d8850cef6e17",
			pacts: []pacttesting.Pact{
				"pull_request_commits_alice_contributed",
				"pull_request_review_submitted_alice_bob_approved",
			},

			expectedFinalStatus: approval.StatusEventStatusSuccess,
		},

		{
			name: `PR review Submitted (requires approval from CAB - FOO, Alice and Bob are members of CAB - FOO, Alice is a coauthor of a commit in PR, her review is ignored. Bob's review is accepted.')`,

			eventType:      eventTypePullRequestReview,
			eventBody:      readGitHubExampleFile("pull_request_review_submitted.json"),
			eventSignature: "sha256=802a01c378001fbbbea8f59e7d5eab688550bcbd097491abc907d8850cef6e17",
			pacts: []pacttesting.Pact{
				"pull_request_commits_alice_coauthor",
				"pull_request_review_submitted_alice_bob_approved",
			},

			expectedFinalStatus: approval.StatusEventStatusSuccess,
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
						if err := os.Setenv("GITHUB_BASE_URL", url); err != nil {
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

					// Call the handler and make sure the response matches our expectations.
					req := buildRequest(tt.eventType, tt.eventBody, tt.eventSignature)
					res := httptest.NewRecorder()

					// Act
					// we keep the legacy as is while providing new test structure
					api := newApi()
					api.init()
					api.Handle(res, req)

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
