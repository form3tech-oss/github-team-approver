package approval

import (
	"sort"

	"github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"
)

type MatchedRule struct {
	ConfigRule configuration.Rule
	Approvals  TeamApprovals
}

func NewMatchedRule(rule configuration.Rule) MatchedRule {
	return MatchedRule{
		ConfigRule: rule,
		Approvals:  make(TeamApprovals),
	}
}

func (mr MatchedRule) RecordApproval(teamHandle string, count int) {
	if count >= 1 {
		mr.Approvals[teamHandle] = count
	}
}

func (mr MatchedRule) ApprovingTeamNames() []string {
	approving := []string{}
	for name, approvals := range mr.Approvals {
		if approvals > 0 {
			approving = append(approving, name)
		}
	}

	sort.Strings(approving)
	return approving
}

func (mr MatchedRule) PendingTeamNames() []string {
	pending := []string{}
	for _, name := range mr.ConfigRule.ApprovingTeamHandles {
		approvals, ok := mr.Approvals[name]
		if !ok || approvals == 0 {
			pending = append(pending, name)
		}
	}

	sort.Strings(pending)
	return pending
}

func (mr MatchedRule) Fulfilled() bool {
	r := mr.ConfigRule

	switch {
	case r.ForceApproval:
		return true
	case r.ApprovalMode == configuration.ApprovalModeRequireAny:
		return mr.Approvals.AnyTeamApproved()
	case r.ApprovalMode == configuration.ApprovalModeRequireAll:
		return mr.Approvals.AllTeamsApproved(r.ApprovingTeamHandles)
	}

	return false
}

type TeamApprovals map[string]int

func (ta TeamApprovals) AnyTeamApproved() bool {
	total := 0
	for _, count := range ta {
		total += count
	}
	return total >= 1
}

func (ta TeamApprovals) AllTeamsApproved(allTeams []string) bool {
	for _, handle := range allTeams {
		approvals, present := ta[handle]
		if !present || approvals < 1 {
			return false
		}
	}
	return true
}
