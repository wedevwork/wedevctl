package wedev

import (
	"fmt"
	"testing"
)

// TestDeleteServerCleansNameIndex covers the fix for the orphaned server
// name-index entry that DeleteServer previously left behind (it deleted the
// index with key "name" instead of "networkID:name").
func TestDeleteServerCleansNameIndex(t *testing.T) {
	_, sm := newTestManager(t)

	net, err := sm.CreateNetwork("svcnet", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}
	if _, err := sm.CreateServer(net.ID, "srv", "vpn.example.com", 51820, "10.0.0.1", "priv", "pub"); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	if err := sm.DeleteServer(net.ID); err != nil {
		t.Fatalf("DeleteServer() error = %v", err)
	}

	// Both indexes must be cleared.
	if _, err := sm.GetServerByName(net.ID, "srv"); err == nil {
		t.Error("GetServerByName() should fail after DeleteServer()")
	}
	if _, err := sm.GetServerByNetworkID(net.ID); err == nil {
		t.Error("GetServerByNetworkID() should fail after DeleteServer()")
	}
	// A fresh server with the same name must be creatable — i.e. no orphan
	// name-index entry survived the delete.
	if _, err := sm.CreateServer(net.ID, "srv", "vpn2.example.com", 51820, "10.0.0.1", "priv", "pub"); err != nil {
		t.Errorf("re-creating server 'srv' after delete failed: %v", err)
	}
}

// TestStorageNetworkScoping verifies the network-scoped index buckets keep
// servers, nodes, and config versions correctly isolated per network — and
// that deleting one network leaves the others fully intact.
func TestStorageNetworkScoping(t *testing.T) {
	_, sm := newTestManager(t)

	netA, err := sm.CreateNetwork("neta", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork(neta) error = %v", err)
	}
	netB, err := sm.CreateNetwork("netb", "10.1.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork(netb) error = %v", err)
	}

	// Servers — one per network, looked up by network ID.
	if _, err := sm.CreateServer(netA.ID, "srvA", "a.example.com", 51820, "10.0.0.1", "p", "p"); err != nil {
		t.Fatalf("CreateServer(A) error = %v", err)
	}
	if _, err := sm.CreateServer(netB.ID, "srvB", "b.example.com", 51820, "10.1.0.1", "p", "p"); err != nil {
		t.Fatalf("CreateServer(B) error = %v", err)
	}
	if s, err := sm.GetServerByNetworkID(netA.ID); err != nil || s.Name != "srvA" {
		t.Errorf("GetServerByNetworkID(A) = %v (err %v); want srvA", s, err)
	}
	if s, err := sm.GetServerByNetworkID(netB.ID); err != nil || s.Name != "srvB" {
		t.Errorf("GetServerByNetworkID(B) = %v (err %v); want srvB", s, err)
	}

	// Nodes — listing is scoped to a single network.
	for i := 0; i < 3; i++ {
		if _, err := sm.CreateNode(netA.ID, fmt.Sprintf("a%d", i), "", 51820, fmt.Sprintf("10.0.0.%d", i+2), NodeTypeRoute, "p", "p"); err != nil {
			t.Fatalf("CreateNode(A,%d) error = %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := sm.CreateNode(netB.ID, fmt.Sprintf("b%d", i), "", 51820, fmt.Sprintf("10.1.0.%d", i+2), NodeTypeRoute, "p", "p"); err != nil {
			t.Fatalf("CreateNode(B,%d) error = %v", i, err)
		}
	}
	if nodes, err := sm.ListNodesByNetworkID(netA.ID); err != nil || len(nodes) != 3 {
		t.Errorf("ListNodesByNetworkID(A) len = %d (err %v); want 3", len(nodes), err)
	}
	if nodes, err := sm.ListNodesByNetworkID(netB.ID); err != nil || len(nodes) != 2 {
		t.Errorf("ListNodesByNetworkID(B) len = %d (err %v); want 2", len(nodes), err)
	}

	// Config versions — numbered independently per network.
	for i := 0; i < 3; i++ {
		if _, err := sm.SaveConfigVersion(netA.ID, fmt.Sprintf("hashA%d", i), map[string]string{"x": "y"}); err != nil {
			t.Fatalf("SaveConfigVersion(A,%d) error = %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := sm.SaveConfigVersion(netB.ID, fmt.Sprintf("hashB%d", i), map[string]string{"x": "y"}); err != nil {
			t.Fatalf("SaveConfigVersion(B,%d) error = %v", i, err)
		}
	}
	versA, err := sm.ListConfigVersions(netA.ID)
	if err != nil || len(versA) != 3 {
		t.Fatalf("ListConfigVersions(A) len = %d (err %v); want 3", len(versA), err)
	}
	for i, v := range versA {
		if v.Version != i+1 {
			t.Errorf("ListConfigVersions(A)[%d].Version = %d; want %d", i, v.Version, i+1)
		}
	}
	if vs, err := sm.ListConfigVersions(netB.ID); err != nil || len(vs) != 2 {
		t.Errorf("ListConfigVersions(B) len = %d (err %v); want 2", len(vs), err)
	}
	if latest, err := sm.GetLatestConfigVersion(netA.ID); err != nil || latest.Version != 3 {
		t.Errorf("GetLatestConfigVersion(A) version = %v (err %v); want 3", latest, err)
	}
	if got, err := sm.GetConfigVersion(netA.ID, 2); err != nil || got.ContentHash != "hashA1" {
		t.Errorf("GetConfigVersion(A,2) hash = %v (err %v); want hashA1", got, err)
	}

	// Deleting netA must leave netB completely intact.
	if err := sm.DeleteNetwork("neta"); err != nil {
		t.Fatalf("DeleteNetwork(neta) error = %v", err)
	}
	if _, err := sm.GetServerByNetworkID(netB.ID); err != nil {
		t.Errorf("netB server should survive deletion of netA: %v", err)
	}
	if nodes, err := sm.ListNodesByNetworkID(netB.ID); err != nil || len(nodes) != 2 {
		t.Errorf("netB nodes len = %d (err %v) after deleting netA; want 2", len(nodes), err)
	}
	if vs, err := sm.ListConfigVersions(netB.ID); err != nil || len(vs) != 2 {
		t.Errorf("netB configs len = %d (err %v) after deleting netA; want 2", len(vs), err)
	}

	// netA's resources must be fully gone from every index.
	if _, err := sm.GetServerByNetworkID(netA.ID); err == nil {
		t.Error("netA server still present after DeleteNetwork")
	}
	if nodes, err := sm.ListNodesByNetworkID(netA.ID); err != nil || len(nodes) != 0 {
		t.Errorf("netA nodes len = %d (err %v) after DeleteNetwork; want 0", len(nodes), err)
	}
	if vs, err := sm.ListConfigVersions(netA.ID); err != nil || len(vs) != 0 {
		t.Errorf("netA configs len = %d (err %v) after DeleteNetwork; want 0", len(vs), err)
	}
}
