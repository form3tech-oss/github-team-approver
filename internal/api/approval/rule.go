package approval

import "github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"

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
	approved := []string{}
	for name, approvals := range mr.Approvals {
		if approvals > 0 {
			approved = append(approved, name)
		}
	}

	return approved
}

func (mr MatchedRule) PendingTeamNames() []string {
	pending := []string{}
	for _, name := range mr.ConfigRule.ApprovingTeamHandles {
		// only append if we don't have an approval or we have zero approvals
		approvals, ok := mr.Approvals[name]
		if !ok || approvals == 0 {
			pending = append(pending, name)
		}
	}
	return pending
}

func (mr MatchedRule) IsFulfilled() bool {
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

func (ta TeamApprovals) AllTeamsApproved(handles []string) bool {
	for _, handle := range handles {
		approvals, present := ta[handle]
		if !present || approvals < 1 {
			return false
		}
	}
	return true
}
