# GitLab mirror and Harbor publish

The operator image is built and pushed to Harbor from GitLab CI on the mirror at
`gitlab.com/dk-raas/dkai/stoat-operator`.

GitHub Actions on `main` runs verification, then mirrors the repository to GitLab.
GitLab CI runs tests, builds the Docker image, and publishes to
`harbor.dataknife.net/library/stoat-operator`.

## Credentials (already configured at org/group level)

No per-repo secrets or CI variables are required for normal operation.

### GitHub — org secret `GITLAB_TOKEN`

The DataKnifeAI organization provides `GITLAB_TOKEN` to repositories that mirror
to GitLab. Verify access for this repo:

```bash
gh api repos/DataKnifeAI/stoat-operator/actions/organization-secrets
```

Expected: `GITLAB_TOKEN` in the `secrets` list.

### GitLab — group variables on `dk-raas/dkai`

Harbor and registry credentials are inherited from the parent group. Verify:

```bash
glab variable list --group dk-raas/dkai
```

## Mirror project

https://gitlab.com/dk-raas/dkai/stoat-operator

Create the empty GitLab project (if missing) before the first mirror push:

```bash
glab repo create dk-raas/dkai/stoat-operator --private --description "Stoat Kubernetes operator"
```
