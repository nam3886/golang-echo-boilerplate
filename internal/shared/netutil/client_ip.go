// Package netutil provides network-related context helpers.
package netutil

import "context"

type contextKey string

const clientIPKey contextKey = "client_ip"

// WithClientIP stores the client IP in the context.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, clientIPKey, ip)
}

// GetClientIP extracts the client IP from context.
func GetClientIP(ctx context.Context) string {
	if ip, ok := ctx.Value(clientIPKey).(string); ok {
		return ip
	}
	return ""
}
