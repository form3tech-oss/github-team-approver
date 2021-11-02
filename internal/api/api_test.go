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

func TestWhenRepoLacksConfigurationFile(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithoutConfigurationFile().
		PullRequestExists().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		StatusNoContentReturned().
		ExpectNoReviewRequestsMade().
		ExpectNoCommentsMade()
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
		ExpectCommentAliceIgnoredAsReviewer().
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

func TestWhenPRHasNoReviewsAndAuthorIsPartOfTeam(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		CommitsWithBobAsContributor().
		PullRequestHasNoReviews().
		NoCommentsExist().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectNoCommentsMade().
		ExpectLabelsUpdated().
		ExpectNoReviewRequestsMade()
}

func TestWhenPRReviewedByAuthorNotPartOfTeam(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		CommitsWithCharlieAsContributor().
		AliceApprovesPullRequest().
		NoCommentsExist().
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

func TestWhenPRReviewedByNonTeamMemberAndAuthorsArePartOfTeam(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		CommitsWithBobandEveAsContributor().
		CharlieApprovesPullRequest().
		NoCommentsExist().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectNoCommentsMade().
		ExpectLabelsUpdated().
		ExpectNoReviewRequestsMade()
}

func TestGitHubTeamApproverCleansUpOldIgnoredReviewsComments(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		EncryptionKeyExists().
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		IgnoredReviewCommentsExist().
		CommitsWithAliceAsContributor().
		AliceApprovesPullRequest().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectPreviousIgnoredReviewCommentsDeleted().
		ExpectCommentAliceIgnoredAsReviewer().
		ExpectLabelsUpdated().
		ExpectedReviewRequestsMadeForFoo()
}