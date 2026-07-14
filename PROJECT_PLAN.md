# Project Plan — Rail Connectors

Rail Connectors is the fiat-side boundary of the on-ramp: a single Go module
that builds one deployable binary per rail family (`card`, `ach`, `sepa`, `pix`,
`upi`), all behind a common `RailConnector` interface. This plan decomposes the
README spec into ordered implementation stages that incrementally build the
interface, each rail adapter, the cross-cutting resilience / inbound / settlement
machinery, and finally the test/coverage/Docker harness.

## Stage 1

### Goal

Define the common `RailConnector` Go interface, shared domain types
(`RailContext`, `RailResponse`, `RailStatus`), the normalized error taxonomy, and
the rail-family selection bootstrap (`RAIL_FAMILY`) so every later stage can
build against a stable contract.

### Tasks

- [x] Add `internal/rail/types.go` with `RailContext`, `RailResponse`,
      `RailStatus` constants, and `IdempotencyKey` derivation helper
      (`<tx_id>:<operation>:<attempt>`).
- [x] Add `internal/rail/connector.go` with the six-method `RailConnector`
      interface.
- [x] Add `internal/rail/errors.go` with the normalized error code taxonomy
      (`INSUFFICIENT_FUNDS`, `DO_NOT_HONOR`, `FRAUD_DECLINE`,
      `EXPIRED_INSTRUMENT`, `INVALID_REQUEST`, `RAIL_UNAVAILABLE`,
      `SETTLEMENT_BREAK`) and a `RailError` type carrying code + reason.
- [x] Add `internal/rail/registry.go` with a factory map keyed on
      `RAIL_FAMILY` and a `New(ctx, family, cfg) (RailConnector, error)`
      entrypoint; unknown family returns `INVALID_REQUEST`.
- [x] Wire `cmd/rail-connector/main.go` to read env vars, build the connector,
      and fail fast on missing required config for the active family.
- [x] Add structured logging scaffold (zerolog or slog) emitting `tx_id`,
      `rail`, `rail_request_id` on every interface call wrapper.

### Acceptance criteria

- `go build ./...` and `go vet ./...` pass.
- A unit test constructs the registry with a stub connector for each family and
  asserts `New` returns the right type given `RAIL_FAMILY`.
- Idempotency key helper is unit-tested for stability across replays.
- `RAIL_FAMILY=invalid` produces a clear `INVALID_REQUEST` error at startup.

## Stage 2

### Goal

Implement the **Card** adapter that delegates Authorize/Capture/Refund/Status
to a configurable card processor (Stripe or Adyen, selected via
`RAIL_CARD_PROCESSOR`) and exposes the rail through the common interface.

### Tasks

- [ ] Add `internal/card/adapter.go` implementing `RailConnector`.
- [ ] Add `internal/card/stripe/` client: token-based auth, payment intents,
      captures, refunds, payment status retrieval.
- [ ] Add `internal/card/adyen/` client with the equivalent operations against
      Adyen's `/payments`, `/payments/{id}/captures`, `/payments/{id}/refunds`,
      `/payments/{id}` endpoints.
- [ ] Map processor decline codes onto the normalized error taxonomy in
      `internal/card/errors.go`.
- [ ] Persist one row per outbound call in `rail_requests` (status,
      `idempotency_key`, `rail_ref`, `error_code`).
- [ ] Emit structured logs and Prometheus `rail_authorize_latency` /
      `rail_capture_latency` metrics.

### Acceptance criteria

- Both Stripe and Adyen clients are selectable via `RAIL_CARD_PROCESSOR`.
- httptest-based unit tests assert request payloads, idempotency headers, and
  normalized error mapping for each processor.
- `rail_requests` row is inserted for each call with the correct
  `idempotency_key`.
- `go test ./internal/card/...` passes with `go test -race`.

## Stage 3

### Goal

Implement the **ACH** adapter: NACHA file generation, bank partner API
submission for batches, status polling, and ACH return handling through the
common interface.

### Tasks

- [ ] Add `internal/ach/adapter.go` implementing `RailConnector`.
- [ ] Add `internal/ach/nacha/` encoder for file header, batch header, entry
      detail, batch control, and file control records (PPD debits).
- [ ] Add `internal/ach/bankapi/` client submitting NACHA files and querying
      batch status against `RAIL_ACH_PARTNER_URL`.
- [ ] Implement `Authorize` as an ACH pre-note, `Capture` as batch submission,
      `Refund` as a reversing entry, `GetStatus` as batch status polling.
- [ ] Map ACH return codes (R01 NSF, R02 closed account, R10 customer advice,
      etc.) onto the normalized taxonomy in `internal/ach/errors.go`.
- [ ] Persist `rail_requests` rows and emit `rail_authorize_latency` /
      `rail_capture_latency` metrics.

### Acceptance criteria

- Generated NACHA files pass a structural validator (file/batch control totals
  balance, record counts match) in unit tests.
- Bank API client unit tests cover submission and status polling happy path plus
  return-code normalization.
- `go test ./internal/ach/... -race` passes.

## Stage 4

### Goal

Implement the **SEPA** adapter: ISO20022 `pain.001` initiation to the SEPA
Instant gateway, `pain.002` status consumption, and `camt.053`/`camt.054`
settlement/debit reconciliation through the common interface.

### Tasks

- [ ] Add `internal/sepa/adapter.go` implementing `RailConnector`.
- [ ] Add `internal/sepa/iso20022/` builders for `pain.001.001.09` payment
      initiation messages.
- [ ] Add `internal/sepa/gateway/` client using mTLS (`RAIL_SEPA_MTLS_CERT`)
      for `pain.001` submission and `pain.002` status polling.
- [ ] Implement `Authorize` as `pain.001` submission, `Capture` as the gateway
      confirmation, `Refund` as a reverse `pain.001`, `GetStatus` as
      `pain.002` polling.
- [ ] Map SEPA reason codes (`AC01`, `AM04`, `NOAS`, etc.) onto the normalized
      taxonomy in `internal/sepa/errors.go`.
- [ ] Persist `rail_requests` and emit metrics.

### Acceptance criteria

- Generated `pain.001` messages validate against the ISO20022 schema in unit
  tests.
- Gateway client unit tests cover happy path, mTLS config, and reason-code
  normalization.
- `go test ./internal/sepa/... -race` passes.

## Stage 5

### Goal

Implement the **PIX** adapter against Banco Central do Brasil SPI (Sistema de
Pagamentos Instantâneos), including DICT key resolution, instant payment
initiation, status, and refund through the common interface.

### Tasks

- [ ] Add `internal/pix/adapter.go` implementing `RailConnector`.
- [ ] Add `internal/pix/spi/` client for DICT key resolution and payment
      endpoints (`/v1/pix/payments`, status, refund).
- [ ] Implement `Authorize`/`Capture` as a single instant payment, `Refund` as
      a PIX refund, `GetStatus` as payment status.
- [ ] Map PIX return reasons onto the normalized taxonomy in
      `internal/pix/errors.go`.
- [ ] Persist `rail_requests` and emit metrics.

### Acceptance criteria

- SPI client unit tests cover DICT resolution, payment initiation, status, and
  refund happy paths plus error normalization.
- `go test ./internal/pix/... -race` passes.

## Stage 6

### Goal

Implement the **UPI** adapter against NPCI UPI Collect / Intent APIs, including
payment status, refund, and chargeback flows through the common interface.

### Tasks

- [ ] Add `internal/upi/adapter.go` implementing `RailConnector`.
- [ ] Add `internal/upi/npci/` client for UPI Collect request, status, refund,
      and dispute endpoints.
- [ ] Implement `Authorize` as UPI Collect initiation, `Capture` as collect
      confirmation, `Refund` as UPI refund, `Chargeback` as dispute recording,
      `GetStatus` as collect status.
- [ ] Map NPCI response codes (`00`, `ZP`, `ZD`, etc.) onto the normalized
      taxonomy in `internal/upi/errors.go`.
- [ ] Persist `rail_requests`, `rail_chargebacks` rows and emit
      `rail_chargeback_rate` metrics.

### Acceptance criteria

- NPCI client unit tests cover collect, status, refund, and dispute flows plus
  error normalization.
- `rail_chargebacks` rows are inserted for chargeback events.
- `go test ./internal/upi/... -race` passes.

## Stage 7

### Goal

Add per-rail circuit breaker and retry-with-jitter middleware that wraps every
adapter's outbound calls, returning a normalized `RAIL_UNAVAILABLE` error when
the circuit is open and retrying transient failures idempotently.

### Tasks

- [ ] Add `internal/rail/circuit/` breaker scoped to `<rail>:<endpoint>` with
      `CIRCUIT_MAX_FAILURES` threshold and half-open probe.
- [ ] Add `internal/rail/retry/` exponential backoff + jitter reusing the
      outbound `IdempotencyKey`, capped by `RETRY_MAX_ATTEMPTS`.
- [ ] Wrap outbound calls in `internal/rail/middleware.go` so every adapter
      benefits transparently.
- [ ] Emit `rail_circuit_open` Prometheus metric per rail.
- [ ] Unit-test breaker open/half-open/closed transitions and retry behavior
  against a stub client.

### Acceptance criteria

- A failing stub client trips the breaker after `CIRCUIT_MAX_FAILURES` and
  subsequent calls return `RAIL_UNAVAILABLE` without hitting the stub.
- Retry middleware replays transient 5xx with the same idempotency key up to
  `RETRY_MAX_ATTEMPTS` and then surfaces the normalized error.
- `go test ./internal/rail/... -race` passes.

## Stage 8

### Goal

Implement inbound webhook receivers for each rail (`POST /webhooks/card`,
`/webhooks/ach`, `/webhooks/sepa`, `/webhooks/pix`, `/webhooks/upi`) with
signature verification and dispatch to per-rail handlers.

### Tasks

- [x] Add `internal/webhooks/server.go` HTTP mux mounting the five receiver
      paths.
- [x] Add `internal/webhooks/verify.go` HMAC verifier against
      `RAIL_WEBHOOK_SECRET`; reject with `401` on mismatch.
- [x] Add per-rail handlers that decode the payload, update `rail_requests` /
      `rail_chargebacks`, and translate rail-specific events to the common
      `RailStatus`.
- [x] Emit `rail.chargeback.received` events for dispute payloads to the event
      bus and audit-event-log.
- [x] Unit-test each receiver with signed and unsigned payloads.

### Acceptance criteria

- Unsigned or bad-signature payloads return `401` and do not mutate state.
- Each receiver's happy path updates the correct row and emits the right
  downstream event.
- `go test ./internal/webhooks/... -race` passes.

## Stage 9

### Goal

Implement scheduled settlement file ingestion for each rail (NACHA summary
CSV, ISO20022 `camt.053`/`camt.054`, Banco Central SPI reports, NPCI settlement
files), match entries to `rail_requests`, and emit settlement + audit events.

### Tasks

- [x] Add `internal/settlement/scheduler.go` pulling files from SFTP / API on a
      configurable cadence per rail.
      *(Simplified: in-memory `settlement.Tracker` instead of SFTP/API.)*
- [ ] Add per-rail parsers in `internal/settlement/{nacha,iso20022,spi,npci}/`.
- [x] For each parsed entry: match to a `rail_requests` row, insert a
      `rail_settlements` row, update the request status to `Settled`.
      *(Simplified: `Store.AddSettle` matches by `payment_id` in memory.)*
- [x] Emit `rail.settlement.completed` to Reconciliation and audit-event-log.
      *(Simplified: emitted via in-memory `audit.Sink`.)*
- [ ] Unmatched entries emit a `SETTLEMENT_BREAK` normalized error and an alert
      event.
- [ ] Unit-test parsers with sample fixtures and the matcher against a seeded
      `rail_requests` table (use sqlmock or an in-memory pg).

### Acceptance criteria

- All four parsers correctly populate `rail_settlements` rows from sample
  fixtures.
- Matched entries produce `rail.settlement.completed` events; unmatched entries
  produce `SETTLEMENT_BREAK`.
- `go test ./internal/settlement/... -race` passes.

## Stage 10

### Goal

Harden the service for delivery: comprehensive tests, race-safe
concurrency, lint/gofmt gates, Prometheus metrics endpoint, Docker image, and
CI pipeline alignment with the README.

### Tasks

- [x] Raise package coverage; report via `codecov.yml`
      and close gaps flagged by `go test -cover`.
- [x] Ensure `go test -race ./...` passes and add race flag to CI.
- [ ] Add `make lint` (golangci-lint) and `make fmt-check` targets; wire into
      CI.
- [ ] Add `/metrics` Prometheus endpoint exposing all `rail_*` metrics.
- [x] Finalize `Dockerfile` multi-stage build producing a minimal image per
      `RAIL_FAMILY` build arg.
      *(Simplified: single-stage build of `cmd/rail-connectors`; no per-family
      build arg.)*
- [ ] Add a `docker-compose.test.yml` smoke test that boots Postgres + the
      binary and exercises one authorize/capture round-trip.

### Acceptance criteria

- `go test -race -cover ./...` passes and coverage reported to Codecov.
- `make lint` and `gofmt -l .` produce no diff.
- `docker build` succeeds and the resulting image passes the smoke test.
- CI workflow runs build, lint, race tests, coverage upload, and Docker build
  green on `main`.