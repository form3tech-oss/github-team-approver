package internal

import (
	"context"
	"fmt"
	"github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"
	"regexp"
)

// download config
// if alert match
// fire alert


func handlePrMergeEvent(ctx context.Context, event event) error {

	var (
		ownerLogin     = event.GetRepo().GetOwner().GetLogin()
		repoName       = event.GetRepo().GetName()
		prTargetBranch = event.GetPullRequest().GetBase().GetRef()
		prBody         = event.GetPullRequest().GetBody()
	)

	getLogger(ctx).Tracef("Computing the set of alerts that applies to target branch %q", prTargetBranch)

	alerts, err := computeAlertsForTargetBranch(getClient(), ownerLogin, repoName, prTargetBranch)
	if err != nil {
		return fmt.Errorf("could not compute alerts for target branch: %s on repo: %s, err: %v", prTargetBranch, repoName, err)
	}

	// loop round all alerts checking for alerts
	for _, alert := range alerts {

		m, err := regexp.MatchString(fmt.Sprintf("(?i)%s", alert.Regex), prBody)
		if err != nil {
			return err
		}
		if m {
			getLogger(ctx).Tracef("matched alert expression: %q, firing alert", alert.Regex)
			// fire alert here
		}
	}



	return nil
}



func computeAlertsForTargetBranch(c *client, ownerLogin, repoName, targetBranch string) ([]configuration.Alert, error) {
	// Get the configuration for approvals in the current repository.
	cfg, err := c.getConfiguration(ownerLogin, repoName)
	if err != nil {
		return nil, err
	}
	// Compute the set of alerts that applies to the target branch.
	var alerts []configuration.Alert
	for _, prCfg := range cfg.PullRequestApprovalRules {
		if len(prCfg.TargetBranches) == 0 || indexOf(prCfg.TargetBranches, targetBranch) >= 0 {
			alerts = append(alerts, prCfg.Alerts...)
		}
	}
	return alerts, nil
}
