package transport

import "context"

// OffPeakMessagesPath is the idle-lane inference path; requests carrying a
// ticket are forwarded here instead of the plan endpoint.
const OffPeakMessagesPath = "/api/v1/off-peak/anthropic/v1/messages"

type offPeakTicketKey struct{}

// WithOffPeakTicket marks a request as riding a ready off-peak ticket. The
// upstream call then targets the idle lane and carries the ticket headers.
func WithOffPeakTicket(ctx context.Context, ticketID string) context.Context {
	if ticketID == "" {
		return ctx
	}
	return context.WithValue(ctx, offPeakTicketKey{}, ticketID)
}

// OffPeakTicket returns the ticket id bound to ctx ("" when absent).
func OffPeakTicket(ctx context.Context) string {
	id, _ := ctx.Value(offPeakTicketKey{}).(string)
	return id
}

func upstreamURLFor(ctx context.Context) string {
	if OffPeakTicket(ctx) != "" {
		return InferenceOrigin + OffPeakMessagesPath
	}
	return InferenceOrigin + PlanAnthropicPath
}
