# github-team-approver

GitHub application to manage approvals to release software.

## Installing

### Prerequisites

* A [Kubernetes](https://kubernetes.io/) cluster having [OpenFaaS](https://www.openfaas.com/) installed and publicly exposed.

### Registering as a GitHub App

The first step towards installing `github-team-approver` is to generate a secret meant to allow validation of incoming payloads.
It is recommended that this secret is generated using 1Password (or any other method that generates a cryptographically secure secret).
Then, you should proceed to registering `github-team-approver` as a GitHub application using [this link](https://github.com/settings/apps/new), and according to the following instructions:

* **GitHub App Name:** Choose a meaningful value.
* **Homepage URL:** Choose a meaningful value.
* **Webhook URL:** Choose `https://<host>/function/github-team-approver` (where `<host>` is the host where the [OpenFaaS](https://www.openfaas.com/) gateway is exposed).
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

`github-team-approver` runs as an [OpenFaaS](https://www.openfaas.com/) function.
In its turn, OpenFaas runs on top of [Kubernetes](https://kubernetes.io/).
Hence, and as mentioned above, a Kubernetes cluster is required to run `github-team-approver`.
For local development and testing, this may correspond to a [Docker Desktop](https://www.docker.com/products/docker-desktop) or [Minikube](https://github.com/kubernetes/minikube) cluster.
For production, a managed offering of Kubernetes such as [GKE](https://cloud.google.com/kubernetes-engine/) or [EKS](https://aws.amazon.com/eks/) is strongly recommended.

Once you've got a running OpenFaaS installation (including a working `faas` or `faas-cli` command), run the following command to create the required [secrets](https://docs.openfaas.com/reference/secrets/) (containing the GitHub application's private key and webhook secret token):

```shell
$ make secrets \
    GITHUB_APP_PRIVATE_KEY=<path-to-private-key> \
    GITHUB_APP_WEBHOOK_SECRET_TOKEN=<path-to-webhook-secret-token>
```

Then, run the following command to build, push and deploy `github-team-approver`:

```shell
$ make up \
    GITHUB_APP_ID=<app-id> \
    GITHUB_APP_INSTALLATION_ID=<installation-id>
``` 

**NOTE:** Depending on your setup, you may need to specify a custom Docker image using the `DOCKER_IMG` and `DOCKER_TAG` variables in the command above.

**NOTE:** If you are using AWS ECR as the container registry, you may need to run the following command:

```shell
$ eval $(aws ecr get-login --no-include-email)
```

**NOTE:** If you are running OpenFaaS on top of Minikube, you may need to configure OpenFaaS to use the `IfNotPresent` image pull policy, as well as to run the following command:

```shell
$ eval $(minikube docker-env)
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
    approving_team_handles:
    - "<id-or-name-or-slug>"
    - "<id-or-name-or-slug>"
    - "<id-or-name-or-slug>"
    approval_mode: "<approval-mode>"
    labels:
    - "<label>"
    - "<label>"
    force_approval: false # or true!
``` 

Each item under `pull_request_approval_rules` represents how approval for PRs made to specific target branches should work, according to the following table:

| Field | Description |
|----------------|-------------|
| `regex` | Regular expression to match the body of the pull request against. If matched, approval from each listed team will be required. |
| `approving_team_handles` | The list of approving teams, in the form of IDs, names or slugs. |
| `approval_mode` | One of `require_any` or `require_all`.
| `labels`  | The set of labels to apply to the pull request. Labels are prefixed with the `github-team-approver/` prefix.  |
| `force_approval` | Whether to automatically approve PRs matching the regular expression without waiting for review.

A live example of a configuration file can be seen [here](https://github.com/form3tech/application-versions/blob/develop/.github/GITHUB_TEAM_APPROVER.yaml).

#### Remarks

* Each team listed under `approving_team_handles` should have "Read" access (at least) to the repository.
* If the `target_branches` field is omitted or left empty, the specified rules are applied to all PRs regardless of the target branch.
* PRs made against branches for which no rules are defined are automatically marked as approved.
* PRs made against branches for which rules are defined **MUST** match at least one rule to be approved.
