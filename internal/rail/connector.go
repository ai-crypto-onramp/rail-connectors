package rail

import "context"

// Connector is the common interface every rail adapter implements.
type Connector interface {
	// Authorize reserves / authorizes funds on the rail.
	Authorize(ctx context.Context, in Context) (Response, error)
	// Capture settles a previously authorized amount.
	Capture(ctx context.Context, in Context, amount float64) (Response, error)
	// Refund returns a captured / settled amount to the payer.
	Refund(ctx context.Context, in Context, amount float64) (Response, error)
	// GetStatus returns the current status of a payment on the rail.
	GetStatus(ctx context.Context, in Context) (Status, error)
}
