package discovery_test

import (
	"testing"

	"netcluster/internal/discovery"
)

func TestExpandTargets_SingleIP(t *testing.T) {
	ips, err := discovery.ExpandTargets("10.0.0.5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ips) != 1 || ips[0] != "10.0.0.5" {
		t.Fatalf("expected [10.0.0.5], got %v", ips)
	}
}

func TestExpandTargets_CIDR(t *testing.T) {
	ips, err := discovery.ExpandTargets("10.0.0.0/30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.0", "10.0.0.1", "10.0.0.2", "10.0.0.3"}
	if len(ips) != len(want) {
		t.Fatalf("expected %d addresses, got %d: %v", len(want), len(ips), ips)
	}
	for i, ip := range want {
		if ips[i] != ip {
			t.Errorf("index %d: expected %s, got %s", i, ip, ips[i])
		}
	}
}

func TestExpandTargets_DashedRange(t *testing.T) {
	ips, err := discovery.ExpandTargets("10.0.0.1-10.0.0.3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	if len(ips) != len(want) {
		t.Fatalf("expected %d addresses, got %d: %v", len(want), len(ips), ips)
	}
	for i, ip := range want {
		if ips[i] != ip {
			t.Errorf("index %d: expected %s, got %s", i, ip, ips[i])
		}
	}
}

func TestExpandTargets_RangeAcrossOctetBoundary(t *testing.T) {
	ips, err := discovery.ExpandTargets("10.0.0.254-10.0.1.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.254", "10.0.0.255", "10.0.1.0", "10.0.1.1"}
	if len(ips) != len(want) {
		t.Fatalf("expected %d addresses, got %d: %v", len(want), len(ips), ips)
	}
	for i, ip := range want {
		if ips[i] != ip {
			t.Errorf("index %d: expected %s, got %s", i, ip, ips[i])
		}
	}
}

func TestExpandTargets_ReversedRangeIsError(t *testing.T) {
	_, err := discovery.ExpandTargets("10.0.0.10-10.0.0.1")
	if err == nil {
		t.Fatal("expected error for reversed range, got nil")
	}
}

func TestExpandTargets_InvalidSpecIsError(t *testing.T) {
	_, err := discovery.ExpandTargets("not-an-ip-or-range")
	if err == nil {
		t.Fatal("expected error for invalid spec, got nil")
	}
}

func TestExpandTargets_ExceedsCapIsError(t *testing.T) {
	// A /16 is 65536 addresses, well over the 4096 cap.
	_, err := discovery.ExpandTargets("10.0.0.0/16")
	if err == nil {
		t.Fatal("expected error for a target spec exceeding the cap, got nil")
	}
}
