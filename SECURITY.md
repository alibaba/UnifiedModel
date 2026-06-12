# Security Policy

Security reports are welcome and should be handled privately before public disclosure.

## Supported Versions

UModel Open Source is maintained on the main development line until stable release branches are published. Security fixes are accepted there first and should be backported once versioned release branches exist.

| Version | Supported |
|---|---:|
| `main` | Yes |
| Tagged releases | Not yet published |

## Reporting A Vulnerability

Do not open a public issue for a vulnerability.

Preferred reporting paths:

1. Use GitHub private vulnerability reporting if it is enabled for the repository.
2. If private vulnerability reporting is not available, contact the maintainers through the private channel listed by the hosting organization.

Please include:

- Affected commit, branch, or release.
- Reproduction steps.
- Expected and observed behavior.
- Impact assessment.
- Any known workaround.

## Maintainer Response

Maintainers should acknowledge a complete report within 5 business days, triage severity, and coordinate a fix or disclosure plan with the reporter.

## Security Boundaries

Current open-source security defaults:

- `make dev`, Docker, and Compose use `file.memory` local persistence.
- MCP write tools are disabled by default.
- AgentGateway resources expose metadata and templates, not runtime rows.
- The UModel import endpoint (`POST /api/v1/umodel/{workspace}/import`, `umctl umodel import`) reads model packs from operator-provided **server-local filesystem paths** by design, similar to `kubectl apply -f <path>`. It is an operator capability and must not be exposed to untrusted callers. Keep the import endpoint off any internet-facing surface, or front it with authentication/authorization.
- This release does not include multi-tenant authorization or cloud-hosted control plane behavior.

Do not use the local development server as an internet-facing production service without adding authentication, authorization, transport security, rate limits, audit logging, and deployment hardening.
