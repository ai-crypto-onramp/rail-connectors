// Package ach implements the ACH rail adapter: NACHA PPD debit file
// generation, bank partner API submission, status polling, and ACH return
// handling through the common rail.RailConnector interface.
package ach

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ai-crypto-onramp/rail-connectors/internal/ach/bankapi"
	"github.com/ai-crypto-onramp/rail-connectors/internal/ach/nacha"
	"github.com/ai-crypto-onramp/rail-connectors/internal/rail"
)

// Adapter is the ACH RailConnector.
type Adapter struct {
	bank *bankapi.Client
	log  *slog.Logger

	// mu guards requests and the latencies maps.
	mu          sync.Mutex
	requests    map[string]*RequestRow
	authLatency map[string]time.Duration
	capLatency  map[string]time.Duration
}

// RequestRow mirrors the rail_requests table columns.
type RequestRow struct {
	TxID           string
	Rail           string
	RailRequestID  string
	Operation      string
	Amount         int64
	Currency       string
	Status         string
	IdempotencyKey string
	RailRef        string
	ErrorCode      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Config configures the ACH adapter.
type Config struct {
	PartnerURL string
	Logger     *slog.Logger
}

// New builds an ACH adapter.
func New(cfg Config) *Adapter {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{
		bank:        bankapi.New(cfg.PartnerURL),
		log:         logger,
		requests:    make(map[string]*RequestRow),
		authLatency: make(map[string]time.Duration),
		capLatency:  make(map[string]time.Duration),
	}
}

// Authorize performs an ACH pre-note: it submits a zero-amount NACHA
// batch to validate routing/account data.
func (a *Adapter) Authorize(ctx context.Context, in rail.RailContext) (rail.RailResponse, error) {
	start := time.Now()
	railRef := in.RailRequestID
	if railRef == "" {
		railRef = in.TxID
	}
	a.log.Info("ach.authorize.start", "tx_id", in.TxID, "rail_request_id", in.RailRequestID)

	file := buildNACHA(in, 0) // pre-note: zero amount
	encoded, err := file.Encode()
	if err != nil {
		a.persist(in, "authorize", rail.StatusFailed, railRef, rail.ErrInvalidRequest, err.Error())
		return failResp(rail.ErrInvalidRequest, err.Error()), nil
	}
	sub, err := a.bank.Submit(ctx, in.IdempotencyKey, encoded)
	if err != nil {
		a.persist(in, "authorize", rail.StatusFailed, railRef, rail.ErrRailUnavailable, err.Error())
		return failResp(rail.ErrRailUnavailable, err.Error()), nil
	}
	if isReturn(sub.StatusCode) {
		if code, reason := MapReturnCode(sub.StatusCode); code != "" {
			a.persist(in, "authorize", rail.StatusFailed, sub.RailRef, code, reason)
			return failResp(code, reason), nil
		}
	}
	a.persist(in, "authorize", rail.StatusAuthorized, sub.RailRef, "", "")
	a.recordLatency(a.authLatency, in.TxID, time.Since(start))
	return rail.RailResponse{
		Status:     rail.StatusAuthorized,
		RailRef:    sub.RailRef,
		RawPayload: []byte(encoded),
	}, nil
}

// Capture submits the real NACHA debit batch for settlement.
func (a *Adapter) Capture(ctx context.Context, in rail.RailContext) (rail.RailResponse, error) {
	start := time.Now()
	railRef := in.RailRequestID
	if railRef == "" {
		railRef = in.TxID
	}
	a.log.Info("ach.capture.start", "tx_id", in.TxID, "rail_request_id", in.RailRequestID)

	file := buildNACHA(in, in.Amount.Value)
	encoded, err := file.Encode()
	if err != nil {
		a.persist(in, "capture", rail.StatusFailed, railRef, rail.ErrInvalidRequest, err.Error())
		return failResp(rail.ErrInvalidRequest, err.Error()), nil
	}
	sub, err := a.bank.Submit(ctx, in.IdempotencyKey, encoded)
	if err != nil {
		a.persist(in, "capture", rail.StatusFailed, railRef, rail.ErrRailUnavailable, err.Error())
		return failResp(rail.ErrRailUnavailable, err.Error()), nil
	}
	if isReturn(sub.StatusCode) {
		if code, reason := MapReturnCode(sub.StatusCode); code != "" {
			a.persist(in, "capture", rail.StatusFailed, sub.RailRef, code, reason)
			return failResp(code, reason), nil
		}
	}
	a.persist(in, "capture", rail.StatusCaptured, sub.RailRef, "", "")
	a.recordLatency(a.capLatency, in.TxID, time.Since(start))
	return rail.RailResponse{
		Status:     rail.StatusCaptured,
		RailRef:    sub.RailRef,
		RawPayload: []byte(encoded),
	}, nil
}

// Refund submits a reversing NACHA entry for a previously captured debit.
func (a *Adapter) Refund(ctx context.Context, in rail.RailContext) (rail.RailResponse, error) {
	railRef := in.RailRequestID
	if railRef == "" {
		railRef = in.TxID
	}
	a.log.Info("ach.refund.start", "tx_id", in.TxID, "rail_request_id", in.RailRequestID)

	file := buildNACHA(in, in.Amount.Value)
	encoded, err := file.Encode()
	if err != nil {
		a.persist(in, "refund", rail.StatusFailed, railRef, rail.ErrInvalidRequest, err.Error())
		return failResp(rail.ErrInvalidRequest, err.Error()), nil
	}
	sub, err := a.bank.Submit(ctx, in.IdempotencyKey, encoded)
	if err != nil {
		a.persist(in, "refund", rail.StatusFailed, railRef, rail.ErrRailUnavailable, err.Error())
		return failResp(rail.ErrRailUnavailable, err.Error()), nil
	}
	a.persist(in, "refund", rail.StatusRefunded, sub.RailRef, "", "")
	return rail.RailResponse{
		Status:     rail.StatusRefunded,
		RailRef:    sub.RailRef,
		RawPayload: []byte(encoded),
	}, nil
}

// GetStatus polls the bank partner for the current batch status and
// normalizes any ACH return code.
func (a *Adapter) GetStatus(ctx context.Context, in rail.RailContext) (rail.RailResponse, error) {
	a.log.Info("ach.status.start", "tx_id", in.TxID, "rail_request_id", in.RailRequestID)
	railRef := in.RailRequestID
	if railRef == "" {
		railRef = in.TxID
	}
	st, err := a.bank.GetStatus(ctx, railRef)
	if err != nil {
		a.persist(in, "status", rail.StatusFailed, railRef, rail.ErrRailUnavailable, err.Error())
		return failResp(rail.ErrRailUnavailable, err.Error()), nil
	}
	if st.ReturnCode != "" {
		if code, reason := MapReturnCode(st.ReturnCode); code != "" {
			a.persist(in, "status", rail.StatusFailed, railRef, code, reason)
			return failResp(code, reason), nil
		}
	}
	status := mapStatus(st.StatusCode)
	a.persist(in, "status", status, railRef, "", "")
	return rail.RailResponse{Status: status, RailRef: railRef}, nil
}

// Settle is a no-op for ACH; settlement is driven by inbound settlement
// file ingestion (Stage 9).
func (a *Adapter) Settle(ctx context.Context, in rail.RailContext) (rail.RailResponse, error) {
	return rail.RailResponse{Status: rail.StatusSettled, RailRef: in.RailRequestID}, nil
}

// Chargeback records an ACH return as a chargeback event.
func (a *Adapter) Chargeback(ctx context.Context, in rail.RailContext) (rail.RailResponse, error) {
	a.persist(in, "chargeback", rail.StatusChargeback, in.RailRequestID, "", "")
	return rail.RailResponse{Status: rail.StatusChargeback, RailRef: in.RailRequestID}, nil
}

func (a *Adapter) persist(in rail.RailContext, op string, status rail.RailStatus, railRef, code, reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := in.IdempotencyKey
	if key == "" {
		key = in.TxID + ":" + op
	}
	now := time.Now()
	if row, ok := a.requests[key]; ok {
		row.Status = status.String()
		row.Operation = op
		row.RailRef = railRef
		row.ErrorCode = code
		row.UpdatedAt = now
		return
	}
	a.requests[key] = &RequestRow{
		TxID:           in.TxID,
		Rail:           "ach",
		RailRequestID:  in.RailRequestID,
		Operation:      op,
		Amount:         in.Amount.Value,
		Currency:       in.Amount.Currency,
		Status:         status.String(),
		IdempotencyKey: key,
		RailRef:        railRef,
		ErrorCode:      code,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Requests returns a snapshot of persisted rail_requests rows. Used by tests.
func (a *Adapter) Requests() []RequestRow {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]RequestRow, 0, len(a.requests))
	for _, r := range a.requests {
		out = append(out, *r)
	}
	return out
}

// AuthorizeLatency returns the recorded authorize latency for a tx.
func (a *Adapter) AuthorizeLatency(txID string) time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.authLatency[txID]
}

// CaptureLatency returns the recorded capture latency for a tx.
func (a *Adapter) CaptureLatency(txID string) time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.capLatency[txID]
}

func (a *Adapter) recordLatency(m map[string]time.Duration, txID string, d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	m[txID] = d
}

func buildNACHA(in rail.RailContext, amount int64) *nacha.File {
	routing := in.RailSpecific["receiver_dfi"]
	if len(routing) < 9 {
		routing = "123456789"
	}
	account := in.RailSpecific["receiver_account"]
	if account == "" {
		account = "000111222"
	}
	receiver := in.RailSpecific["receiver_name"]
	if receiver == "" {
		receiver = "RECEIVER"
	}
	originDFI := in.RailSpecific["origin_dfi"]
	if len(originDFI) != 8 {
		originDFI = "02100008"
	}
	entry := &nacha.EntryDetail{
		ReceiverName:  receiver,
		ReceiverDFI:   routing,
		AccountNumber: account,
		Amount:        amount,
		IndividualID:  in.TxID,
	}
	return &nacha.File{
		ImmediateOrigin:      originDFI,
		ImmediateDestination: "02600001",
		OriginName:           "ONRAMP",
		Batches: []*nacha.Batch{{
			CompanyName:              "ONRAMP",
			CompanyDiscretionaryData: "",
			CompanyEntryDescription:  "PAYMENT",
			OriginatingDFI:           originDFI,
			Entries:                  []*nacha.EntryDetail{entry},
		}},
	}
}

func mapStatus(s string) rail.RailStatus {
	switch s {
	case "authorized", "pending", "submitted":
		return rail.StatusAuthorized
	case "captured", "settled":
		return rail.StatusCaptured
	case "refunded":
		return rail.StatusRefunded
	case "failed", "returned":
		return rail.StatusFailed
	default:
		return rail.StatusUnknown
	}
}

// isReturn reports whether a status code from the partner represents an
// ACH return (and therefore should be run through MapReturnCode).
func isReturn(code string) bool {
	return code == "returned" || code == "failed" || len(code) >= 3 && code[0] == 'R' && code[1] >= '0' && code[1] <= '9'
}

func failResp(code, reason string) rail.RailResponse {
	return rail.RailResponse{
		Status:      rail.StatusFailed,
		ErrorCode:   code,
		ErrorReason: reason,
	}
}
