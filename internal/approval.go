package internal

import (
	"context"
	"fmt"
	"net/url"
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

	commits, err := c.getPRCommits(ownerLogin, repoName, prNumber)
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

		// Reviewers who have committed to the PR as well thus dismissed as allowed reviewers
		dismissedReviewers []string
	)

	// Check if each required team has approved the pull request.
	for _, rule := range rules {
		// Check whether the pull request's body matches the aforementioned regex (ignoring case).
		var prBodyMatch bool
		if rule.Regex != "" {
			prBodyMatch, err = regexp.MatchString(fmt.Sprintf("(?i)%s", rule.Regex), prBody)
			if err != nil {
				return "", "", nil, nil, err
			}
		}
		// check whether there is a rule on a directory and it has changed
		var matchedDirectories []string
		for _, directory := range rule.Directories {
			commitFiles, err := c.getPullRequestCommitFiles(ownerLogin, repoName, prNumber)
			if err != nil {
				return "", "", nil, nil, fmt.Errorf("directory match: get pull request commit files: %v", err)
			}
			directoryMatched, err := isDirectoryChanged(directory, commitFiles)
			if err != nil {
				return "", "", nil, nil, fmt.Errorf("directory match: is directory changed: %v", err)
			}
			if directoryMatched {
				matchedDirectories = append(matchedDirectories, directory)
			}
		}

		// check whether there is a rule on a label and it matches
		prLabelMatch, err := isRegexLabelMatched(c, ownerLogin, repoName, prNumber, rule.RegexLabel)
		if err != nil {
			return "", "", nil, nil, err
		}

		if !prBodyMatch && len(matchedDirectories) == 0 && !prLabelMatch {
			getLogger(ctx).Tracef("PR doesn't match regular expression %q, directory %q or label regular expression %q", rule.Regex, rule.Directories, rule.RegexLabel)
			continue
		}
		if prBodyMatch {
			getLogger(ctx).Tracef("PR matches regular expression %q", rule.Regex)
		}
		if len(matchedDirectories) > 0 {
			getLogger(ctx).Tracef("PR matches directory %q", strings.Join(matchedDirectories, ", "))
		}
		if prLabelMatch {
			getLogger(ctx).Tracef("PR matches label regular expression %q", rule.RegexLabel)
		}
		rulesMatched += 1

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

			allowed, dismissed := splitMembers(members, commits)

			// Check whether the current team has approved the PR.
			if approvalCount := countApprovalsForTeam(reviews, allowed); approvalCount >= 1 {
				// Add the current team to the list of approving teams.
				getLogger(ctx).Tracef("Team %q has approved!", teamName)
				approvingTeamNamesForRule = appendIfMissing(approvingTeamNamesForRule, teamName)
			} else {
				// Add the current team to the slice of pending teams.
				getLogger(ctx).Tracef("Team %q hasn't approved yet", teamName)
				pendingTeamNamesForRule = appendIfMissing(pendingTeamNamesForRule, teamName)
				dismissedReviewers = uniqueAppend(dismissedReviewers, dismissed)
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
		if err := c.reportDismissedReviews(ownerLogin, repoName, prNumber, dismissedReviewers); err != nil {
			return "", "", nil, nil, err
		}
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

func isRegexLabelMatched(c *client, ownerLogin, repoName string, prNumber int, regexLabel string) (bool, error) {

	if regexLabel == "" {
		return false, nil
	}

	labels, err := c.getLabels(ownerLogin, repoName, prNumber)
	if err != nil {
		return false, fmt.Errorf("regex label match: get PR labels: %v", err)
	}

	for _, label := range labels {
		if prLabelMatch, err := regexp.MatchString(fmt.Sprintf("(?i)%s", regexLabel), label); err != nil || prLabelMatch {
			return prLabelMatch, err
		}
	}
	return false, nil
}

// return true if there were any changes in the specified directory. If the directory starts with '/' match is done
// with HasPrefix, otherwise match is done with Contains function
func isDirectoryChanged(directory string, commitFiles []*github.CommitFile) (bool, error) {

	var startsWith bool
	directory = strings.TrimSuffix(directory, "/")
	if strings.HasPrefix(directory, "/") {
		startsWith = true
		directory = strings.TrimPrefix(directory, "/")
	}

	for _, commitFile := range commitFiles {

		// we are not checking changes (or additions/deletions) because this can be a new or deleted file
		if commitFile == nil || commitFile.ContentsURL == nil {
			return false, fmt.Errorf("commit file %+v has nil contents url, skipping", commitFile)
		}

		relPath, err := contentsUrlToRelDir(*commitFile.ContentsURL)
		if err != nil {
			return false, err
		}

		if startsWith {
			if strings.HasPrefix(relPath, directory) {
				return true, nil
			}
		} else {
			if strings.Contains(relPath, directory) {
				return true, nil
			}
		}
	}
	return false, nil
}

// return relative directory of contents url (strips 'https://api.github.com/repos/<org>/<repo>/' and file part)
func contentsUrlToRelDir(contentsUrl string) (string, error) {

	u, err := url.Parse(contentsUrl)
	if err != nil {
		return "", fmt.Errorf("cannot parse contents url: %v", err)
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) < 3 {
		return "", fmt.Errorf("invalid contents url path %s, expected at least 3 parts - repos/<org>/<repo>", u.Path)
	}
	if pathParts[0] != "repos" {
		return "", fmt.Errorf("invalid contents url path %s, expected path - repos/<org>/<repo>", u.Path)
	}
	return strings.Join(pathParts[3:len(pathParts)-1], "/"), nil
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

func countApprovalsForTeam(reviews []*github.PullRequestReview, teamMembers []string) (approvalCount int) {
	// Build a map containing the usernames of each team member.
	isTeamMember := map[string]bool{}
	for _, t := range teamMembers {
		isTeamMember[t] = true
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

func splitMembers(members []*github.User, commits []*github.RepositoryCommit) ([]string, []string) {
	authors := map[string]bool{}
	for _, c := range commits {
		authors[c.GetCommitter().GetLogin()] = true
	}

	var allowed, dismissed []string

	for _, m := range members {
		login := m.GetLogin()
		if _, ok := authors[m.GetLogin()]; !ok {
			allowed = append(allowed, login)
		} else {
			dismissed = append(dismissed, login)
		}
	}

	return allowed, dismissed
}
