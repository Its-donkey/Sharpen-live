// file name — /internal/alert/platforms/youtube/api/player_test.go
package api

import "testing"

func TestParsePlayerResponse_Basic(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{
			name:    "valid payload",
			payload: `{"videoDetails":{"videoId":"abc123"}}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			payload: `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePlayerResponse(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePlayerResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
