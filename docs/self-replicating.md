[← Back](README.md) | [English](self-replicating.md) | [Japanese](self-replicating-ja.md)

# Self-Replicating Credentials

## What is self-replicating?

key-rest prevents the LLM agent from **seeing** credential values, but does not prevent the agent from **using** credentials to generate new ones via authenticated API calls. If a service's API allows creating a new credential of the same type using an existing one, the agent could:

1. Call the credential creation endpoint via key-rest
2. Receive the newly created credential in the response body
3. The new credential is **not** in key-rest's credential store, so response masking does not catch it

This is an architectural limitation of the credential proxy model ([issue #9](https://github.com/koteitan/key-rest/issues/9)). We call this property **self-replicating** — whether a credential can create another credential of the same type (key-X -> key-X).

## Per-Service Investigation

### GitHub

| Credential | Self-replicating? | Details |
|---|---|---|
| PAT (classic/fine-grained) | **No** | No API endpoint exists to create PATs. `POST /authorizations` was deprecated in 2020. |
| OAuth token | **No** | Issuing a new token requires the OAuth flow (browser authorization + `client_id` + `client_secret`). None of these are obtainable from the OAuth token itself. |
