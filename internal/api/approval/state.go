package approval

import (
	"fmt"
	"github.com/google/go-github/v28/github"
	log "github.com/sirupsen/logrus"
	"strings"
)

const (
	statusEventDescriptionApprovedFormatString   = "Approved by:\n%s"
	statusEventDescriptionForciblyApproved       = "Forcibly approved."
	statusEventDescriptionNoReviewsRequested     = "No teams have been identified as having to be requested for a review."
	statusEventDescriptionNoRulesMatched         = "The PR's body doesn't meet the requirements."
	statusEventDescriptionPendingFormatString    = "Needs approval from:\n%s"
)

type state struct {
	labels []string

	// forceApproval is used to check whether we must forcibly approve the PR (as defined by at least one rule).
	forceApproval bool
	// rulesMatched will hold the total number of rules matched.
	rulesMatched int

	// approvingTeamNames will hold the names of the teams that have approved the current PR.
	approvingTeamNames []string
	// pendingTeamNames will hold the names of the teams that haven't approved the current PR yet.
	pendingTeamNames []string
	// Reviewers who have committed to the PR as well thus ignored as allowed reviewers
	ignoredReviewers []string
}

func newState() *state {
	return &state{}
}

func (s *state) addLabel(label string) {
	s.labels = appendIfMissing(s.labels, label)
}

func (s *state) incRulesMatched() {
	s.rulesMatched += 1
}

func (s *state) addApprovingTeamNames(name string) {
	s.approvingTeamNames = appendIfMissing(s.approvingTeamNames, name)
}

func (s *state) addPendingTeamNames(name string) {
	s.pendingTeamNames = appendIfMissing(s.pendingTeamNames, name)
}

func (s *state) addIgnoredReviewers(reviewers []string) {
	s.ignoredReviewers = uniqueAppend(s.ignoredReviewers, reviewers)
}

func (s *state) result(log *log.Entry, teams []*github.Team) *Result {
	result := &Result{
		finalLabels:      s.labels,
		ignoredReviewers: s.ignoredReviewers,
	}

	// Compute the final status based on whether all required approvals have been met.
	switch {
	case s.rulesMatched == 0:
		// No rules have been matched, which represents an error.
		result.description = statusEventDescriptionNoRulesMatched
		result.status = StatusEventStatusPending
	case s.forceApproval:
		// The PR is being forcibly approved.
		result.description = statusEventDescriptionForciblyApproved
		result.status = StatusEventStatusSuccess
	case len(s.pendingTeamNames) > 0:
		// At least one team must still approve the PR before it goes green.
		result.description = fmt.Sprintf(
			statusEventDescriptionPendingFormatString, strings.Join(s.pendingTeamNames, "\n"))
		result.status = StatusEventStatusPending
	case len(s.pendingTeamNames) == 0 && len(s.approvingTeamNames) == 0:
		// No teams have been identified as having to be requested for a review.
		// NOTE: This should not really happen in practice.
		result.description = statusEventDescriptionNoReviewsRequested
		result.status = StatusEventStatusSuccess
	default:
		// The PR has been approved either by all or at least one of the approving teams.
		result.description = fmt.Sprintf(statusEventDescriptionApprovedFormatString, strings.Join(s.approvingTeamNames, "\n"))
		result.status = StatusEventStatusSuccess

		// Avoid requesting additional reviews.
		s.pendingTeamNames = make([]string, 0, 0)
	}

	result.reviewsToRequest = computeReviewsToRequest(log, teams, s.pendingTeamNames)

	return result
}

func computeReviewsToRequest(log *log.Entry, teams []*github.Team, pendingTeams []string) []string {
	var reviewsToRequest []string

	for _, pendingTeam := range pendingTeams {
		for _, team := range teams {
			if pendingTeam == team.GetName() {
				reviewsToRequest = appendIfMissing(reviewsToRequest, team.GetSlug())
			}
		}
	}
	log.Tracef("Reviews will be requested from the following teams: %v", reviewsToRequest)
	return reviewsToRequest
}