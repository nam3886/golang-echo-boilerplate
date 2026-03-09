// Package connectutil provides helpers for mapping domain errors to Connect RPC errors.
package connectutil

import (
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
)

var codeToConnect = map[domainerr.ErrorCode]connect.Code{
	domainerr.CodeInvalidArgument:    connect.CodeInvalidArgument,
	domainerr.CodeNotFound:           connect.CodeNotFound,
	domainerr.CodeAlreadyExists:      connect.CodeAlreadyExists,
	domainerr.CodePermissionDenied:   connect.CodePermissionDenied,
	domainerr.CodeUnauthenticated:    connect.CodeUnauthenticated,
	domainerr.CodeFailedPrecondition: connect.CodeFailedPrecondition,
	domainerr.CodeInternal:           connect.CodeInternal,
	domainerr.CodeUnavailable:        connect.CodeUnavailable,
	domainerr.CodeResourceExhausted:  connect.CodeResourceExhausted,
}

// DomainErrorToConnect maps a DomainError to a Connect RPC error.
// Non-domain errors are logged and returned as a generic internal error to avoid leaking internals.
func DomainErrorToConnect(err error) error {
	var domErr *domainerr.DomainError
	if errors.As(err, &domErr) {
		code := codeToConnect[domErr.Code]
		return connect.NewError(code, fmt.Errorf("%s", domErr.Message))
	}
	slog.Error("unhandled internal error", "err", err)
	return connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
}
