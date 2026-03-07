package grpc

import (
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	userv1 "github.com/gnha/gnha-services/gen/proto/user/v1"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
	domainerr "github.com/gnha/gnha-services/internal/shared/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// toProto converts a domain User to a protobuf User.
func toProto(u *domain.User) *userv1.User {
	return &userv1.User{
		Id:        string(u.ID()),
		Email:     u.Email(),
		Name:      u.Name(),
		Role:      string(u.Role()),
		CreatedAt: timestamppb.New(u.CreatedAt()),
		UpdatedAt: timestamppb.New(u.UpdatedAt()),
	}
}

// domainErrorToConnect maps DomainError to Connect RPC error codes.
// Non-domain errors are logged and returned as a generic internal error to avoid leaking internals.
func domainErrorToConnect(err error) error {
	var domErr *domainerr.DomainError
	if errors.As(err, &domErr) {
		code := codeToConnect[domErr.Code]
		return connect.NewError(code, err)
	}
	slog.Error("unhandled internal error", "err", err)
	return connect.NewError(connect.CodeInternal, fmt.Errorf("internal error"))
}

var codeToConnect = map[domainerr.ErrorCode]connect.Code{
	domainerr.CodeInvalidArgument:    connect.CodeInvalidArgument,
	domainerr.CodeNotFound:           connect.CodeNotFound,
	domainerr.CodeAlreadyExists:      connect.CodeAlreadyExists,
	domainerr.CodePermissionDenied:   connect.CodePermissionDenied,
	domainerr.CodeUnauthenticated:    connect.CodeUnauthenticated,
	domainerr.CodeFailedPrecondition: connect.CodeFailedPrecondition,
	domainerr.CodeInternal:           connect.CodeInternal,
	domainerr.CodeUnavailable:        connect.CodeUnavailable,
}
