#!/usr/bin/env bash
# One-time setup of Azure and GitHub for the deployment of profit-tracker. See docs/deployment.md.
# Run it from the repository root with the private Azure login: direnv exec . scripts/setup-azure.sh
# A second run makes no error and no duplicate.
set -euo pipefail

repo="JRojowski/profit-tracker"
tenant="66d3a25a-71b1-4b08-8a83-e82d607b8a02"
location="polandcentral"
group="rg-profit-tracker"
identity="id-profit-tracker-github"
credential="github-main"
subject="repo:$repo:ref:refs/heads/main"
budget="budget-profit-tracker"

step() { printf '\n== %s\n' "$1"; }

step "Check the Azure login"
account_tenant="$(az account show --query tenantId --output tsv)"
if [ "$account_tenant" != "$tenant" ]; then
	echo "az account show gives the tenant $account_tenant. This is not the private tenant." >&2
	echo "Run the script with the private login: direnv exec . scripts/setup-azure.sh" >&2
	exit 1
fi
subscription="$(az account show --query id --output tsv)"
user="$(az account show --query user.name --output tsv)"
echo "Private account $user, subscription $subscription."

step "Check the GitHub login"
gh repo view "$repo" --json name >/dev/null

step "Register the resource providers"
for provider in Microsoft.App Microsoft.OperationalInsights Microsoft.ManagedIdentity Microsoft.Consumption; do
	az provider register --namespace "$provider" --wait --output none
	echo "$provider registered."
done

step "Create the resource group $group"
az group create --name "$group" --location "$location" --output none
group_id="$(az group show --name "$group" --query id --output tsv)"

step "Create the identity of GitHub Actions"
az identity create --name "$identity" --resource-group "$group" --location "$location" --output none
client_id="$(az identity show --name "$identity" --resource-group "$group" --query clientId --output tsv)"
principal_id="$(az identity show --name "$identity" --resource-group "$group" --query principalId --output tsv)"
if az identity federated-credential show --name "$credential" --identity-name "$identity" \
	--resource-group "$group" --output none 2>/dev/null; then
	echo "The federated credential $credential exists."
else
	az identity federated-credential create --name "$credential" --identity-name "$identity" \
		--resource-group "$group" --issuer "https://token.actions.githubusercontent.com" \
		--subject "$subject" --audiences "api://AzureADTokenExchange" --output none
	echo "The federated credential $credential is created for $subject."
fi

step "Give the identity the role Contributor on $group only"
assigned="$(az role assignment list --assignee "$principal_id" --scope "$group_id" \
	--role Contributor --query "length(@)" --output tsv)"
if [ "$assigned" != "0" ]; then
	echo "The role assignment exists."
else
	# A new identity can take some seconds to appear in Entra ID.
	for attempt in 1 2 3 4 5 6; do
		if az role assignment create --assignee-object-id "$principal_id" --assignee-principal-type ServicePrincipal \
			--role Contributor --scope "$group_id" --output none 2>/dev/null; then
			echo "The role assignment is created."
			break
		fi
		[ "$attempt" = 6 ] && { echo "The role assignment failed." >&2; exit 1; }
		echo "The identity is not ready. Wait 10 seconds."
		sleep 10
	done
fi

step "Create the budget $budget"
if az rest --method get --output none 2>/dev/null \
	--url "https://management.azure.com$group_id/providers/Microsoft.Consumption/budgets/$budget?api-version=2026-06-01"; then
	echo "The budget exists."
else
	az deployment group create --resource-group "$group" --name budget \
		--template-file infra/budget.bicep \
		--parameters startDate="$(date +%Y-%m-01)" email="$user" --output none
	echo "The budget is created. The alerts go to $user."
fi

step "Store the GHCR token as a GitHub secret"
replace="y"
secrets="$(gh secret list --repo "$repo" --json name --jq '.[].name')"
if printf '%s\n' "$secrets" | grep -qx GHCR_TOKEN; then
	read -r -p "The secret GHCR_TOKEN exists. Replace it? [y/N] " replace
fi
if [ "$replace" = "y" ] || [ "$replace" = "Y" ]; then
	echo "Make a classic token with only the scope read:packages, valid for 1 year:"
	echo "https://github.com/settings/tokens/new?scopes=read:packages&description=profit-tracker-ghcr"
	read -r -s -p "Paste the token (the input is hidden): " token
	echo
	# curl reads the header from the input, so that the token is not in the process list.
	headers="$(printf 'Authorization: Bearer %s\n' "$token" |
		curl -sS -D - -o /dev/null -H @- https://api.github.com/user | tr -d '\r')"
	scopes="$(printf '%s\n' "$headers" | grep -i '^x-oauth-scopes:' | cut -d' ' -f2- || true)"
	expires="$(printf '%s\n' "$headers" | grep -i '^github-authentication-token-expiration:' | cut -d' ' -f2- || true)"
	if [ "$scopes" != "read:packages" ]; then
		echo "The token has the scopes '$scopes'. It must have only read:packages." >&2
		exit 1
	fi
	ghcr_user="$(GH_TOKEN="$token" gh api user --jq .login)"
	printf '%s' "$token" | gh secret set GHCR_TOKEN --repo "$repo"
	unset token
	gh variable set GHCR_USERNAME --repo "$repo" --body "$ghcr_user"
	gh variable set GHCR_TOKEN_EXPIRES --repo "$repo" --body "${expires:-never}"
	echo "The secret GHCR_TOKEN is stored for the user $ghcr_user."
fi
echo "The GHCR token expires: $(gh variable get GHCR_TOKEN_EXPIRES --repo "$repo" 2>/dev/null || echo unknown)"

step "Store the Azure IDs as GitHub variables"
gh variable set AZURE_CLIENT_ID --repo "$repo" --body "$client_id"
gh variable set AZURE_TENANT_ID --repo "$repo" --body "$tenant"
gh variable set AZURE_SUBSCRIPTION_ID --repo "$repo" --body "$subscription"

step "Done"
echo "Start the first deployment: gh workflow run deploy.yml --repo $repo"
