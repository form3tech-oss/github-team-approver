package internal

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"
	"github.com/google/go-github/v28/github"
)

func computeApprovalStatus(ctx context.Context, c *client, ownerLogin, repoName string, prNumber int, prTargetBranch string, prBody string, initialLabels []string) (status, description string, finalLabels []string, reviewsToRequest []string, err error) {
	// Compute the set of rules that applies to the target branch.
	getLogger(ctx).Tracef("Computing the set of rules that applies to target branch %q", prTargetBranch)
	rules, err := computeRulesForTargetBranch(c, ownerLogin, repoName, prTargetBranch)
	if err != nil {
		return "", "", nil, nil, err
	}
	if len(rules) > 0 {
		getLogger(ctx).Tracef("A total of %d rules apply to target branch %q", len(rules), prTargetBranch)
	} else {
		getLogger(ctx).Tracef("No rules apply to target branch %q", prTargetBranch)
		return statusEventStatusSuccess, statusEventDescriptionNoRulesForTargetBranch, nil, nil, nil
	}

	// Grab the list of teams under the current organisation.
	teams, err := c.getTeams(ownerLogin)
	if err != nil {
		return "", "", nil, nil, err
	}

	// Grab the list of all the reviews for the current PR.
	reviews, err := c.getPullRequestReviews(ownerLogin, repoName, prNumber)
	if err != nil {
		return "", "", nil, nil, err
	}

	// Copy all labels not owned by ourselves from the "initialLabels" slice into "finalLabels" so we can update the latter with the final set of labels as we go.
	for _, label := range initialLabels {
		if !strings.HasPrefix(label, pullRequestLabelPrefix) {
			finalLabels = appendIfMissing(finalLabels, label)
		}
	}

	var (
		// approvingTeamNames will hold the names of the teams that have approved the current PR.
		approvingTeamNames = make([]string, 0, 0)
		// forceApproval is used to check whether we must forcibly approve the PR (as defined by at least one rule).
		forceApproval bool
		// pendingTeamNames will hold the names of the teams that haven't approved the current PR yet.
		pendingTeamNames = make([]string, 0, 0)
		// rulesMatched will hold the total number of rules matched.
		rulesMatched = 0
	)

	// Check if each required team has approved the pull request.
	for _, rule := range rules {
		// Check whether the pull request's body matches the aforementioned regex (ignoring case).
		m, err := regexp.MatchString(fmt.Sprintf("(?i)%s", rule.Regex), prBody)
		if err != nil {
			return "", "", nil, nil, err
		}
		if m {
			getLogger(ctx).Tracef("PR matches regular expression %q", rule.Regex)
			rulesMatched += 1
		} else {
			getLogger(ctx).Tracef("PR doesn't match regular expression %q", rule.Regex)
			continue
		}

		// Add the current label to the set of final labels.
		for _, label := range rule.Labels {
			if label != "" {
				finalLabels = appendIfMissing(finalLabels, pullRequestLabelPrefix+label)
			}
		}

		// Forcibly approve the PR in case the current check is configured to do so.
		if rule.ForceApproval {
			forceApproval = true
		}

		approvingTeamNamesForRule, pendingTeamNamesForRule := make([]string, 0, 0), make([]string, 0, 0)

		// Check the approval status for each rule.
		for _, handle := range rule.ApprovingTeamHandles {
			teamName, err := getTeamNameFromTeamHandle(teams, handle)
			if err != nil {
				return "", "", nil, nil, err
			}
			// Grab the list of members on the current approving team.
			members, err := c.getTeamMembers(teams, ownerLogin, teamName)
			if err != nil {
				return "", "", nil, nil, err
			}
			// Check whether the current team has approved the PR.
			if approvalCount := countApprovalsForTeam(reviews, members); approvalCount >= 1 {
				// Add the current team to the list of approving teams.
				getLogger(ctx).Tracef("Team %q has approved!", teamName)
				approvingTeamNamesForRule = appendIfMissing(approvingTeamNamesForRule, teamName)
			} else {
				// Add the current team to the slice of pending teams.
				getLogger(ctx).Tracef("Team %q hasn't approved yet", teamName)
				pendingTeamNamesForRule = appendIfMissing(pendingTeamNamesForRule, teamName)
			}
		}

		// Add the names of the teams that have approved to the set of approving teams.
		for _, n := range approvingTeamNamesForRule {
			approvingTeamNames = appendIfMissing(approvingTeamNames, n)
		}
		// If the approval mode is "require_any" and there's at least one approval, skip requesting additional reviews.
		if rule.ApprovalMode == configuration.ApprovalModeRequireAny && len(approvingTeamNamesForRule) > 0 {
			continue
		}
		// Add the names of the teams that haven't approved yet to the set of pending teams.
		for _, n := range pendingTeamNamesForRule {
			pendingTeamNames = appendIfMissing(pendingTeamNames, n)
		}
	}

	// Compute the final status based on whether all required approvals have been met.
	switch {
	case rulesMatched == 0:
		// No rules have been matched, which represents an error.
		description = statusEventDescriptionNoRulesMatched
		status = statusEventStatusPending
	case forceApproval:
		// The PR is being forcibly approved.
		description = statusEventDescriptionForciblyApproved
		status = statusEventStatusSuccess
	case len(pendingTeamNames) > 0:
		// At least one team must still approve the PR before it goes green.
		description = fmt.Sprintf(statusEventDescriptionPendingFormatString, strings.Join(pendingTeamNames, "\n"))
		status = statusEventStatusPending
	case len(pendingTeamNames) == 0 && len(approvingTeamNames) == 0:
		// No teams have been identified as having to be requested for a review.
		// NOTE: This should not really happen in practice.
		description = statusEventDescriptionNoReviewsRequested
		status = statusEventStatusSuccess
	default:
		// The PR has been approved either by all or at least one of the approving teams.
		description = fmt.Sprintf(statusEventDescriptionApprovedFormatString, strings.Join(approvingTeamNames, "\n"))
		status = statusEventStatusSuccess
		// Avoid requesting additional reviews.
		pendingTeamNames = make([]string, 0, 0)
	}
	return status, truncate(description, statusEventDescriptionMaxLength), finalLabels, computeReviewsToRequest(ctx, teams, pendingTeamNames), nil
}

func computeReviewsToRequest(ctx context.Context, teams []*github.Team, pendingTeams []string) (reviewsToRequest []string) {
	for _, pendingTeam := range pendingTeams {
		for _, team := range teams {
			if pendingTeam == team.GetName() {
				reviewsToRequest = appendIfMissing(reviewsToRequest, team.GetSlug())
			}
		}
	}
	getLogger(ctx).Tracef("Reviews will be requested from the following teams: %v", reviewsToRequest)
	return
}

func computeRulesForTargetBranch(c *client, ownerLogin, repoName, targetBranch string) ([]configuration.Rule, error) {
	// Get the configuration for approvals in the current repository.
	cfg, err := c.getConfiguration(ownerLogin, repoName)
	if err != nil {
		return nil, err
	}
	// Compute the set of rules that applies to the target branch.
	var rules []configuration.Rule
	for _, prCfg := range cfg.PullRequestApprovalRules {
		if len(prCfg.TargetBranches) == 0 || indexOf(prCfg.TargetBranches, targetBranch) >= 0 {
			rules = append(rules, prCfg.Rules...)
		}
	}
	return rules, nil
}

func countApprovalsForTeam(reviews []*github.PullRequestReview, teamMembers []*github.User) (approvalCount int) {
	// Build a map containing the usernames of each team member.
	isTeamMember := map[string]bool{}
	for _, u := range teamMembers {
		isTeamMember[u.GetLogin()] = true
	}

	// Sort reviews for the current PR by the date they were submitted.
	sort.SliceStable(reviews, func(i, j int) bool {
		return reviews[i].GetSubmittedAt().Before(reviews[j].GetSubmittedAt())
	})

	// Pick the latest review for each team member.
	lastReviewByTeamMember := map[string]*github.PullRequestReview{}
	for _, r := range reviews {
		if isTeamMember[r.GetUser().GetLogin()] && r.GetState() != pullRequestReviewStateCommented {
			lastReviewByTeamMember[r.GetUser().GetLogin()] = r
		}
	}

	// Count and return how many approvals we've got from team members.
	approvalCount = 0
	for _, r := range lastReviewByTeamMember {
		if r.GetState() == pullRequestReviewStateApproved {
			approvalCount += 1
		}
	}
	return approvalCount
}

func getTeamNameFromTeamHandle(teams []*github.Team, v string) (string, error) {
	// Remove the "form3tech/" prefix from the team handle if it is present.
	v = strings.TrimPrefix(v, "form3tech/")
	// Lookup the resulting handle in the list of teams.
	for _, team := range teams {
		if strconv.FormatInt(team.GetID(), 10) == v || team.GetSlug() == v || team.GetName() == v {
			return team.GetName(), nil
		}
	}
	return "", fmt.Errorf("Team with name or slug %q not found", v)
}
