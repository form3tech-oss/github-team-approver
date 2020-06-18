pull_request_approval_rules:
- target_branches:
  - master
  rules:
  - approval_mode: require_any
    regex: "- \\[x\\] Yes - this change impacts customers"
    approving_team_handles:
    -  CAB - Foo
    -  CAB - Bar
    labels:
    - needs-cab-approval
  - regex: "- \\[x\\] Yes - this change impacts documentation"
    approving_team_handles:
    -  CAB - Documentation
    labels:
    - needs-doc-approval
  alerts:
  - regex: "- \\[x\\] Yes, this is an emergency release."
    slack_webhook_secret: "KJ2Xy2KhPF6EY_cTn-WqcXHmHXZ982q2J77ydUTuC2u6Vrnd_xaEOiAVJfPFtUbm"
    slack_message: '{"blocks":[{"type":"section","block_id":"section567","text":{"type":"mrkdwn","text":"*PR*: <{{.PullRequest.HTMLURL}}|{{.PullRequest.Title}}> \n *Repo:* {{.Repo.Name}} \n *Branch:* `{{.PullRequest.Base.Ref}}` \n *Author:* {{.PullRequest.User.Login}} \n *Merged By:* {{.PullRequest.MergedBy.Login}} \n\n *Body:* {{.PullRequest.Body}}"},"accessory":{"type":"image","image_url":"{{.PullRequest.User.AvatarURL}}","alt_text":"{{.PullRequest.User.Login}}"}}]}'
