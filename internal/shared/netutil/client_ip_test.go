package netutil_test

import (
	"context"
	"testing"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/netutil"
)

func TestGetClientIP_Set(t *testing.T) {
	ctx := netutil.WithClientIP(context.Background(), "192.168.1.1")
	ip := netutil.GetClientIP(ctx)
	if ip != "192.168.1.1" {
		t.Errorf("expected %q, got %q", "192.168.1.1", ip)
	}
}

func TestGetClientIP_Empty(t *testing.T) {
	ip := netutil.GetClientIP(context.Background())
	if ip != "" {
		t.Errorf("expected empty string, got %q", ip)
	}
}

func TestGetClientIP_Override(t *testing.T) {
	ctx := netutil.WithClientIP(context.Background(), "10.0.0.1")
	ctx = netutil.WithClientIP(ctx, "10.0.0.2")
	ip := netutil.GetClientIP(ctx)
	if ip != "10.0.0.2" {
		t.Errorf("expected %q after override, got %q", "10.0.0.2", ip)
	}
}
