package requestcontext

import (
	"context"
	"testing"
)

func TestClientIP(t *testing.T) {
	ctx := WithClientIP(context.Background(), "192.0.2.10")

	ip, ok := ClientIP(ctx)
	if !ok {
		t.Fatal("ClientIP() did not find the stored IP")
	}
	if ip != "192.0.2.10" {
		t.Fatalf("ClientIP() = %q, want %q", ip, "192.0.2.10")
	}
}

func TestClientIPRejectsMissingOrEmptyValue(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"missing": context.Background(),
		"empty":   WithClientIP(context.Background(), ""),
	} {
		t.Run(name, func(t *testing.T) {
			if ip, ok := ClientIP(ctx); ok || ip != "" {
				t.Fatalf("ClientIP() = (%q, %t), want (empty, false)", ip, ok)
			}
		})
	}
}
