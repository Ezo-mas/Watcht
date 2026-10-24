package config

import "testing"

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{name: "default", env: map[string]string{}, want: ":8080"},
		{name: "empty uses default", env: map[string]string{"WATCHT_LISTEN": ""}, want: ":8080"},
		{name: "custom port", env: map[string]string{"WATCHT_LISTEN": ":9090"}, want: ":9090"},
		{name: "host and port", env: map[string]string{"WATCHT_LISTEN": "127.0.0.1:8099"}, want: "127.0.0.1:8099"},
		{name: "missing port", env: map[string]string{"WATCHT_LISTEN": "localhost"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(k string) (string, bool) {
				v, ok := tt.env[k]
				return v, ok
			}
			cfg, err := load(lookup)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("load() = %+v, want error", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("load() error = %v", err)
			}
			if cfg.Listen != tt.want {
				t.Errorf("Listen = %q, want %q", cfg.Listen, tt.want)
			}
		})
	}
}
