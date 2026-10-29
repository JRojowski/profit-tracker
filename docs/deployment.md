# Deployment to Azure

The tool runs in Azure Container Apps in the private Azure subscription of the user. ADR 0006 gives the reasons.

Warning: until the sign-in is done (ticket #35), the address of the tool is public and has no sign-in. Do not import real XTB exports before that.

## Parts

| Part | Where |
|---|---|
| Image | `ghcr.io/jrojowski/profit-tracker:<commit SHA>`, private |
| Infrastructure | `infra/main.bicep`, applied at each deployment |
| Budget | `infra/budget.bicep`, applied one time by the setup script |
| One-time setup | `scripts/setup-azure.sh` |
| Deployment | `.github/workflows/deploy.yml` |
| Resource group | `rg-profit-tracker` in Poland Central |

## Use the private Azure login

The gitignored `.envrc` selects the private Azure login. Run each Azure command in this repository with `direnv exec .`.

Before each change in Azure, check the account:

```bash
direnv exec . az account show --query user.name --output tsv
```

The result must be the private account, not the work account.

## Do the one-time setup

Do this step one time. A second run makes no error and no duplicate.

1. Log in to GitHub with `gh auth login`.
2. Run the setup script from the repository root:

   ```bash
   direnv exec . scripts/setup-azure.sh
   ```

3. When the script asks for the GHCR token, open the link that it shows.
4. Make a classic token with only the scope `read:packages`. Set the expiry to 1 year.
5. Paste the token. The script stores it as the GitHub secret `GHCR_TOKEN`. It saves the token to no file.
6. Write down the expiry date that the script shows. The GitHub variable `GHCR_TOKEN_EXPIRES` also holds it.

The script does these steps:

- It stops when the Azure login is not the private tenant.
- It registers the resource providers.
- It creates the resource group `rg-profit-tracker`.
- It creates the identity `id-profit-tracker-github` with a federated credential for the branch `main`. GitHub Actions signs in to Azure with this identity, with no password.
- It gives this identity the role Contributor on the resource group only.
- It creates a budget of 20 PLN each month, with e-mail alerts at 50%, 80% and 100%.
- It stores the GHCR token and the Azure IDs in GitHub.

## Deploy

Each push to `main` with a green CI starts the workflow `Deploy`. It does these steps:

1. It builds the image and pushes it to GHCR. The tag is the commit SHA.
2. It signs in to Azure with OIDC.
3. It applies `infra/main.bicep` with the new tag.
4. It checks that `GET /api/health` returns 200. If not, the deployment is red.
5. It checks that the new revision is ready. If not, the old revision still gets the traffic, and the deployment is red.

A CI run of a pull request does not deploy. A manual deployment runs only from the branch `main`.

To deploy again with no new commit, run:

```bash
gh workflow run deploy.yml
```

To deploy an earlier commit, give its SHA. The workflow does not run the tests for this commit. Use only a commit of `main` with a green CI:

```bash
gh workflow run deploy.yml -f sha=<commit SHA>
```

## Find the address

The summary of each deployment run shows the address. You can also get it with this command:

```bash
direnv exec . az containerapp show --name profit-tracker --resource-group rg-profit-tracker \
  --query properties.configuration.ingress.fqdn --output tsv
```

## Check the scale to zero

When nobody uses the tool for about 5 minutes, it has 0 replicas. Show the replicas with this command:

```bash
direnv exec . az containerapp replica list --name profit-tracker --resource-group rg-profit-tracker --output table
```

The first request after a pause starts a new replica. This takes some seconds.

## Data

In this version, the database is on the disk of the replica. When the replica stops, the database goes away. Ticket #34 adds Litestream, which keeps a copy in Blob Storage.
