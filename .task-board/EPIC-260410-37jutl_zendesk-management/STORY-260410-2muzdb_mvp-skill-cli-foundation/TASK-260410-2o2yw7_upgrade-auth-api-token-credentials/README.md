# Upgrade Auth API Token Credentials

## Description
Store organization, email, and API token for Zendesk auth, add cleanup alias, and prepare Basic auth credentials for live API checks.

## Scope
Store organization, email, and API token across keychain and env_or_file config, add auth cleanup alias, and keep backward compatibility where practical.

## Acceptance Criteria
CLI supports set-access with organization email token, auth clean aliases clear-access, stored credentials retain email without printing the secret, and tests plus smoke verification pass.
