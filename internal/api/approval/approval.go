package approval

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"

	ghclient "github.com/form3tech-oss/github-team-approver/internal/api/github"

	"github.com/google/go-github/v28/github"
)

const (
	pullRequestReviewStateApproved  = "APPROVED"
	pullRequestReviewStateCommented = "COMMENTED"
	pullRequestLabelPrefix          = "github-team-approver/"
	statusEventDescriptionNoRulesForTargetBranch = "No rules are defined for the target branch."
	StatusEventStatusPending = "pending"
	StatusEventStatusSuccess = "success"
)

type Approval struct {
	log    *logrus.Entry
	client *ghclient.Client
}

func NewApproval(log *logrus.Entry, client *ghclient.Client) *Approval {
	return &Approval{
		log:    log,
		client: client,
	}
}

type PR struct {
	OwnerLogin    string
	RepoName      string
	TargetBranch  string
	Body          string
	Number        int
	InitialLabels []string
}

func NewPR(ownerLogin, repoName, targetBranch, body string, number int, labels []string) *PR {
	return &PR{
		OwnerLogin:    ownerLogin,
		RepoName:      repoName,
		Number:        number,
		TargetBranch:  targetBranch,
		Body:          body,
		InitialLabels: labels,
	}
}

func (approval *Approval) ComputeApprovalStatus(ctx context.Context, pr *PR) (*Result, error) {
	// Compute the set of rules that applies to the target branch.
	approval.log.Tracef("Computing the set of rules that applies to target branch %q", pr.TargetBranch)
	rules, err := approval.computeRulesForTargetBranch(ctx, pr)
	if err != nil {
		return nil, err
	}
	if len(rules) > 0 {
		approval.log.Tracef("A total of %d rules apply to target branch %q", len(rules), pr.TargetBranch)
	} else {
		approval.log.Tracef("No rules apply to target branch %q", pr.TargetBranch)
		status := &Result{
			status:      StatusEventStatusSuccess,
			description: statusEventDescriptionNoRulesForTargetBranch,
		}

		return status, nil
	}

	// Grab the list of teams under the current organisation.
	teams, err := approval.client.GetTeams(ctx, pr.OwnerLogin)
	if err != nil {
		return nil, err
	}

	commits, err := approval.client.GetPRCommits(ctx, pr.OwnerLogin, pr.RepoName, pr.Number)
	if err != nil {
		return nil, err
	}

	// Grab the list of all the reviews for the current PR.
	reviews, err := approval.client.GetPullRequestReviews(ctx, pr.OwnerLogin, pr.RepoName, pr.Number)
	if err != nil {
		return nil, err
	}

	state := newState()

	// Copy all labels not owned by ourselves from the "initialLabels" slice into "finalLabels" so we can update the latter with the final set of labels as we go.
	for _, label := range pr.InitialLabels {
		if !strings.HasPrefix(label, pullRequestLabelPrefix) {
			state.addLabel(label)
		}
	}

	// Check if each required team has approved the pull request.
	for _, rule := range rules {
		// Check whether the pull request's body matches the aforementioned regex (ignoring case).
		var prBodyMatch bool
		if rule.Regex != "" {
			prBodyMatch, err = regexp.MatchString(fmt.Sprintf("(?i)%s", rule.Regex), pr.Body)
			if err != nil {
				return nil, err
			}
		}
		// check whether there is a rule on a directory and it has changed
		var matchedDirectories []string
		for _, directory := range rule.Directories {
			commitFiles, err := approval.client.GetPullRequestCommitFiles(ctx, pr.OwnerLogin, pr.RepoName, pr.Number)
			if err != nil {
				return nil, fmt.Errorf("directory match: get pull request commit files: %w", err)
			}
			directoryMatched, err := isDirectoryChanged(directory, commitFiles)
			if err != nil {
				return nil, fmt.Errorf("directory match: is directory changed: %w", err)
			}
			if directoryMatched {
				matchedDirectories = append(matchedDirectories, directory)
			}
		}

		// check whether there is a rule on a label and it matches
		prLabelMatch, err := approval.isRegexLabelMatched(ctx, pr.OwnerLogin, pr.RepoName, pr.Number, rule.RegexLabel)
		if err != nil {
			return nil, err
		}

		if !prBodyMatch && len(matchedDirectories) == 0 && !prLabelMatch {
			approval.log.Tracef("PR doesn't match regular expression %q, directory %q or label regular expression %q", rule.Regex, rule.Directories, rule.RegexLabel)
			continue
		}
		if prBodyMatch {
			approval.log.Tracef("PR matches regular expression %q", rule.Regex)
		}
		if len(matchedDirectories) > 0 {
			approval.log.Tracef("PR matches directory %q", strings.Join(matchedDirectories, ", "))
		}
		if prLabelMatch {
			approval.log.Tracef("PR matches label regular expression %q", rule.RegexLabel)
		}
		state.incRulesMatched()

		// Add the current label to the set of final labels.
		for _, label := range rule.Labels {
			if label != "" {
				state.addLabel(fmt.Sprintf("%s%s", pullRequestLabelPrefix, label))
			}
		}

		// Forcibly approve the PR in case the current check is configured to do so.
		if rule.ForceApproval {
			state.forceApproval = true
		}

		approvingTeamNamesForRule, pendingTeamNamesForRule := make([]string, 0, 0), make([]string, 0, 0)

		// Check the approval status for each rule.
		for _, handle := range rule.ApprovingTeamHandles {
			teamName, err := getTeamNameFromTeamHandle(teams, handle)
			if err != nil {
				return nil, err
			}
			// Grab the list of members on the current approving team.
			members, err := approval.client.GetTeamMembers(ctx, teams, pr.OwnerLogin, teamName)
			if err != nil {
				return nil, err
			}

			allowed, dismissed := splitMembers(members, commits)

			// Check whether the current team has approved the PR.
			if approvalCount := countApprovalsForTeam(reviews, allowed); approvalCount >= 1 {
				// Add the current team to the list of approving teams.
				approval.log.Tracef("Team %q has approved!", teamName)
				approvingTeamNamesForRule = appendIfMissing(approvingTeamNamesForRule, teamName)
			} else {
				// Add the current team to the slice of pending teams.
				approval.log.Tracef("Team %q hasn't approved yet", teamName)
				pendingTeamNamesForRule = appendIfMissing(pendingTeamNamesForRule, teamName)
				state.addDismissedReviewers(dismissed)
			}
		}

		// Add the names of the teams that have approved to the set of approving teams.
		for _, n := range approvingTeamNamesForRule {
			state.addApprovingTeamNames(n)
		}
		// If the approval mode is "require_any" and there's at least one approval, skip requesting additional reviews.
		if rule.ApprovalMode == configuration.ApprovalModeRequireAny && len(approvingTeamNamesForRule) > 0 {
			continue
		}
		// Add the names of the teams that haven't approved yet to the set of pending teams.
		for _, n := range pendingTeamNamesForRule {
			state.addPendingTeamNames(n)
		}
	}

	result := state.result(approval.log, teams) // state should not be consumed past this point

	if result.pendingReviewsWaiting() {
		err := approval.client.ReportDismissedReviews(
			ctx, pr.OwnerLogin, pr.RepoName, pr.Number, result.dismissedReviewers)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (approval *Approval) isRegexLabelMatched(ctx context.Context, ownerLogin, repoName string, prNumber int, regexLabel string) (bool, error) {

	if regexLabel == "" {
		return false, nil
	}

	labels, err := approval.client.GetLabels(ctx, ownerLogin, repoName, prNumber)
	if err != nil {
		return false, fmt.Errorf("regex label match: get PR labels: %w", err)
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
		return "", fmt.Errorf("cannot parse contents url: %w", err)
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

func (approval *Approval) computeRulesForTargetBranch(ctx context.Context, pr *PR) ([]configuration.Rule, error) {
	// Get the configuration for approvals in the current repository.
	cfg, err := approval.client.GetConfiguration(ctx, pr.OwnerLogin, pr.RepoName)
	if err != nil {
		return nil, err
	}
	// Compute the set of rules that applies to the target branch.
	var rules []configuration.Rule
	for _, prCfg := range cfg.PullRequestApprovalRules {
		if len(prCfg.TargetBranches) == 0 || indexOf(prCfg.TargetBranches, pr.TargetBranch) >= 0 {
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
		if _, ok := authors[login]; !ok {
			allowed = append(allowed, login)
		} else {
			dismissed = append(dismissed, login)
		}
	}

	return allowed, dismissed
}
