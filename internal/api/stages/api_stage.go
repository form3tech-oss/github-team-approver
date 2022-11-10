package stages

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"testing"

	approverCfg "github.com/form3tech-oss/github-team-approver-commons/v2/pkg/configuration"
	"github.com/form3tech-oss/github-team-approver/internal/api/approval"
	"github.com/form3tech-oss/github-team-approver/internal/api/stages/fakegithub"
	"github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/require"
)

const (
	tokenPath              = "testdata/token"
	botName                = "github-team-approver"
	httpHeaderXFinalStatus = "X-Final-Status"

	ignoredReviewerMsg = "Following reviewers have been ignored as they are also authors in the PR:"
)

type ApiStage struct {
	t *testing.T

	WebHookSecret []byte
	fakeGitHub    *fakegithub.FakeGitHub

	app *AppServer

	labels []string

	resp *http.Response
}

func (s *ApiStage) EncryptionKeyExists() *ApiStage {
	abs, err := filepath.Abs("testdata/key")
	require.NoError(s.t, err, "filepath.Abs: %s", err)

	s.setupEnv("ENCRYPTION_KEY_PATH", abs)

	return s
}

func (s *ApiStage) GitHubWebHookTokenExists() *ApiStage {
	abs, err := filepath.Abs(tokenPath)
	require.NoError(s.t, err, "filepath.Abs: %s", err)

	s.setupEnv("GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH", abs)

	return s
}

func (s *ApiStage) FakeGHRunning() *ApiStage {
	s.fakeGitHub = fakegithub.NewFakeGithub(s.t)
	s.setupEnv("GITHUB_BASE_URL", s.fakeGitHub.URL())
	s.setupEnv("GITHUB_STATUS_NAME", botName)

	return s
}

func (s *ApiStage) OrganisationWithTeamFoo() *ApiStage {
	teams := []*github.Team{
		{
			ID:   github.Int64(1),
			Name: github.String("CAB - Foo"),
			Slug: github.String("cab-foo"),
		},
	}

	teamMembers := fakegithub.Team{*teams[0].ID: []*github.User{
		{
			Login: github.String("alice"),
		},
		{
			Login: github.String("bob"),
		},
		{
			Login: github.String("eve"),
		},
	}}
	s.fakeGitHub.SetOrg(&fakegithub.Org{
		OwnerName:   "form3tech",
		Teams:       teams,
		TeamMembers: teamMembers,
	})

	return s
}

func (s *ApiStage) RepoWithFooAsApprovingTeam() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	approvingTeam := *s.fakeGitHub.Org().Teams[0].Name

	repo := &fakegithub.Repo{
		Name: "some-service",

		ApproverCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{"master"},
					Rules: []approverCfg.Rule{
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- \[x\] Yes - this change impacts customers`,
							ApprovingTeamHandles: []string{approvingTeam},
							Labels:               []string{},
						},
					},
				},
			},
		},
	}
	s.fakeGitHub.SetRepo(repo)
	s.fakeGitHub.SetRepoContents([]*github.RepositoryContent{
		{
			Name:        github.String("GITHUB_TEAM_APPROVER.yaml"),
			DownloadURL: github.String(fmt.Sprintf("%s/master/%s", s.fakeGitHub.RepoURL(), approverCfg.ConfigurationFilePath)),
		},
	})

	return s
}

func (s *ApiStage) RepoWithNoContributorReviewEnabledAndFooAsApprovingTeam() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	approvingTeam := *s.fakeGitHub.Org().Teams[0].Name

	repo := &fakegithub.Repo{
		Name: "some-service",

		ApproverCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{"master"},
					Rules: []approverCfg.Rule{
						{
							IgnoreContributorApproval: true,
							ApprovalMode:              approverCfg.ApprovalModeRequireAny,
							Regex:                     `- \[x\] Yes - this change impacts customers`,
							ApprovingTeamHandles:      []string{approvingTeam},
							Labels:                    []string{},
						},
					},
				},
			},
		},
	}
	s.fakeGitHub.SetRepo(repo)
	s.fakeGitHub.SetRepoContents([]*github.RepositoryContent{
		{
			Name:        github.String("GITHUB_TEAM_APPROVER.yaml"),
			DownloadURL: github.String(fmt.Sprintf("%s/master/%s", s.fakeGitHub.RepoURL(), approverCfg.ConfigurationFilePath)),
		},
	})

	return s
}

func (s *ApiStage) RepoWithFooAsApprovingTeamAndMultipleRules() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	approvingTeam := *s.fakeGitHub.Org().Teams[0].Name

	repo := &fakegithub.Repo{
		Name: "some-service",

		ApproverCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{"master"},
					Rules: []approverCfg.Rule{
						{
							IgnoreContributorApproval: true,
							Directories:               []string{"code/"},
							ApprovalMode:              approverCfg.ApprovalModeRequireAny,
							Regex:                     `- \[x\] Yes - this change impacts customers`,
							ApprovingTeamHandles:      []string{approvingTeam},
							Labels:                    []string{},
						},
						{
							IgnoreContributorApproval: false,
							ApprovalMode:              approverCfg.ApprovalModeRequireAny,
							Regex:                     `- \[x\] Yes - this change impacts customers`,
							ApprovingTeamHandles:      []string{approvingTeam},
							Labels:                    []string{},
						},
					},
				},
			},
		},
	}
	s.fakeGitHub.SetRepo(repo)
	s.fakeGitHub.SetRepoContents([]*github.RepositoryContent{
		{
			Name:        github.String("GITHUB_TEAM_APPROVER.yaml"),
			DownloadURL: github.String(fmt.Sprintf("%s/master/%s", s.fakeGitHub.RepoURL(), approverCfg.ConfigurationFilePath)),
		},
	})

	return s
}

func (s *ApiStage) RepoWithFooAsApprovingTeamWithEmergencyRule() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	approvingTeam := *s.fakeGitHub.Org().Teams[0].Name

	repo := &fakegithub.Repo{
		Name: "some-service",

		ApproverCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{"master"},
					Rules: []approverCfg.Rule{
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- \[x\] Yes - this change impacts customers`,
							ApprovingTeamHandles: []string{approvingTeam},
							Labels:               []string{},
						},
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- \[x\] Yes - Emergency`,
							ApprovingTeamHandles: []string{"CAB - Foo"},
							Labels:               []string{},
							ForceApproval:        true,
						},
					},
				},
			},
		},
	}
	s.fakeGitHub.SetRepo(repo)
	s.fakeGitHub.SetRepoContents([]*github.RepositoryContent{
		{
			Name:        github.String("GITHUB_TEAM_APPROVER.yaml"),
			DownloadURL: github.String(fmt.Sprintf("%s/master/%s", s.fakeGitHub.RepoURL(), approverCfg.ConfigurationFilePath)),
		},
	})

	return s
}

func (s *ApiStage) RepoWithoutConfigurationFile() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())

	repo := &fakegithub.Repo{
		Name: "some-service",
	}
	s.fakeGitHub.SetRepo(repo)
	s.fakeGitHub.SetRepoContents([]*github.RepositoryContent{})

	return s
}

func (s *ApiStage) RepoWithConfigurationReferencingInvalidTeamHandles() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	approvingTeam := *s.fakeGitHub.Org().Teams[0].Name

	repo := &fakegithub.Repo{
		Name: "some-service",

		ApproverCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{"master"},
					Rules: []approverCfg.Rule{
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- \[x\] Yes - this change impacts customers`,
							ApprovingTeamHandles: []string{approvingTeam},
							Labels:               []string{},
						},
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- \[x\] Yes - Emergency`,
							ApprovingTeamHandles: []string{"CRAB - Foo"},
							Labels:               []string{},
							ForceApproval:        true,
						},
					},
				},
			},
		},
	}
	s.fakeGitHub.SetRepo(repo)
	s.fakeGitHub.SetRepoContents([]*github.RepositoryContent{
		{
			Name:        github.String("GITHUB_TEAM_APPROVER.yaml"),
			DownloadURL: github.String(fmt.Sprintf("%s/master/%s", s.fakeGitHub.RepoURL(), approverCfg.ConfigurationFilePath)),
		},
	})

	return s
}

func (s *ApiStage) GitHubTeamApproverRunning() *ApiStage {
	s.t.Cleanup(s.app.Shutdown)
	s.setupEnv("LOG_LEVEL", "TRACE")

	err := s.app.Start()
	require.NoError(s.t, err, "app start")

	return s
}

func (s *ApiStage) SendingAnUnsupportedEvent() *ApiStage {
	payload := &struct{}{}

	c := newClient(s.t, s.app.URL(), s.WebHookSecret)
	s.resp = c.sendEvent(payload, "NON_EXISTING_EVENT")

	return s
}

func (s *ApiStage) SendingEventWithInvalidSignature() *ApiStage {
	c := newClient(s.t, s.app.URL(), s.WebHookSecret)
	s.resp = c.sendEventWithIncorrectSignature(&struct{}{})
	return s
}

func (s *ApiStage) StatusNoContentReturned() *ApiStage {
	require.NotNil(s.t, s.resp)
	require.Equal(s.t, http.StatusNoContent, s.resp.StatusCode)
	require.Empty(s.t, s.resp.Header.Get(httpHeaderXFinalStatus))

	return s
}
func (s *ApiStage) ExpectPendingAnswerReturned() *ApiStage {
	require.NotNil(s.t, s.resp)
	require.Equal(s.t, http.StatusOK, s.resp.StatusCode)
	require.Equal(s.t, approval.StatusEventStatusPending, s.resp.Header.Get(httpHeaderXFinalStatus))
	return s
}

func (s *ApiStage) ExpectErrorAnswerReturned() *ApiStage {
	require.NotNil(s.t, s.resp)
	require.Equal(s.t, http.StatusOK, s.resp.StatusCode)
	require.Equal(s.t, approval.StatusEventStatusError, s.resp.Header.Get(httpHeaderXFinalStatus))
	return s
}

func (s *ApiStage) ExpectSuccessAnswerReturned() *ApiStage {
	require.NotNil(s.t, s.resp)
	require.Equal(s.t, http.StatusOK, s.resp.StatusCode)
	require.Equal(s.t, approval.StatusEventStatusSuccess, s.resp.Header.Get(httpHeaderXFinalStatus))
	return s
}

func (s *ApiStage) setupEnv(k, v string) {
	s.t.Cleanup(func() {
		err := os.Unsetenv(k)
		require.NoError(s.t, err)
	})
	err := os.Setenv(k, v)
	require.NoError(s.t, err)
}

func (s *ApiStage) ExpectBadRequestReturned() {
	require.NotNil(s.t, s.resp)
	require.Equal(s.t, http.StatusBadRequest, s.resp.StatusCode)
}

func (s *ApiStage) CommitsWithBobAsContributor() *ApiStage {
	commits := []*github.RepositoryCommit{
		{
			SHA: github.String("some-sha-1"),

			Author: &github.User{
				Login: github.String("bob"),
			},
			Committer: &github.User{
				Login: github.String("bob"),
			},
		},
	}
	s.fakeGitHub.SetCommits(commits)

	return s
}

func (s *ApiStage) CommitsWithCharlieAsContributor() *ApiStage {
	commits := []*github.RepositoryCommit{
		{
			SHA: github.String("some-sha-1"),

			Author: &github.User{
				Login: github.String("charlie"),
			},
			Committer: &github.User{
				Login: github.String("charlie"),
			},
		},
	}
	s.fakeGitHub.SetCommits(commits)

	return s
}

func (s *ApiStage) CommitsWithBobAndEveAsContributor() *ApiStage {
	commits := []*github.RepositoryCommit{
		{
			SHA: github.String("some-sha-1"),

			Author: &github.User{
				Login: github.String("bob"),
			},
			Committer: &github.User{
				Login: github.String("eve"),
			},
		},
	}
	s.fakeGitHub.SetCommits(commits)

	return s
}

func (s *ApiStage) CommitsWithAliceAsContributor() *ApiStage {
	commits := []*github.RepositoryCommit{
		{
			SHA: github.String("some-sha-1"),

			Author: &github.User{
				Login: github.String("alice"),
			},
			Committer: &github.User{
				Login: github.String("alice"),
			},
		},
	}
	s.fakeGitHub.SetCommits(commits)

	return s
}

func (s *ApiStage) CommitsWithAliceAsCoAuthor() *ApiStage {
	return s.commitsWithAuthorAndCoAuthor("bob", "alice")
}

func (s *ApiStage) CommitsWithBobAsCoAuthor() *ApiStage {
	return s.commitsWithAuthorAndCoAuthor("eve", "bob")
}

func (s *ApiStage) commitsWithAuthorAndCoAuthor(author, coauthor string) *ApiStage {
	message := fmt.Sprintf("commit message\n\nCo-authored-by: %s <12345678+%s@users.noreply.github.com>", coauthor, coauthor)
	commits := []*github.RepositoryCommit{
		{
			SHA: github.String("some-sha-1"),
			Author: &github.User{
				Login: github.String(author),
			},
			Committer: &github.User{
				Login: github.String(author),
			},
			Commit: &github.Commit{Message: github.String(message)},
		},
	}
	s.fakeGitHub.SetCommits(commits)

	return s
}

func (s *ApiStage) NoCommentsExist() *ApiStage {
	s.fakeGitHub.SetIssueComments([]*github.IssueComment{})
	return s
}

func (s *ApiStage) NoReviewsExist() *ApiStage {
	s.fakeGitHub.SetReviews([]*github.PullRequestReview{})
	return s
}

func (s *ApiStage) IgnoredReviewCommentsExist() *ApiStage {
	msg := fmt.Sprintf("%s\n- @%s\n", ignoredReviewerMsg, "some user")
	comments := []*github.IssueComment{
		{
			ID:   github.Int64(1),
			Body: github.String(msg),
		},
		{
			ID:   github.Int64(2),
			Body: github.String(msg),
		},
	}
	s.fakeGitHub.SetIssueComments(comments)

	return s
}

func (s *ApiStage) AliceApprovesPullRequest() *ApiStage {
	reviews := []*github.PullRequestReview{
		{
			State: github.String("APPROVED"),
			User: &github.User{
				Login: github.String("alice"),
			},
		},
	}

	s.fakeGitHub.SetReviews(reviews)

	return s
}

func (s *ApiStage) CharlieApprovesPullRequest() *ApiStage {
	reviews := []*github.PullRequestReview{
		{
			State: github.String("APPROVED"),
			User: &github.User{
				Login: github.String("charlie"),
			},
		},
	}

	s.fakeGitHub.SetReviews(reviews)

	return s
}

func (s *ApiStage) PullRequestHasNoReviews() *ApiStage {
	var reviews []*github.PullRequestReview
	s.fakeGitHub.SetReviews(reviews)

	return s
}

func (s *ApiStage) PullRequestExists() *ApiStage {

	s.fakeGitHub.SetPR(&fakegithub.PR{
		PRNumber: 1,
		PRCommit: "some-hash",
		Files: []fakegithub.PRFile{
			fakegithub.PRFile{
				SHA:      "sha1",
				Filename: "/code/main.go",
			},
			fakegithub.PRFile{
				SHA:      "sha2",
				Filename: "/config/dev.yaml",
			},
		},
	})

	return s
}

func (s *ApiStage) SendingPREvent() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	require.NotNil(s.t, s.fakeGitHub.Repo())

	approvingTeam := *s.fakeGitHub.Org().Teams[0].Name
	targetBranch := "master"
	s.labels = []string{"foo", "bar", "needs-cab-approval"}

	r := fakegithub.Event{
		OwnerLogin: s.fakeGitHub.Org().OwnerName,
		RepoName:   s.fakeGitHub.Repo().Name,
		PRNumber:   s.fakeGitHub.PR().PRNumber,

		Action:         "opened",
		CommitSHA:      s.fakeGitHub.PR().PRCommit,
		LabelNames:     s.labels,
		PRMerged:       false,
		PRTargetBranch: targetBranch,
		PRCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{targetBranch},
					Rules: []approverCfg.Rule{
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- [x] Yes - this change impacts customers`,
							ApprovingTeamHandles: []string{approvingTeam},
							Labels:               []string{},
						},
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- [x] Yes - Emergency`,
							ApprovingTeamHandles: []string{"CRAB - Foo"},
							Labels:               []string{"needs-cab-approval"},
							ForceApproval:        true,
						},
					},
				},
			},
		},
	}
	payload := r.CreatePullRequestEvent(s.t)

	c := newClient(s.t, s.app.URL(), s.WebHookSecret)
	s.resp = c.sendEvent(payload, "pull_request")

	return s
}

func (s *ApiStage) SendingApprovedPRReviewSubmittedEvent() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	require.NotNil(s.t, s.fakeGitHub.Repo())
	require.NotNil(s.t, s.fakeGitHub.PR())

	targetBranch := "master"
	s.labels = []string{"foo", "bar"}

	r := fakegithub.Event{
		OwnerLogin: s.fakeGitHub.Org().OwnerName,
		RepoName:   s.fakeGitHub.Repo().Name,
		PRNumber:   s.fakeGitHub.PR().PRNumber,

		Action:         "submitted",
		CommitSHA:      s.fakeGitHub.PR().PRCommit,
		LabelNames:     s.labels,
		PRMerged:       false,
		PRTargetBranch: targetBranch,
		PRCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{targetBranch},
					Rules: []approverCfg.Rule{
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- [x] Yes - this change impacts customers`,
							ApprovingTeamHandles: []string{"CAB - Foo"},
							Labels:               []string{},
						},
					},
				},
			},
		},
	}
	payload := r.CreatePullRequestReviewEvent(s.t)

	c := newClient(s.t, s.app.URL(), s.WebHookSecret)
	s.resp = c.sendEvent(payload, "pull_request_review")

	return s
}

func (s *ApiStage) SendingPRReviewSubmittedEventWithForceApproval() *ApiStage {
	require.NotNil(s.t, s.fakeGitHub.Org())
	require.NotNil(s.t, s.fakeGitHub.Repo())
	require.NotNil(s.t, s.fakeGitHub.PR())

	targetBranch := "master"
	s.labels = []string{"foo", "bar", "needs-cab-approval"}

	r := fakegithub.Event{
		OwnerLogin: s.fakeGitHub.Org().OwnerName,
		RepoName:   s.fakeGitHub.Repo().Name,
		PRNumber:   s.fakeGitHub.PR().PRNumber,

		Action:         "submitted",
		CommitSHA:      s.fakeGitHub.PR().PRCommit,
		LabelNames:     s.labels,
		PRMerged:       false,
		PRTargetBranch: targetBranch,
		PRCfg: &approverCfg.Configuration{
			PullRequestApprovalRules: []approverCfg.PullRequestApprovalRule{
				{
					TargetBranches: []string{targetBranch},
					Rules: []approverCfg.Rule{
						{
							ApprovalMode:         approverCfg.ApprovalModeRequireAny,
							Regex:                `- [x] Yes - Emergency`,
							ApprovingTeamHandles: []string{"CAB - Foo"},
							Labels:               []string{"needs-cab-approval"},
							ForceApproval:        true,
						},
					},
				},
			},
		},
	}
	payload := r.CreatePullRequestReviewEvent(s.t)

	c := newClient(s.t, s.app.URL(), s.WebHookSecret)
	s.resp = c.sendEvent(payload, "pull_request_review")

	return s
}

func (s *ApiStage) ExpectLabelsUpdated() *ApiStage {
	expected := s.labels
	actual := s.fakeGitHub.ReportedLabels()
	sort.Strings(expected)
	sort.Strings(actual)

	require.Equal(s.t, expected, actual)
	return s
}

func (s *ApiStage) ExpectStatusPendingReported() *ApiStage {
	status := s.fakeGitHub.ReportedStatus()
	require.Equal(s.t, approval.StatusEventStatusPending, *(status.State))
	require.Equal(s.t, botName, *(status.Context))
	return s
}

func (s *ApiStage) ExpectStatusErrorReported() *ApiStage {
	status := s.fakeGitHub.ReportedStatus()
	require.Equal(s.t, approval.StatusEventStatusError, *(status.State))
	require.Equal(s.t, botName, *(status.Context))
	return s
}

func (s *ApiStage) ExpectStatusSuccessReported() *ApiStage {
	status := s.fakeGitHub.ReportedStatus()
	require.Equal(s.t, approval.StatusEventStatusSuccess, *(status.State))
	require.Equal(s.t, botName, *(status.Context))
	return s
}

func (s *ApiStage) ExpectInvalidTeamHandleInStatusDescription() *ApiStage {
	status := s.fakeGitHub.ReportedStatus()
	require.Regexp(s.t, ".*\\nCRAB - Foo", *(status.Description))
	return s
}

func (s *ApiStage) ExpectedReviewRequestsMadeForFoo() *ApiStage {
	reviews := s.fakeGitHub.RequestedTeamReviews()
	require.Len(s.t, reviews, 1)
	require.Equal(s.t, *s.fakeGitHub.Org().Teams[0].Slug, reviews[0])
	return s
}

func (s *ApiStage) ExpectCommentAliceIgnoredAsReviewer() *ApiStage {
	comment := s.fakeGitHub.ReportedComment()
	require.NotNil(s.t, comment)
	require.NotEmpty(s.t, comment.Body)
	require.Contains(s.t, *comment.Body, ignoredReviewerMsg)
	require.Contains(s.t, *comment.Body, "alice")

	return s
}

func (s *ApiStage) ExpectNoCommentsMade() *ApiStage {
	require.Nil(s.t, s.fakeGitHub.ReportedComment())

	return s
}

func (s *ApiStage) ExpectNoReviewRequestsMade() *ApiStage {
	require.Nil(s.t, s.fakeGitHub.ReportedComment())

	return s
}

func (s *ApiStage) ExpectPreviousIgnoredReviewCommentsDeleted() *ApiStage {
	require.Empty(s.t, s.fakeGitHub.Comments())

	return s
}

func (s *ApiStage) IgnoreRepositoryExists() *ApiStage {
	ignoredRepo := fmt.Sprintf(
		"%s/%s",
		s.fakeGitHub.Org().OwnerName,
		s.fakeGitHub.Repo().Name)

	s.setupEnv("IGNORED_REPOSITORIES", ignoredRepo)
	return s
}

func ApiTest(t *testing.T) (*ApiStage, *ApiStage, *ApiStage) {
	webhookSecret, err := os.ReadFile(tokenPath)
	require.NoError(t, err)

	s := &ApiStage{
		t:             t,
		app:           NewAppServer(t),
		WebHookSecret: webhookSecret,
	}

	return s, s, s
}
