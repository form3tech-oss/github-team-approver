package approval

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/form3tech-oss/github-team-approver-commons/v2/pkg/configuration"

	ghclient "github.com/form3tech-oss/github-team-approver/internal/api/github"

	"github.com/google/go-github/v42/github"
)

const (
	pullRequestReviewStateApproved               = "APPROVED"
	pullRequestReviewStateCommented              = "COMMENTED"
	pullRequestLabelPrefix                       = "github-team-approver/"
	statusEventDescriptionNoRulesForTargetBranch = "No rules are defined for the target branch."
	StatusEventStatusPending                     = "pending"
	StatusEventStatusSuccess                     = "success"
	StatusEventStatusError                       = "error"
)

var (
	ErrInvalidTeamHandle = errors.New("No team could be found with given name or slug")
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
	Author        *github.User
}

func NewPR(ownerLogin, repoName, targetBranch, body string, number int, labels []string, author *github.User) *PR {
	return &PR{
		OwnerLogin:    ownerLogin,
		RepoName:      repoName,
		Number:        number,
		TargetBranch:  targetBranch,
		Body:          body,
		InitialLabels: labels,
		Author:        author,
	}
}

func (a *Approval) ComputeApprovalStatus(ctx context.Context, pr *PR) (*Result, error) {
	// Get the configuration for approvals in the current repository.
	cfg, err := a.client.GetConfiguration(ctx, pr.OwnerLogin, pr.RepoName)
	if err != nil {
		return nil, err
	}

	rules, err := a.computeRulesForTargetBranch(cfg, pr)
	if err != nil {
		return nil, err
	}
	if len(rules) > 0 {
		a.log.Tracef("A total of %d rules apply to target branch %q", len(rules), pr.TargetBranch)
	} else {
		a.log.Tracef("No rules apply to target branch %q", pr.TargetBranch)
		status := &Result{
			status:      StatusEventStatusSuccess,
			description: statusEventDescriptionNoRulesForTargetBranch,
		}

		return status, nil
	}

	// Grab the list of teams under the current organisation.
	teams, err := a.client.GetTeams(ctx, pr.OwnerLogin)
	if err != nil {
		return nil, err
	}

	// Grab the list of all the reviews for the current PR.
	reviews, err := a.client.GetPullRequestReviews(ctx, pr.OwnerLogin, pr.RepoName, pr.Number)
	if err != nil {
		return nil, err
	}

	state := newState()
	state.setApprovingReviewers(reviews)

	// Copy all labels not owned by ourselves from the "initialLabels" slice into "finalLabels" so we can update the latter with the final set of labels as we go.
	for _, label := range pr.InitialLabels {
		if !strings.HasPrefix(label, pullRequestLabelPrefix) {
			state.addLabel(label)
		}
	}

	// Check if each required team has approved the pull request.
	for _, rule := range rules {
		matched, err := a.isRuleMatched(ctx, rule, pr)
		if err != nil {
			return nil, err
		}

		if !matched {
			continue
		}

		// Add the current label to the set of final labels.
		for _, label := range rule.Labels {
			if label != "" {
				state.addLabel(fmt.Sprintf("%s%s", pullRequestLabelPrefix, label))
			}
		}

		mr := NewMatchedRule(rule)
		// Check the approval status for each rule.
		for _, handle := range rule.ApprovingTeamHandles {
			teamName, err := getTeamNameFromTeamHandle(teams, handle)
			if err != nil {
				if errors.Is(err, ErrInvalidTeamHandle) {
					state.addInvalidTeamHandle(handle)
					continue
				}

				return nil, err
			}
			// Grab the list of members on the current approving team.
			members, err := a.client.GetTeamMembers(ctx, teams, pr.OwnerLogin, teamName)
			if err != nil {
				return nil, err
			}

			allowed, ignored, err := a.allowedAndIgnoreReviewers(ctx, pr, members, rule.IgnoreContributorApproval)
			if err != nil {
				return nil, err
			}

			// Check whether the current team has approved the PR.
			approvalCount := countApprovalsForTeam(reviews, allowed)
			// Need to use full team handle here, as we'll be comparing recorded handles
			// to all approving team handles before computing the final status.
			mr.RecordApproval(handle, approvalCount)
			state.addIgnoredReviewers(ignored)
		}
		state.addMatchedRule(mr)
	}

	result := state.result(a.log, teams) // state should not be consumed past this point

	if result.pendingReviewsWaiting() {
		err := a.client.ReportIgnoredReviews(
			ctx, pr.OwnerLogin, pr.RepoName, pr.Number, result.IgnoredReviewers())
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (a *Approval) isRuleMatched(ctx context.Context, rule configuration.Rule, pr *PR) (bool, error) {
	// Check whether the pull request's body matches the aforementioned regex (ignoring case).
	prBodyMatch, err := a.isRegexMatched(ctx, pr.OwnerLogin, pr.RepoName, pr.Number, rule.Regex, pr.Body)
	if err != nil {
		return false, err
	}
	// check whether there is a rule on a directory and it has changed
	directoriesMatch, err := a.areDirectoriesMatched(ctx, pr.OwnerLogin, pr.RepoName, pr.Number, rule.Directories)
	if err != nil {
		return false, err
	}
	// check whether there is a rule on a label and it matches
	prLabelMatch, err := a.isRegexLabelMatched(ctx, pr.OwnerLogin, pr.RepoName, pr.Number, rule.RegexLabel)
	if err != nil {
		return false, err
	}

	if !prBodyMatch && !directoriesMatch && !prLabelMatch {
		a.log.Tracef("PR doesn't match regular expression %v, directory %v or label regular expression %v", rule.Regex, rule.Directories, rule.RegexLabel)
		return false, nil
	}

	shouldMatchDirectories := len(rule.Directories) > 0
	if shouldMatchDirectories && !directoriesMatch {
		a.log.WithField("directories", rule.Directories).Tracef("Rule has 'directories' set but PR does not match")
		return false, nil
	}

	shouldMatchBody := rule.Regex != ""
	if shouldMatchBody && !prBodyMatch {
		a.log.WithField("regex", rule.Regex).Tracef("Rule has 'regex' set but PR does not match")
		return false, nil
	}

	shouldMatchLabels := rule.RegexLabel != ""
	if shouldMatchLabels && !prLabelMatch {
		a.log.WithField("regex_label", rule.RegexLabel).Tracef("Rule has 'regex_label' set but PR does not match")
		return false, nil
	}

	a.log.WithFields(logrus.Fields{
		"pr":   pr.Number,
		"rule": rule,
	}).Tracef("PR matches rule")
	return true, nil
}

func (a *Approval) isRegexMatched(ctx context.Context, ownerLogin, repoName string, prNumber int, regex string, body string) (bool, error) {
	if regex == "" {
		return false, nil
	}

	var prBodyMatch bool
	prBodyMatch, err := regexp.MatchString(fmt.Sprintf("(?i)%s", regex), body)
	if err != nil {
		return false, err
	}

	return prBodyMatch, nil
}

func (a *Approval) areDirectoriesMatched(ctx context.Context, ownerLogin, repoName string, prNumber int, directories []string) (bool, error) {
	var matchedDirectories []string

	for _, directory := range directories {
		commitFiles, err := a.client.GetPullRequestCommitFiles(ctx, ownerLogin, repoName, prNumber)
		if err != nil {
			return false, fmt.Errorf("directory match: get pull request commit files: %w", err)
		}
		directoryMatched, err := isDirectoryChanged(directory, commitFiles)
		if err != nil {
			return false, fmt.Errorf("directory match: is directory changed: %w", err)
		}
		if directoryMatched {
			matchedDirectories = append(matchedDirectories, directory)
		}
	}

	return len(matchedDirectories) > 0, nil
}

func (a *Approval) isRegexLabelMatched(ctx context.Context, ownerLogin, repoName string, prNumber int, regexLabel string) (bool, error) {

	if regexLabel == "" {
		return false, nil
	}

	labels, err := a.client.GetLabels(ctx, ownerLogin, repoName, prNumber)
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

// computeRulesForTargetBranch computes the set of rules that applies to the target branch.
func (a *Approval) computeRulesForTargetBranch(cfg *configuration.Configuration, pr *PR) ([]configuration.Rule, error) {
	a.log.Tracef("Computing the set of rules that applies to target branch %q", pr.TargetBranch)

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
	return "", fmt.Errorf("Invalid team handle: %q %w", v, ErrInvalidTeamHandle)
}

func (a *Approval) allowedAndIgnoreReviewers(ctx context.Context, pr *PR, members []*github.User, ignoreContributors bool) ([]string, []string, error) {
	if !ignoreContributors {
		var allowedMembers []string
		for _, m := range members {
			allowedMembers = append(allowedMembers, m.GetLogin())
		}
		return allowedMembers, []string{}, nil
	}

	commits, err := a.client.GetPRCommits(ctx, pr.OwnerLogin, pr.RepoName, pr.Number)
	if err != nil {
		return nil, nil, err
	}

	allowed, ignored := filterAllowedAndIgnoreReviewers(members, commits)
	return allowed, ignored, nil
}

func filterAllowedAndIgnoreReviewers(members []*github.User, commits []*github.RepositoryCommit) ([]string, []string) {
	authors := map[string]bool{}
	for _, c := range commits {
		authors[c.GetCommitter().GetLogin()] = true
		for _, coauthor := range findCoAuthors(c.GetCommit().GetMessage()) {
			authors[coauthor] = true
		}
	}

	var allowed, ignored []string

	for _, m := range members {
		login := m.GetLogin()
		if _, ok := authors[login]; !ok {
			allowed = append(allowed, login)
		} else {
			ignored = append(ignored, login)
		}
	}

	return allowed, ignored
}

func findCoAuthors(msg string) []string {
	pattern := "Co-authored-by: .+? <([\\w\\+-]+)@users.noreply.github.com>"
	r := regexp.MustCompile(pattern)

	coauthors := []string{}
	for _, match := range r.FindAllStringSubmatch(msg, -1) {
		coauthor := match[1]
		if strings.Contains(coauthor, "+") {
			parts := strings.Split(coauthor, "+")
			coauthors = append(coauthors, parts[1])
		} else {
			coauthors = append(coauthors, coauthor)
		}
	}

	return coauthors
}
