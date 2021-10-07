package internal

const (
	defaultAppName = "github-team-approver"
)

const (
	contextKeyLogger = "logger"
)

const (
	eventTypePullRequest       = "pull_request"
	eventTypePullRequestReview = "pull_request_review"
)

const (
	envAppName                         = "APP_NAME"
	envGitHubAppId                     = "GITHUB_APP_ID"
	envGitHubAppInstallationId         = "GITHUB_APP_INSTALLATION_ID"
	envGitHubAppPrivateKeyPath         = "GITHUB_APP_PRIVATE_KEY_PATH"
	envGitHubAppWebhookSecretTokenPath = "GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH"
	envGitHubBaseURL                   = "GITHUB_BASE_URL"
	envGitHubStatusName                = "GITHUB_STATUS_NAME"
	envIgnoredRepositories             = "IGNORED_REPOSITORIES"
	envLogLevel                        = "LOG_LEVEL"
	envSecretStoreType                 = "SECRET_STORE_TYPE" // Set to AWS_SSM for the ability to run in ECS using SSM. Empty, not set or anything else for default K8s secret
	envLogzioTokenPath                 = "LOGZIO_TOKEN_PATH"
	envEncryptionKeyPath               = "ENCRYPTION_KEY_PATH"
	envUseCachingTransport             = "USE_CACHING_TRANSPORT"
)

const (
	httpHeaderXFinalStatus    = "X-Final-Status"
	httpHeaderXGithubDelivery = "X-GitHub-Delivery"
	httpHeaderXGithubEvent    = "X-GitHub-Event"
	httpHeaderXHubSignature   = "X-Hub-Signature"
)

const (
	logFieldDeliveryID  = "delivery_id"
	logFieldEventType   = "event_type"
	logFieldPR          = "pr"
	logFieldRepo        = "repo"
	logFieldServiceName = "service_name"
	logFieldOwner       = "owner"
	logFieldPage        = "page"
)

const (
	logzioListenerURL = "https://listener-eu.logz.io:8071"
)

const (
	pullRequestActionEdited      = "edited"
	pullRequestActionOpened      = "opened"
	pullRequestActionReopened    = "reopened"
	pullRequestActionSynchronize = "synchronize"
	pullRequestActionClosed      = "closed"
)

const (
	pullRequestLabelPrefix = "github-team-approver/"
)

const (
	pullRequestReviewActionDismissed = "dismissed"
	pullRequestReviewActionEdited    = pullRequestActionEdited
	pullRequestReviewActionSubmitted = "submitted"
)

const (
	pullRequestReviewStateApproved  = "APPROVED"
	pullRequestReviewStateCommented = "COMMENTED"
)

const (
	statusEventDescriptionApprovedFormatString   = "Approved by:\n%s"
	statusEventDescriptionForciblyApproved       = "Forcibly approved."
	statusEventDescriptionMaxLength              = 140
	statusEventDescriptionNoReviewsRequested     = "No teams have been identified as having to be requested for a review."
	statusEventDescriptionNoRulesMatched         = "The PR's body doesn't meet the requirements."
	statusEventDescriptionNoRulesForTargetBranch = "No rules are defined for the target branch."
	statusEventDescriptionPendingFormatString    = "Needs approval from:\n%s"
)

const (
	statusEventStatusPending = "pending"
	statusEventStatusSuccess = "success"
)
