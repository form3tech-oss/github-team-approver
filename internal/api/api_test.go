package api_test

import (
	"github.com/form3tech-oss/github-team-approver/internal/api/stages"
	"testing"
)

func TestWhenSendingInvalidSignatures(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		GitHubTeamApproverRunning()
	when.
		SendingEventWithInvalidSignature()
	then.
		ExpectBadRequestReturned()
}
func TestWhenSendingUnsupportedEvent(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		GitHubTeamApproverRunning()
	when.
		SendingAnUnsupportedEvent()
	then.
		StatusNoContentReturned()
}

func TestWhenEventIsForIgnoredRepository(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		IgnoreRepositoryExists().
		PullRequestExists().
		CommitsWithAliceAsContributor().
		AliceApprovesPullRequest().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		StatusNoContentReturned().
		ExpectNoReviewRequestsMade().
		ExpectNoCommentsMade()
}

func TestWhenReviewApproverIsAContributor(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		NoCommentsExist().
		CommitsWithAliceAsContributor().
		AliceApprovesPullRequest().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectCommentAliceDismissedAsReviewer().
		ExpectLabelsUpdated().
		ExpectedReviewRequestsMadeForFoo()
}

func TestWhenReviewApproverIsNotAContributor(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		CommitsWithBobAsContributor().
		AliceApprovesPullRequest().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectSuccessAnswerReturned().
		ExpectStatusSuccessReported().
		ExpectNoCommentsMade().
		ExpectLabelsUpdated().
		ExpectNoReviewRequestsMade()
}

func TestGitHubTeamApproverCleansUpOldDismissedComments(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		DismissedCommentsExist().
		CommitsWithAliceAsContributor().
		AliceApprovesPullRequest().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectPreviousDismissedCommentsDeleted().
		ExpectCommentAliceDismissedAsReviewer().
		ExpectLabelsUpdated().
		ExpectedReviewRequestsMadeForFoo()
}
