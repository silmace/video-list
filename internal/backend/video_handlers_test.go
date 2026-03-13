package backend

import "testing"

func TestGetTimeDifference(t *testing.T) {
	tests := []struct {
		name    string
		start   string
		end     string
		want    string
		wantErr bool
	}{
		{
			name:  "normal range",
			start: "00:00:05",
			end:   "00:00:15",
			want:  "10",
		},
		{
			name:  "cross minute",
			start: "00:59:59",
			end:   "01:00:01",
			want:  "2",
		},
		{
			name:    "invalid format",
			start:   "bad",
			end:     "00:00:10",
			wantErr: true,
		},
		{
			name:    "end before start",
			start:   "00:00:10",
			end:     "00:00:09",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTimeDifference(tt.start, tt.end)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("getTimeDifference() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("getTimeDifference() = %s, want %s", got, tt.want)
			}
		})
	}
}
