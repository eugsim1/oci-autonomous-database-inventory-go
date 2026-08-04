package config

import (
	"io"
	"reflect"
	"testing"
)

func TestParseRegionsNormalizesAndDeduplicates(t *testing.T) {
	cfg, err := Parse([]string{
		"--regions", "eu-paris-1, US-ASHBURN-1,eu-paris-1",
		"--workers", "3",
	}, io.Discard)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := []string{"eu-paris-1", "us-ashburn-1"}
	if !reflect.DeepEqual(cfg.Regions, want) {
		t.Fatalf("Regions = %#v, want %#v", cfg.Regions, want)
	}
}

func TestParseRejectsUnknownAuth(t *testing.T) {
	if _, err := Parse([]string{"--auth", "magic"}, io.Discard); err == nil {
		t.Fatal("Parse() expected an error")
	}
}

func TestParseAcceptsOKEWorkloadIdentity(t *testing.T) {
	cfg, err := Parse([]string{"--auth", "oke_workload_identity"}, io.Discard)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.AuthMode != AuthOKEWorkloadIdentity {
		t.Fatalf("AuthMode = %q, want %q", cfg.AuthMode, AuthOKEWorkloadIdentity)
	}
}
