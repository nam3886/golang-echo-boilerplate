package grpc

import (
	userv1 "github.com/gnha/gnha-services/gen/proto/user/v1"
	"github.com/gnha/gnha-services/internal/modules/user/domain"
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

