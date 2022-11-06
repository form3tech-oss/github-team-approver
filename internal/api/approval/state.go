package approval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/v42/github"
	log "github.com/sirupsen/logrus"
)

const (
	statusEventDescriptionApprovedFormatString = "Approved by:\n%s"
	statusEventDescriptionForciblyApproved     = "Forcibly approved."
	statusEventDescriptionNoReviewsRequested   = "No teams have been identified as having to be requested for a review."
	statusEventDescriptionNoRulesMatched       = "The PR's body doesn't meet the requirements."
	statusEventDescriptionPendingFormatString  = "Needs approval from:\n%s"
	statusEventDescriptionInvalidTeamHandles   = "Invalid config: no teams could be found for the following handles:\n%s"
)

type state struct {
	labels []string
	// forceApproval is used to check whether we must forcibly approve the PR (as defined by at least one rule).
	forceApproval bool
	// matchedRules keeps track of all rules applicable to the current PR and their reviews
	matchedRules []MatchedRule
	// approvingReviewers tracks users who have approved the pull request
	approvingReviewers map[string]bool
	// Reviewers who have committed to the PR as well thus ignored as allowed reviewers
	ignoredReviewers []string
	// Invalid team handles found in configuration
	invalidTeamHandles []string
}

func newState() *state {
	return &state{
		approvingReviewers: make(map[string]bool),
		matchedRules:       make([]MatchedRule, 0),
	}
}

func (s *state) addLabel(label string) {
	s.labels = appendIfMissing(s.labels, label)
}

func (s *state) addMatchedRule(mr MatchedRule) {
	s.matchedRules = append(s.matchedRules, mr)
}

func (s *state) addInvalidTeamHandle(name string) {
	s.invalidTeamHandles = appendIfMissing(s.invalidTeamHandles, name)
}

func (s *state) setApprovingReviewers(reviews []*github.PullRequestReview) {
	approving := map[string]bool{}

	for _, review := range reviews {
		if *review.State == pullRequestReviewStateApproved {
			approving[review.GetUser().GetLogin()] = true
		}
	}
	s.approvingReviewers = approving
}

// The members that will be mentioned in the bot comment when it publishes ignored PR reviewers comment.
// We only mention reviewers who approved a PR and also contributed to the PR thus being ignored as valid reviewers.
// Without this filtering, we would be mentioning all contributing authors even if they didn't review the PR.
func (s *state) addIgnoredReviewers(ignoredTeamMembers []string) {
	var ignoredReviewers []string

	for _, r := range ignoredTeamMembers {
		if _, ok := s.approvingReviewers[r]; ok {
			ignoredReviewers = append(ignoredReviewers, r)
		}
	}

	s.ignoredReviewers = uniqueAppend(s.ignoredReviewers, ignoredReviewers)
}

func (s *state) allRulesFulfilled() bool {
	for _, rule := range s.matchedRules {
		if !rule.Fulfilled() {
			return false
		}
	}

	return true
}

func (s *state) pendingTeamNames() []string {
	allPending := []string{}
	for _, rule := range s.matchedRules {
		rulePending := rule.PendingTeamNames()
		allPending = uniqueAppend(allPending, rulePending)
	}

	return allPending
}

func (s *state) approvingTeamNames() []string {
	allApproving := []string{}
	for _, rule := range s.matchedRules {
		ruleApproving := rule.ApprovingTeamNames()
		allApproving = uniqueAppend(allApproving, ruleApproving)
	}

	return allApproving
}

func (s *state) shouldForceApprove() bool {
	for _, rule := range s.matchedRules {
		if rule.ConfigRule.ForceApproval {
			return true
		}
	}

	return false
}

func (s *state) result(log *log.Entry, teams []*github.Team) *Result {
	result := &Result{
		finalLabels:      s.labels,
		ignoredReviewers: s.ignoredReviewers,
	}

	pendingTeamNames := s.pendingTeamNames()
	approvingTeamNames := s.approvingTeamNames()
	allRulesFulfilled := s.allRulesFulfilled()

	// Compute the final status based on whether all required approvals have been met.
	switch {
	case len(s.matchedRules) == 0:
		// No rules have been matched, which represents an error.
		result.description = statusEventDescriptionNoRulesMatched
		result.status = StatusEventStatusPending
	case len(s.invalidTeamHandles) > 0:
		// The configuration references a non-existent team
		result.description = fmt.Sprintf(statusEventDescriptionInvalidTeamHandles, strings.Join(s.invalidTeamHandles, "\n"))
		result.status = StatusEventStatusError
	case s.shouldForceApprove():
		// The PR is being forcibly approved.
		result.description = statusEventDescriptionForciblyApproved
		result.status = StatusEventStatusSuccess
		result.reviewsToRequest = computeReviewsToRequest(log, teams, pendingTeamNames)
	case len(pendingTeamNames) > 0 && !allRulesFulfilled:
		// At least one team must still approve the PR before it goes green.
		result.description = fmt.Sprintf(
			statusEventDescriptionPendingFormatString, strings.Join(pendingTeamNames, "\n"))
		result.status = StatusEventStatusPending
		result.reviewsToRequest = computeReviewsToRequest(log, teams, pendingTeamNames)
	case len(pendingTeamNames) == 0 && len(approvingTeamNames) == 0:
		// No teams have been identified as having to be requested for a review.
		// NOTE: This should not really happen in practice.
		result.description = statusEventDescriptionNoReviewsRequested
		result.status = StatusEventStatusSuccess
	case allRulesFulfilled:
		sort.Strings(approvingTeamNames)
		result.description = fmt.Sprintf(statusEventDescriptionApprovedFormatString, strings.Join(approvingTeamNames, "\n"))
		result.status = StatusEventStatusSuccess
	}

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
