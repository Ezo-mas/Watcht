package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthURL(t *testing.T) {
	tests := []struct {
		listen  string
		want    string
		wantErr bool
	}{
		{listen: ":8080", want: "http://127.0.0.1:8080/healthz"},
		{listen: "0.0.0.0:8080", want: "http://127.0.0.1:8080/healthz"},
		{listen: "[::]:8080", want: "http://[::1]:8080/healthz"},
		{listen: "127.0.0.1:8099", want: "http://127.0.0.1:8099/healthz"},
		{listen: "192.168.1.10:8080", want: "http://192.168.1.10:8080/healthz"},
		{listen: "nonsense", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.listen, func(t *testing.T) {
			got, err := healthURL(tt.listen)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("healthURL(%q) = %q, want error", tt.listen, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("healthURL(%q) error = %v", tt.listen, err)
			}
			if got != tt.want {
				t.Errorf("healthURL(%q) = %q, want %q", tt.listen, got, tt.want)
			}
		})
	}
}

func TestCheckHealth(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "healthy", status: http.StatusOK},
		{name: "unhealthy", status: http.StatusServiceUnavailable, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/healthz" {
					t.Errorf("path = %q, want /healthz", r.URL.Path)
				}
				w.WriteHeader(tt.status)
			}))
			defer srv.Close()

			err := checkHealth(context.Background(), srv.Listener.Addr().String())
			if (err != nil) != tt.wantErr {
				t.Errorf("checkHealth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckHealthNoServer(t *testing.T) {
	// Grab a free port, then close it so nothing is listening there.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	if err := checkHealth(context.Background(), addr); err == nil {
		t.Fatal("checkHealth() with no server = nil, want error")
	}
}

func TestRunCommands(t *testing.T) {
	tests := []struct {
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{args: []string{"version"}, wantCode: 0, wantStdout: "watcht dev"},
		{args: []string{"--help"}, wantCode: 0, wantStdout: "Usage: watcht"},
		{args: []string{"bogus"}, wantCode: 2, wantStderr: `unknown command "bogus"`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
