// file name — /internal/alert/platforms/youtube/onboarding/onboarding_test.go
package onboarding

import (
	"context"
	"testing"
	"time"
)

func TestOnboardChannel_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     OnboardRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: OnboardRequest{
				ChannelID: "abc123",
				Lease:     time.Hour,
			},
			wantErr: false,
		},
		{
			name: "missing channel id",
			req: OnboardRequest{
				Lease: time.Hour,
			},
			wantErr: true,
		},
	}

	ctx := context.Background()
	svc := Service{}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := svc.OnboardChannel(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Fatalf("OnboardChannel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
