// Package connectutil provides helpers for mapping domain errors to Connect RPC errors.
package connectutil

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
	sharederr "github.com/gnha/golang-echo-boilerplate/internal/shared/errors"
)

var codeToConnect = map[sharederr.ErrorCode]connect.Code{
	sharederr.CodeInvalidArgument:    connect.CodeInvalidArgument,
	sharederr.CodeNotFound:           connect.CodeNotFound,
	sharederr.CodeAlreadyExists:      connect.CodeAlreadyExists,
	sharederr.CodePermissionDenied:   connect.CodePermissionDenied,
	sharederr.CodeUnauthenticated:    connect.CodeUnauthenticated,
	sharederr.CodeFailedPrecondition: connect.CodeFailedPrecondition,
	sharederr.CodeInternal:           connect.CodeInternal,
	sharederr.CodeUnavailable:        connect.CodeUnavailable,
	sharederr.CodeResourceExhausted:  connect.CodeResourceExhausted,
}

// DomainErrorToConnect maps a DomainError to a Connect RPC error.
// Non-domain errors are logged and returned as a generic internal error to avoid leaking internals.
func DomainErrorToConnect(ctx context.Context, err error) error {
	var domErr *sharederr.DomainError
	if errors.As(err, &domErr) {
		code, ok := codeToConnect[domErr.Code]
		if !ok {
			code = connect.CodeInternal
		}
		return connect.NewError(code, errors.New(domErr.Message))
	}
	slog.ErrorContext(ctx, "unhandled internal error", "err", err)
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}
