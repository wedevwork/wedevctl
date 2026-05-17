package util

import "testing"

func TestIPPool_MarkIPAllocated(t *testing.T) {
	pool, err := NewIPPool("10.0.0.0/24")
	if err != nil {
		t.Fatalf("NewIPPool() error = %v", err)
	}

	// Empty IP is rejected.
	if err := pool.MarkIPAllocated(""); err == nil {
		t.Error("MarkIPAllocated(\"\") should return an error")
	}

	// First mark succeeds.
	if err := pool.MarkIPAllocated("10.0.0.5"); err != nil {
		t.Errorf("MarkIPAllocated() error = %v, want nil", err)
	}

	// Marking the same IP again is rejected.
	if err := pool.MarkIPAllocated("10.0.0.5"); err == nil {
		t.Error("MarkIPAllocated() of an already-allocated IP should return an error")
	}

	allocated := pool.GetAllocatedIPs()
	if !allocated["10.0.0.5"] {
		t.Error("GetAllocatedIPs() should contain the marked IP 10.0.0.5")
	}
}

func TestIPPool_GetAllocatedIPs_IsCopy(t *testing.T) {
	pool, err := NewIPPool("10.0.0.0/24")
	if err != nil {
		t.Fatalf("NewIPPool() error = %v", err)
	}
	if _, err := pool.AllocateNodeIP(); err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}

	snapshot := pool.GetAllocatedIPs()
	if len(snapshot) != 1 {
		t.Fatalf("GetAllocatedIPs() len = %d, want 1", len(snapshot))
	}

	// Mutating the returned map must not affect the pool's internal state.
	snapshot["10.0.0.99"] = true
	if len(pool.GetAllocatedIPs()) != 1 {
		t.Error("GetAllocatedIPs() must return a copy, not the internal map")
	}
}

func TestIPPool_SyncNextIndex(t *testing.T) {
	pool, err := NewIPPool("10.0.0.0/24")
	if err != nil {
		t.Fatalf("NewIPPool() error = %v", err)
	}

	// Server IP is index 0; mark a node IP several slots in.
	if err := pool.MarkIPAllocated(pool.GetServerIP()); err != nil {
		t.Fatalf("MarkIPAllocated(serverIP) error = %v", err)
	}
	if err := pool.MarkIPAllocated("10.0.0.4"); err != nil {
		t.Fatalf("MarkIPAllocated() error = %v", err)
	}

	pool.SyncNextIndex()

	// After syncing, the next allocation must not collide with 10.0.0.4.
	ip, err := pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}
	if ip == "10.0.0.4" {
		t.Errorf("AllocateNodeIP() returned an already-allocated IP %s after SyncNextIndex()", ip)
	}
}
