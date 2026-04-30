# tcctl Definition of Done

## Purpose

This document defines what must be true before `tcctl` can be considered implementation-complete for the MVP operator CLI.

The checklist combines command-line tool best practices with the touch-connect product contract:

- `tcctl` is a human control surface.
- `tcctl` does not execute AI work.
- `tcctl` does not replace `tc-worker`.
- `tcctl` does not write storage or JetStream directly.
- all accepted CLI-visible state comes from `tc-control` responses backed by server records or projections.
- approval commands preserve human accountability.
- JSON output is stable enough for automation.

## Completion Gate

`tcctl` is not done until all items below are true:

- command groups, flags, arguments, help text, and exit codes are stable.
- every command maps to `tc-control` Control API, not storage or broker internals.
- inputs are validated before requests are sent.
- human output is readable and JSON output is stable and exact.
- approval commands preserve actor identity and approval hash.
- retry, DLQ replay, and scenario commands are explicit and auditable.
- config is environment-driven with flag override and safe redaction.
- command parsing, request mapping, output, error, and scenario tests are repeatable.

## 1. Command Model DoD

Done means:

- command groups match the implementation contract.
- every command has a stable name, args contract, flags contract, examples, and help text.
- command aliases are either documented or not supported.
- destructive or protected commands require explicit intent flags when appropriate.
- commands never depend on interactive prompts for required automation paths.
- `--help` works for root, group, and command levels.
- `--version` reports CLI version, build commit when available, and compatible API version.

Minimum command groups:

```text
tcctl server
tcctl endpoint
tcctl task
tcctl message
tcctl artifact
tcctl approval
tcctl dlq
tcctl scenario
```

Tests must cover:

- root help.
- group help.
- command help.
- invalid command.
- missing required args.
- unknown flags.
- version output.

## 2. Control API Mapping DoD

Done means:

- every command calls `tc-control` Control API.
- no command writes storage directly.
- no command writes JetStream directly.
- no command executes local task work.
- request and response types are versioned.
- client timeout is configurable.
- control/server domain errors map to stable CLI exit codes.
- retries are bounded and never duplicate protected commands silently.

Required mapping coverage:

- server health.
- server version.
- endpoint list, inspect, and capabilities.
- task create, status, history, retry, and cancel.
- message send, inspect, and task history.
- artifact list, inspect, and finalize.
- approval list, inspect, approve, and reject.
- DLQ list, inspect, and replay.
- canonical scenario run and verify.

Tests must cover:

- request path or method mapping.
- request body mapping.
- Control API compatibility failure.
- control success response.
- control/server contract rejection.
- unavailable control plane.
- timeout.

## 3. Input Validation DoD

Done means:

- command args are validated before API calls.
- ids and refs use exact server-facing values.
- message creation maps to canonical envelope fields.
- delivery class is validated against known values.
- `readback_required` is explicit for critical handoff commands.
- protected side effect intent requires idempotency and protected scope fields or server policy delegation.
- file input, stdin input, and inline input behavior are documented when supported.
- large payloads are not silently inlined into message bodies.

Required message fields:

```text
target_capability
payload.summary
payload.body
payload.references[]
constraints[]
delivery_class
readback_required
```

Tests must cover:

- invalid id.
- invalid delivery class.
- missing target capability.
- missing payload summary.
- oversized inline payload.
- protected side effect missing idempotency data.

## 4. Output and Exit Code DoD

Done means:

- default human output is concise and useful.
- `--json` output preserves exact ids and refs.
- JSON output has stable field names.
- success writes expected output to stdout.
- diagnostics and errors write to stderr.
- no command leaks secrets, credentials, full prompts, or raw artifact bodies.
- pagination output is stable.
- exit codes match the implementation contract.

Required exit code classes:

```text
0 success
1 command or validation failure
2 server rejected the request by domain contract
3 transport or server unavailable
4 authentication or authorization failure
```

Tests must cover:

- human output shape.
- JSON output shape.
- error output to stderr.
- exit code mapping.
- no secret in output.

## 5. Approval and Protected Operation DoD

Done means:

- approve and reject require actor identity.
- approval decision includes current `approval_hash`.
- expired approval requests cannot be approved.
- self-approval policy is explicit and enforced by server response.
- approval decision does not mutate original message.
- protected commands show enough context for a human decision without exposing secrets.
- approval command output includes decision id and server-accepted state.

Tests must cover:

- approve request payload.
- reject request payload.
- missing actor identity.
- approval hash mismatch.
- expired approval.
- server rejection.

## 6. Retry, DLQ, and Scenario DoD

Done means:

- task retry is explicit.
- DLQ replay is explicit.
- replay output shows the server-accepted replay request or result.
- retry and replay never fake success locally.
- canonical scenario run creates only server-accepted records.
- fake-worker mode, if implemented, is explicit and cannot be mistaken for real worker execution.
- canonical scenario verify reads server state and validates the MVP completion criteria.

Scenario verify must check:

- at least one readback-required handoff succeeded.
- at least two artifact versions exist.
- validation report includes SonarQube gate pass.
- approval decision is approved.
- side effect execution record is succeeded.
- side effect uniqueness boundary executed once.
- final task state is completed.

Tests must cover:

- retry request mapping.
- DLQ replay request mapping.
- canonical scenario run payloads.
- canonical scenario verify success.
- canonical scenario verify failure.

## 7. Config and Environment DoD

Done means:

- command flags override environment variables.
- environment variables override local config file.
- local config file overrides built-in defaults.
- required config is validated before API calls.
- request timeout uses typed duration parsing.
- output format is configurable.
- secrets are redacted from diagnostics.
- config inspection, if implemented, redacts sensitive values.

Required environment variable namespace:

```text
TCCTL_CONTROL_URL
TCCTL_ACTOR_ID
TCCTL_WORKSPACE_ID
TCCTL_OUTPUT_FORMAT
TCCTL_REQUEST_TIMEOUT
TCCTL_AUTH_TOKEN
TCCTL_CONFIG_FILE
```

Tests must cover:

- flag precedence.
- environment precedence.
- config file loading.
- missing control url.
- invalid timeout.
- secret redaction.

## 8. Security DoD

Done means:

- actor identity is attached to protected requests.
- auth token or credential handling is explicit.
- compatible Control API version is verified before executing schema-dependent commands.
- credentials are never printed.
- object ids are not trusted as authorization proof.
- server-side authorization failure maps to exit code `4`.
- response rendering avoids excessive data exposure.
- approval and replay commands require explicit operator intent.
- TLS or trusted network assumptions are documented.

## 9. Observability and Release DoD

Done means:

- optional verbose mode shows request id, server url, and timing without secrets.
- logs or debug output can include `message_id`, `task_id`, `attempt_ref`, `endpoint_ref`, and `correlation_id` when available.
- every command has command parsing tests.
- every command has request mapping tests.
- JSON output has snapshot or schema tests.
- server rejection and unavailable server tests pass.
- Control API compatibility tests pass.
- canonical scenario tests pass.
- generated docs or help output are current if generation is used.

## Reference Baselines

- POSIX utility argument syntax and utility syntax guidelines: https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap12.html
- Cobra documentation for Go CLI applications: https://cobra.dev/docs/
- Go flag package documentation: https://pkg.go.dev/flag
- Twelve-Factor App config principle: https://www.12factor.net/config
- OWASP API Security project: https://owasp.org/API-Security/

## Related Contracts

- `tcctl/docs/implementation-contract.md`
- `tcctl/docs/implementation-task-list.md`
- `tc-control/docs/implementation-contract.md`
- `tc-server/docs/implementation-contract.md`
- `docs/active/contracts/ai-communication-layer-contract.md`
- `docs/active/product/mvp-canonical-scenario.md`
- `docs/active/contracts/message-task-state-model.md`
- `docs/active/contracts/approval-identity-policy.md`
- `docs/active/contracts/delivery-semantics.md`
