package approval

import (
	"testing"

	"github.com/form3tech-oss/github-team-approver-commons/v2/pkg/configuration"
	"github.com/stretchr/testify/require"
)

func TestTeamApprovals_AnyTeamApproved(t *testing.T) {
	tests := map[string]struct {
		teamApprovals TeamApprovals
		result        bool
	}{
		"when no teams approved": {
			teamApprovals: TeamApprovals{"A-Team": 0, "B-Team": 0},
			result:        false,
		},
		"when one team approved": {
			teamApprovals: TeamApprovals{"A-Team": 0, "B-Team": 1},
			result:        true,
		},
		"when both teams approved": {
			teamApprovals: TeamApprovals{"A-Team": 2, "B-Team": 1},
			result:        true,
		},
		"when empty": {
			teamApprovals: TeamApprovals{},
			result:        false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.result, tt.teamApprovals.AnyTeamApproved())
		})
	}
}

func TestTeamApprovals_AllTeamsApproved(t *testing.T) {
	tests := map[string]struct {
		teamApprovals TeamApprovals
		result        bool
	}{
		"when no teams approved": {
			teamApprovals: TeamApprovals{"A-Team": 0, "B-Team": 0},
			result:        false,
		},
		"when one team approved, second missing": {
			teamApprovals: TeamApprovals{"A-Team": 1},
			result:        false,
		},
		"when one team approved": {
			teamApprovals: TeamApprovals{"A-Team": 0, "B-Team": 1},
			result:        false,
		},
		"when both teams approved": {
			teamApprovals: TeamApprovals{"A-Team": 2, "B-Team": 1},
			result:        true,
		},
		"when empty": {
			teamApprovals: TeamApprovals{},
			result:        false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			handles := []string{"A-Team", "B-Team"}
			require.Equal(t, tt.result, tt.teamApprovals.AllTeamsApproved(handles))
		})
	}
}

func TestMatchedRule_ApprovingTeamNames(t *testing.T) {
	tests := map[string]struct {
		matchedRule MatchedRule
		result      []string
	}{
		"one approving team": {
			matchedRule: MatchedRule{
				Approvals: TeamApprovals{"A-Team": 0, "B-Team": 1},
			},
			result: []string{"B-Team"},
		},
		"multiple approving teams": {
			matchedRule: MatchedRule{
				Approvals: TeamApprovals{"A-Team": 2, "B-Team": 1},
			},
			result: []string{"A-Team", "B-Team"},
		},
		"no approving teams": {
			matchedRule: MatchedRule{
				Approvals: TeamApprovals{"A-Team": 0, "B-Team": 0},
			},
			result: []string{},
		},
		"empty": {
			matchedRule: MatchedRule{
				Approvals: TeamApprovals{},
			},
			result: []string{},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.result, tt.matchedRule.ApprovingTeamNames())
		})
	}
}

func TestMatchedRule_PendingTeamNames(t *testing.T) {
	tests := map[string]struct {
		matchedRule MatchedRule
		result      []string
	}{
		"pending teams": {
			matchedRule: MatchedRule{
				ConfigRule: configuration.Rule{
					ApprovingTeamHandles: []string{"A-Team", "B-Team", "C-Team"},
				},
				Approvals: TeamApprovals{"B-Team": 1},
			},
			result: []string{"A-Team", "C-Team"},
		},
		"no pending teams": {
			matchedRule: MatchedRule{
				ConfigRule: configuration.Rule{
					ApprovingTeamHandles: []string{"A-Team", "B-Team", "C-Team"},
				},
				Approvals: TeamApprovals{"A-Team": 1, "B-Team": 2, "C-Team": 3},
			},
			result: []string{},
		},
		"zero approvals counted for team": {
			matchedRule: MatchedRule{
				ConfigRule: configuration.Rule{
					ApprovingTeamHandles: []string{"A-Team", "B-Team", "C-Team"},
				},
				Approvals: TeamApprovals{"A-Team": 0, "B-Team": 1, "C-Team": 2},
			},
			result: []string{"A-Team"},
		},
		"empty": {
			matchedRule: MatchedRule{
				ConfigRule: configuration.Rule{
					ApprovingTeamHandles: []string{"A-Team", "B-Team", "C-Team"},
				},
				Approvals: TeamApprovals{},
			},
			result: []string{"A-Team", "B-Team", "C-Team"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.result, tt.matchedRule.PendingTeamNames())
		})
	}
}
