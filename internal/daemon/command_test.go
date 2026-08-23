package daemon

import "testing"

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantAddr string
		wantDB   string
		wantErr  bool
	}{
		{
			name:     "default configuration",
			wantAddr: ":17890",
			wantDB:   "./ark.db",
		},
		{
			name:     "custom configuration",
			args:     []string{"--listen", ":8080", "--database", "data/ark.db"},
			wantAddr: ":8080",
			wantDB:   "data/ark.db",
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

			if err == nil && config.DatabasePath != tt.wantDB {
				t.Fatalf("DatabasePath = %q, want %q", config.DatabasePath, tt.wantDB)
			}
		})
	}
}
