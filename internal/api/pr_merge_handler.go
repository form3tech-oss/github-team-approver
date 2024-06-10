package api

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/form3tech-oss/github-team-approver-commons/v2/pkg/configuration"
	"github.com/form3tech-oss/github-team-approver/internal/api/github"
	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

type MergeEventHandler struct {
	api    *API
	log    *logrus.Entry
	client *github.Client
}

func NewMergeEventHandler(api *API, log *logrus.Entry, client *github.Client) *MergeEventHandler {
	return &MergeEventHandler{
		api:    api,
		log:    log,
		client: client,
	}
}

func (handler *MergeEventHandler) handlePrMergeEvent(ctx context.Context, event event) error {
	var (
		ownerLogin     = event.GetRepo().GetOwner().GetLogin()
		repoName       = event.GetRepo().GetName()
		prTargetBranch = event.GetPullRequest().GetBase().GetRef()
		prBody         = event.GetPullRequest().GetBody()
	)

	if handler.api.slackWebhookSecret == "" {
		handler.log.Tracef("Ignoring alerts on repo %s: Slack Webhook Secret not configured", repoName)
		return nil
	}
	webhookURL := handler.api.slackWebhookSecret

	handler.log.Tracef("Computing the set of alerts that applies to target branch %q", prTargetBranch)

	alerts, err := handler.computeAlertsForTargetBranch(ctx, ownerLogin, repoName, prTargetBranch)
	if err != nil {
		return fmt.Errorf("could not compute alerts for target branch: %s on repo: %s, err: %w", prTargetBranch, repoName, err)
	}

	// loop round all alerts checking if alert matches PR
	for _, alert := range alerts {

		m, err := regexp.MatchString(fmt.Sprintf("(?i)%s", alert.Regex), prBody)
		if err != nil {
			return err
		}
		if m {
			handler.log.Tracef("matched alert expression: %q, firing alert", alert.Regex)
			if err != nil {
				handler.log.WithError(err).Errorf("could not decrypt: secret: %s", alert.SlackWebhookSecret)
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

			if err := slack.PostWebhook(webhookURL, &msg); err != nil {
				return err
			}
		}
	}
	return nil
}

func (handler *MergeEventHandler) computeAlertsForTargetBranch(ctx context.Context, ownerLogin, repoName, targetBranch string) ([]configuration.Alert, error) {
	// Get the configuration for approvals in the current repository.
	cfg, err := handler.client.GetConfiguration(ctx, ownerLogin, repoName)
	if err != nil {
		return nil, err
	}
	// Compute the set of alerts that applies to the target branch.
	var alerts []configuration.Alert
	for _, prCfg := range cfg.PullRequestApprovalRules {
		if len(prCfg.TargetBranches) == 0 || isMember(prCfg.TargetBranches, targetBranch) {
			alerts = append(alerts, prCfg.Alerts...)
		}
	}
	return alerts, nil
}
