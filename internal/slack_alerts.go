package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"
	"github.com/slack-go/slack"
	"regexp"
)

func handlePrMergeEvent(ctx context.Context, event event) error {

	var (
		ownerLogin     = event.GetRepo().GetOwner().GetLogin()
		repoName       = event.GetRepo().GetName()
		prTargetBranch = event.GetPullRequest().GetBase().GetRef()
		prBody         = event.GetPullRequest().GetBody()
	)

	if c == nil {
		return fmt.Errorf("can not handle webhook as cryptor not setup")
	}

	getLogger(ctx).Tracef("Computing the set of alerts that applies to target branch %q", prTargetBranch)

	alerts, err := computeAlertsForTargetBranch(getClient(), ownerLogin, repoName, prTargetBranch)
	if err != nil {
		return fmt.Errorf("could not compute alerts for target branch: %s on repo: %s, err: %v", prTargetBranch, repoName, err)
	}

	// loop round all alerts checking if alert matches PR
	for _, alert := range alerts {

		m, err := regexp.MatchString(fmt.Sprintf("(?i)%s", alert.Regex), prBody)
		if err != nil {
			return err
		}
		if m {
			getLogger(ctx).Tracef("matched alert expression: %q, firing alert", alert.Regex)

			url, err := c.Decrypt(alert.SlackWebhookSecret)
			if err != nil {
				getLogger(ctx).Errorf("could not decrypt: secret: %s, err: %v", alert.SlackWebhookSecret, err)
				return err
			}

			bytes, err := renderTemplate(event, alert.SlackMessage)
			if err != nil {
				return err
			}

			var msg slack.WebhookMessage
			err = json.Unmarshal(bytes, &msg)
			if err != nil {
				return err
			}

			if err := slack.PostWebhook(url, &msg); err != nil {
				return err
			}
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
