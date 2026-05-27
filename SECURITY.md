# Security Policy

## Supported versions

Security fixes are applied on a best-effort basis to:

| Version | Supported |
| --- | --- |
| `master` (latest) | :white_check_mark: |
| Older releases | :x: |

If a vulnerability affects an unsupported version, upgrade to the latest release before requesting a fix.

## Reporting a vulnerability

Please do not open public issues for suspected security vulnerabilities.

Use GitHub Security Advisories for private reporting:

1. Go to the repository Security tab.
2. Click "Report a vulnerability".
3. Share detailed reproduction steps and impact.

If private reporting through GitHub is unavailable, contact maintainers through repository ownership channels and include "Security" in the subject.

## What to include in your report

To help maintainers triage quickly, include:

- A clear description of the vulnerability and potential impact.
- Reproduction steps or a proof of concept.
- Affected commit/tag/version.
- Environment details (OS, Go version, Docker version, deployment mode).
- Any suggested remediation (optional).

Do not include production secrets, credentials, or personal data in your report.

## Response and disclosure process

Maintainers aim to:

- Acknowledge new reports within 5 business days.
- Triage severity and affected scope.
- Work on a fix and coordinate release timing.
- Credit the reporter unless anonymous disclosure is requested.

Please allow time for a fix before public disclosure. Coordinated disclosure helps protect users.

## Scope notes for this project

This service handles sensitive one-time messages and Vault tokens. Reports are especially valuable for issues related to:

- Secret leakage (logs, responses, storage, transport).
- Token misuse or replay that breaks one-time read guarantees.
- Authentication/authorization bypass in Vault interactions.
- TLS misconfiguration that can expose secrets in transit.
- File upload handling weaknesses (size limits, validation, processing).

## Non-security bugs

For non-security bugs, use the public issue templates in [bug_report.yml](.github/ISSUE_TEMPLATE/bug_report.yml) and [feature_request.yml](.github/ISSUE_TEMPLATE/feature_request.yml).
