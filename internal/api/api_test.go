package api_test

import (
	"testing"

	"github.com/form3tech-oss/github-team-approver/internal/api/stages"
)

func TestWhenSendingInvalidSignatures(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
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

func TestWhenNoContributorReviewIsUnsetAndReviewApproverIsAContributor(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
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
		ExpectSuccessAnswerReturned().
		ExpectStatusSuccessReported().
		ExpectNoCommentsMade().
		ExpectLabelsUpdated().
		ExpectNoReviewRequestsMade()
}

func TestWhenNoContributorReviewIsEnabledAndReviewApproverIsAContributor(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithNoContributorReviewEnabledAndFooAsApprovingTeam().
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

func TestWhenNoContributorReviewIsEnabledAndReviewApproverIsACoAuthor(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithNoContributorReviewEnabledAndFooAsApprovingTeam().
		PullRequestExists().
		NoCommentsExist().
		CommitsWithAliceAsCoAuthor().
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

func TestWhenMultipleRulesMatch(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeamAndMultipleRules().
		PullRequestExists().
		NoCommentsExist().
		CommitsWithAliceAsCoAuthor().
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

func TestWhenReviewApproverIsNotACoAuthor(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		CommitsWithBobAsCoAuthor().
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

func TestWhenReviewApproverIsReopener(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		NoCommentsExist().
		CommitsWithBobAsCoAuthor().
		AliceApprovesPullRequest().
		EventsWithAliceAsReopener().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectCommentAliceIgnoredAsReviewer().
		ExpectLabelsUpdated()
}

func TestForciblyApprovedPRWithoutAnyReviewsIsPassed(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeamWithEmergencyRule().
		PullRequestExists().
		PullRequestHasNoReviews().
		CommitsWithBobAsContributor().
		GitHubTeamApproverRunning()
	when.
		SendingPRReviewSubmittedEventWithForceApproval()
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
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithFooAsApprovingTeam().
		PullRequestExists().
		CommitsWithBobAndEveAsContributor().
		CharlieApprovesPullRequest().
		NoCommentsExist().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectInvalidCommentCharlieIgnoredAsReviewer().
		ExpectLabelsUpdated().
		ExpectedReviewRequestsMadeForFoo()
}

func TestGitHubTeamApproverCleansUpOldIgnoredReviewsComments(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithNoContributorReviewEnabledAndFooAsApprovingTeam().
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

func TestGitHubTeamApproverCleansUpOldInvalidReviewsComments(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithNoContributorReviewEnabledAndFooAsApprovingTeam().
		PullRequestExists().
		InvalidReviewCommentsExist().
		CommitsWithAliceAsContributor().
		CharlieApprovesPullRequest().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectPreviousIgnoredReviewCommentsDeleted().
		ExpectInvalidCommentCharlieIgnoredAsReviewer().
		ExpectLabelsUpdated().
		ExpectedReviewRequestsMadeForFoo()
}

func TestGitHubTeamApproverCleansUpOldInvalidAndRetainsIgnoredReviewsComments(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithNoContributorReviewEnabledAndFooAsApprovingTeam().
		PullRequestExists().
		InvalidAndIgnoredReviewCommentsExist().
		CommitsWithAliceAsContributor().
		CharlieApprovesPullRequest().
		GitHubTeamApproverRunning()
	when.
		SendingApprovedPRReviewSubmittedEvent()
	then.
		ExpectPendingAnswerReturned().
		ExpectStatusPendingReported().
		ExpectPreviousAliceIgnoredReviewCommentsRetained().
		ExpectInvalidCommentCharlieIgnoredAsReviewer().
		ExpectLabelsUpdated().
		ExpectedReviewRequestsMadeForFoo()
}

func TestGitHubTeamApproverReportsInvalidTeamHandlesInConfiguration(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithConfigurationReferencingInvalidTeamHandles().
		PullRequestExists().
		NoReviewsExist().
		GitHubTeamApproverRunning()
	when.
		SendingPREvent()
	then.
		ExpectErrorAnswerReturned().
		ExpectStatusErrorReported().
		ExpectInvalidTeamHandleInStatusDescription()
}

func TestNoTeamRequestedForReviewIfConfigurationIsInvalid(t *testing.T) {
	given, when, then := stages.ApiTest(t)

	given.
		GitHubWebHookTokenExists().
		FakeGHRunning().
		OrganisationWithTeamFoo().
		RepoWithConfigurationReferencingInvalidTeamHandles().
		PullRequestExists().
		NoReviewsExist().
		GitHubTeamApproverRunning()
	when.
		SendingPREvent()
	then.
		ExpectNoReviewRequestsMade()
}
