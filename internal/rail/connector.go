package rail

import "context"

// RailConnector is the common six-method interface every rail adapter
// implements.
type RailConnector interface {
	Authorize(ctx context.Context, in RailContext) (RailResponse, error)
	Capture(ctx context.Context, in RailContext) (RailResponse, error)
	Refund(ctx context.Context, in RailContext) (RailResponse, error)
	GetStatus(ctx context.Context, in RailContext) (RailResponse, error)
	Settle(ctx context.Context, in RailContext) (RailResponse, error)
	Chargeback(ctx context.Context, in RailContext) (RailResponse, error)
}
