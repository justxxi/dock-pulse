package config

import (
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid loopback config",
			args:    []string{"-listen-addr", "127.0.0.1:8080"},
			wantErr: false,
		},
		{
			name:    "non loopback without token fails",
			args:    []string{"-listen-addr", "0.0.0.0:8080"},
			wantErr: true,
		},
		{
			name:    "non loopback with token passes",
			args:    []string{"-listen-addr", "0.0.0.0:8080", "-token", "secret123"},
			wantErr: false,
		},
		{
			name:    "invalid log level",
			args:    []string{"-log-level", "invalid"},
			wantErr: true,
		},
		{
			name:    "tls cert without key fails",
			args:    []string{"-tls-cert", "cert.pem"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load([]string{"-listen-addr", "127.0.0.1:9090"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Errorf("expected listen-addr 127.0.0.1:9090, got %s", cfg.ListenAddr)
	}
	if cfg.StatsInterval != 2*time.Second {
		t.Errorf("expected stats interval 2s, got %v", cfg.StatsInterval)
	}
}
