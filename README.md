# github-team-approver

GitHub application to manage approvals to release software.

## Installing

### Registering as a GitHub App

The first step towards installing `github-team-approver` is to generate a secret meant to allow validation of incoming payloads.
It is recommended that this secret is generated using 1Password (or any other method that generates a cryptographically secure secret).
Then, you should proceed to registering `github-team-approver` as a GitHub application using [this link](https://github.com/settings/apps/new), and according to the following instructions:

* **GitHub App Name:** Choose a meaningful value.
* **Homepage URL:** Choose a meaningful value.
* **Webhook URL:** Choose `https://<host>/` (where `<host>` is the host where the application is exposed).
* **Webhook Secret**: The secret created above.
* **SSL Verification:** Depending on your setup, you may need to choose "_Disable (not recommended)_".
* **Permissions:** Choose the following sets of permissions:
  * **Repository permissions:**
    * _Contents_: _Read only_
    * _Pull requests_: _Read & write_
    * _Commit statuses_: _Read & write_
  * **Organisation permissions:**
    * _Members_: _Read only_
* **Subscribe to events:** Tick the following checkboxes:
  * _Pull request_
  * _Pull request review_
* **Where can this GitHub App be installed?** Choose "_Any account_".

Upon successful registration, you'll be taken to the GitHub application's administration page.
Take note of the value of the "_App ID_" field, as it will be needed later on.
Then, scroll down to the bottom of the page and click "_Generate a private key_".
This will generate and download the GitHub application's private key, which will be used to authenticate the application with GitHub.
Take note of the path to where the private key is downloaded.
Finally, click on the "_Install App_" tab, choose the target GitHub organization and click "_Install_" (possibly choosing only a subset of the GitHub organization's repositories).

Upon successful installation, you'll be taken to a page having a URL of following form:

```
https://github.com/organizations/<org>/settings/installations/<installation-id>
```

Take note of the value of `<installation-id>`, as it will be needed later on.

### Running

`github-team-approver` is meant to run as a standalone application on top of [Kubernetes](https://kubernetes.io/).
Hence, a Kubernetes cluster is required to run `github-team-approver`.
For local development and testing, this may correspond to a [Docker Desktop](https://www.docker.com/products/docker-desktop) or [Minikube](https://github.com/kubernetes/minikube) cluster.
For production, a managed offering of Kubernetes such as [GKE](https://cloud.google.com/kubernetes-engine/) or [EKS](https://aws.amazon.com/eks/) is strongly recommended.

Once you've got a running Kubernetes cluster, run the following command to create the required secret containing the GitHub application's private key, webhook secret token and optionally a 256 bit key if you want to use slack alerts:

```shell
$ make secret \
    GITHUB_APP_PRIVATE_KEY_PATH=<path-to-private-key> \
    GITHUB_APP_WEBHOOK_SECRET_TOKEN_PATH=<path-to-webhook-secret-token> 
```

Then, run the following command to build and deploy `github-team-approver` in development mode:

```shell
$ make skaffold.dev \
    GITHUB_APP_ID=<app-id> \
    GITHUB_APP_INSTALLATION_ID=<installation-id>
```

**NOTE:** Depending on your setup, you may need to specify a custom Docker image using the `DOCKER_IMG` and `DOCKER_TAG` variables in the command above.

**NOTE:** If you are using AWS ECR as the container registry, you may need to run the following command:

```shell
docker login <registry> --username AWS --password $(aws ecr get-login-password)
```

### Configuring

When called in result of a pull request event, `github-team-approver` reads its configuration from the `.github/GITHUB_TEAM_APPROVER.yaml` file in the default branch of the repository from which the event originated.
It then analyses the body of the source pull request, as well as its reviews, and determines whether to approve the pull request based on that data.
The aforementioned configuration file must obey the following format:

```yaml
pull_request_approval_rules:
- target_branches:
  - "<name>"
  - "<name>"
  rules:
  - regex: "<regex>"
    regex_labels: "<regex>"
    directories:
      - "<direcotry>"
    approving_team_handles:
    - "<id-or-name-or-slug>"
    - "<id-or-name-or-slug>"
    - "<id-or-name-or-slug>"
    approval_mode: "<approval-mode>"
    labels:
    - "<label>"
    - "<label>"
    force_approval: false # or true!
  alerts:
   - regex: '- \[x\] Emergency\.'
     slack_message: "emergency change merged <PR_URL>"
```

Each item under `pull_request_approval_rules` represents how approval for PRs made to specific target branches should work, according to the following table:

| Field | Description |
|----------------|-------------|
| `regex` | Regular expression to match the body of the pull request against. If matched, approval from each listed team will be required. |
| `regex_label` | Regular expression to match label(s) of the pull request against. If matched, approval from each listed team will be required. |
| `directories` | Optional list of relative or absolute paths to directories that should be checked for changes. If not provided, all directories are checked. |
| `approving_team_handles` | The list of approving teams, in the form of IDs, names or slugs. |
| `approval_mode` | One of `require_any` or `require_all`.
| `labels`  | The set of labels to apply to the pull request. Labels are prefixed with the `github-team-approver/` prefix.  |
| `force_approval` | Whether to automatically approve PRs matching the regular expression without waiting for review.
| `ignore_contributors_approval` | Whether to ignore approvals of people who pushed a commit to the PR or are a co-author of at least one of the commits. |

Each item under `alert` represents a slack alert that will fire if regex is matched:

| Field | Description |
|----------------|-------------|
| `regex` | Regular expression to match the body of the pull request against. If matched, slack alert will be fired |
| `slack_message` | Slack webhook you wish to post, should be a json object that matches [`slack.WebhookMessage`](https://github.com/slack-go/slack/blob/b04b8521281b8e06bd4bb5b9c83a81e2a12e2141/webhooks.go#L8-L18).  Set `text` field for plain message or you can use the `blocks` field to build a [fancy message](https://api.slack.com/messaging/webhooks#advanced_message_formatting). The `slack_message` field will be put through go templating with the PR event, so you can template out anything from that payload.  For an example see template, see `examples/templates/merged.template` |

A live example of a configuration file can be seen [here](https://github.com/form3tech/application-versions/blob/develop/.github/GITHUB_TEAM_APPROVER.yaml).

#### Slack integration

In order to send a slack alert you need to register a slack app and setup a webhook to a channel.  Upon doing this slack will generate a secret url, do not share this url as it will enable anyone to post to your slack channel.

Expose your generated webhook in `SLACK_WEBHOOK_SECRET` variable.

#### Remarks

* Each team listed under `approving_team_handles` should have "Read" access (at least) to the repository.
* If the `target_branches` field is omitted or left empty, the specified rules are applied to all PRs regardless of the target branch.
* PRs made against branches for which no rules are defined are automatically marked as approved.
* PRs made against branches for which rules are defined **MUST** match at least one rule to be approved.
* At the moment detecting co-authors supports only [`noreply` email addresses from GitHub](https://docs.github.com/en/account-and-profile/setting-up-and-managing-your-personal-account-on-github/managing-email-preferences/setting-your-commit-email-address#about-commit-email-addresses).
