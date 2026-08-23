package daemon

import "testing"

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantAddr string
		wantErr  bool
	}{
		{
			name:     "default listen address",
			wantAddr: ":17890",
		},
		{
			name:     "custom listen address",
			args:     []string{"--listen", ":8080"},
			wantAddr: ":8080",
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := parseConfig(tt.args)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parseConfig() error = %v, want error = %t", err, tt.wantErr)
			}

			if err == nil && config.ListenAddr != tt.wantAddr {
				t.Fatalf("ListenAddr = %q, want %q", config.ListenAddr, tt.wantAddr)
			}
		})
	}
}
