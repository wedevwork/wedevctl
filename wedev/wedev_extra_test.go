package wedev

import (
	"path/filepath"
	"testing"

	"github.com/wedevctl/util"
)

// newTestManager returns a VirtualNetworkManager backed by a temp-file BoltDB.
func newTestManager(t *testing.T) (*VirtualNetworkManager, *StorageManager) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	t.Cleanup(func() { sm.Close() })
	vnm, err := NewVirtualNetworkManager(sm, util.NewDefaultIPValidator())
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}
	return vnm, sm
}

func TestVirtualNetwork_GetListDelete(t *testing.T) {
	vnm, _ := newTestManager(t)

	if _, err := vnm.CreateVirtualNetwork("neta", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork(neta) error = %v", err)
	}
	if _, err := vnm.CreateVirtualNetwork("netb", "10.1.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork(netb) error = %v", err)
	}

	// GetVirtualNetwork — found.
	got, err := vnm.GetVirtualNetwork("neta")
	if err != nil {
		t.Fatalf("GetVirtualNetwork(neta) error = %v", err)
	}
	if got.CIDR != "10.0.0.0/24" {
		t.Errorf("GetVirtualNetwork(neta) CIDR = %q, want 10.0.0.0/24", got.CIDR)
	}

	// GetVirtualNetwork — not found.
	if _, err := vnm.GetVirtualNetwork("missing"); err == nil {
		t.Error("GetVirtualNetwork(missing) should return an error")
	}

	// ListVirtualNetworks.
	nets, err := vnm.ListVirtualNetworks()
	if err != nil {
		t.Fatalf("ListVirtualNetworks() error = %v", err)
	}
	if len(nets) != 2 {
		t.Errorf("ListVirtualNetworks() len = %d, want 2", len(nets))
	}

	// DeleteVirtualNetwork — success then not found.
	if err := vnm.DeleteVirtualNetwork("netb"); err != nil {
		t.Errorf("DeleteVirtualNetwork(netb) error = %v", err)
	}
	if err := vnm.DeleteVirtualNetwork("netb"); err == nil {
		t.Error("DeleteVirtualNetwork(netb) twice should return an error")
	}
}

func TestServer_GetUpdateDelete(t *testing.T) {
	vnm, _ := newTestManager(t)
	if _, err := vnm.CreateVirtualNetwork("svcnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("svcnet", "srv", "vpn.example.com", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	// GetServer.
	srv, err := vnm.GetServer("svcnet")
	if err != nil {
		t.Fatalf("GetServer() error = %v", err)
	}
	if srv.Name != "srv" {
		t.Errorf("GetServer() Name = %q, want srv", srv.Name)
	}

	// UpdateServer — success.
	updated, err := vnm.UpdateServer("svcnet", "new.example.com", 51821)
	if err != nil {
		t.Fatalf("UpdateServer() error = %v", err)
	}
	if updated.PublicAddress != "new.example.com" || updated.Port != 51821 {
		t.Errorf("UpdateServer() = %s:%d, want new.example.com:51821", updated.PublicAddress, updated.Port)
	}

	// UpdateServer — invalid address.
	if _, err := vnm.UpdateServer("svcnet", "bad address", 51821); err == nil {
		t.Error("UpdateServer() with invalid address should fail")
	}

	// GetServer / UpdateServer / DeleteServer — unknown network.
	if _, err := vnm.GetServer("nope"); err == nil {
		t.Error("GetServer(nope) should fail")
	}
	if _, err := vnm.UpdateServer("nope", "x.example.com", 1); err == nil {
		t.Error("UpdateServer(nope) should fail")
	}
	if err := vnm.DeleteServer("nope"); err == nil {
		t.Error("DeleteServer(nope) should fail")
	}

	// DeleteServer — success.
	if err := vnm.DeleteServer("svcnet"); err != nil {
		t.Errorf("DeleteServer() error = %v", err)
	}
}

func TestServer_CreateErrors(t *testing.T) {
	vnm, _ := newTestManager(t)
	if _, err := vnm.CreateVirtualNetwork("errnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}

	if _, err := vnm.CreateServer("missing", "srv", "vpn.example.com", 51820); err == nil {
		t.Error("CreateServer() on missing network should fail")
	}
	if _, err := vnm.CreateServer("errnet", "srv", "bad address", 51820); err == nil {
		t.Error("CreateServer() with invalid address should fail")
	}
}

func TestNode_GetListUpdate(t *testing.T) {
	vnm, _ := newTestManager(t)
	if _, err := vnm.CreateVirtualNetwork("nodenet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("nodenet", "srv", "vpn.example.com", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	if _, err := vnm.CreateNode("nodenet", "peer1", "1.2.3.4", 51820, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode(peer1) error = %v", err)
	}
	if _, err := vnm.CreateNode("nodenet", "route1", "", 51820, NodeTypeRoute); err != nil {
		t.Fatalf("CreateNode(route1) error = %v", err)
	}

	// GetNode — found and not found.
	if _, err := vnm.GetNode("nodenet", "peer1"); err != nil {
		t.Errorf("GetNode(peer1) error = %v", err)
	}
	if _, err := vnm.GetNode("nodenet", "ghost"); err == nil {
		t.Error("GetNode(ghost) should fail")
	}
	if _, err := vnm.GetNode("missing", "peer1"); err == nil {
		t.Error("GetNode on missing network should fail")
	}

	// ListNodes.
	nodes, err := vnm.ListNodes("nodenet")
	if err != nil {
		t.Fatalf("ListNodes() error = %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("ListNodes() len = %d, want 2", len(nodes))
	}
	if _, err := vnm.ListNodes("missing"); err == nil {
		t.Error("ListNodes(missing) should fail")
	}

	// UpdateNode — change route1 to peer (requires address).
	upd, err := vnm.UpdateNode("nodenet", "route1", "5.6.7.8", 51822, NodeTypePeer)
	if err != nil {
		t.Fatalf("UpdateNode(route1->peer) error = %v", err)
	}
	if upd.Type != NodeTypePeer || upd.PublicAddress != "5.6.7.8" {
		t.Errorf("UpdateNode() = %s/%s, want peer/5.6.7.8", upd.Type, upd.PublicAddress)
	}

	// UpdateNode — peer without address is rejected.
	if _, err := vnm.UpdateNode("nodenet", "peer1", "", 51820, NodeTypePeer); err == nil {
		t.Error("UpdateNode(peer without address) should fail")
	}
	// UpdateNode — invalid address.
	if _, err := vnm.UpdateNode("nodenet", "peer1", "bad address", 51820, NodeTypePeer); err == nil {
		t.Error("UpdateNode with invalid address should fail")
	}
	// UpdateNode — unknown network / node.
	if _, err := vnm.UpdateNode("missing", "peer1", "1.2.3.4", 1, NodeTypePeer); err == nil {
		t.Error("UpdateNode on missing network should fail")
	}
	if _, err := vnm.UpdateNode("nodenet", "ghost", "1.2.3.4", 1, NodeTypePeer); err == nil {
		t.Error("UpdateNode on missing node should fail")
	}
}

func TestNode_CreateErrors(t *testing.T) {
	vnm, _ := newTestManager(t)
	if _, err := vnm.CreateVirtualNetwork("cn", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}

	if _, err := vnm.CreateNode("missing", "n1", "1.2.3.4", 51820, NodeTypePeer); err == nil {
		t.Error("CreateNode on missing network should fail")
	}
	if _, err := vnm.CreateNode("cn", "n1", "", 51820, NodeTypePeer); err == nil {
		t.Error("CreateNode(peer without address) should fail")
	}
	if _, err := vnm.CreateNode("cn", "n1", "bad address", 51820, NodeTypePeer); err == nil {
		t.Error("CreateNode with invalid address should fail")
	}
}

// TestEnsureIPPoolReconstruction exercises the IP-pool restore path by using a
// fresh manager over a database that already holds a server and nodes.
func TestEnsureIPPoolReconstruction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	vnm1, err := NewVirtualNetworkManager(sm, util.NewDefaultIPValidator())
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}
	if _, err := vnm1.CreateVirtualNetwork("rnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm1.CreateServer("rnet", "srv", "vpn.example.com", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	if _, err := vnm1.CreateNode("rnet", "n1", "1.2.3.4", 51820, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}

	// A second manager has an empty in-memory pool cache and must rebuild it.
	vnm2, err := NewVirtualNetworkManager(sm, util.NewDefaultIPValidator())
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}
	node, err := vnm2.CreateNode("rnet", "n2", "5.6.7.8", 51820, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode() after restart error = %v", err)
	}
	if node.VirtualIP == "" {
		t.Error("CreateNode() after restart returned an empty VirtualIP")
	}
	if err := vnm2.DeleteNode("rnet", "n2"); err != nil {
		t.Errorf("DeleteNode() error = %v", err)
	}
}

// TestEnsureIPPoolFromScratch exercises ensureIPPool's reconstruction branch:
// a fresh manager operates on a network that has no persisted IP-pool state.
func TestEnsureIPPoolFromScratch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Manager 1 only creates the network — no server, no nodes, so no
	// IP-pool state is persisted.
	vnm1, err := NewVirtualNetworkManager(sm, util.NewDefaultIPValidator())
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}
	if _, err := vnm1.CreateVirtualNetwork("scratch", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}

	// Manager 2 has no cached pool and no persisted state to restore, so
	// ensureIPPool must reconstruct the pool from the database.
	vnm2, err := NewVirtualNetworkManager(sm, util.NewDefaultIPValidator())
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}
	node, err := vnm2.CreateNode("scratch", "n1", "1.2.3.4", 51820, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	if node.VirtualIP == "" {
		t.Error("CreateNode() returned an empty VirtualIP")
	}
}

func TestStorage_GetNetworkByID(t *testing.T) {
	_, sm := newTestManager(t)
	net, err := sm.CreateNetwork("idnet", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}

	got, err := sm.GetNetworkByID(net.ID)
	if err != nil {
		t.Fatalf("GetNetworkByID() error = %v", err)
	}
	if got.Name != "idnet" {
		t.Errorf("GetNetworkByID() Name = %q, want idnet", got.Name)
	}

	if _, err := sm.GetNetworkByID("no-such-id"); err == nil {
		t.Error("GetNetworkByID(no-such-id) should fail")
	}
}

func TestStorage_GetConfigHashByVersion(t *testing.T) {
	vnm, sm := newTestManager(t)
	if _, err := vnm.CreateVirtualNetwork("hnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("hnet", "srv", "vpn.example.com", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	gen := NewWireGuardConfigGenerator(sm)
	version, created, err := gen.SaveConfigVersion("hnet")
	if err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}
	if !created {
		t.Fatal("SaveConfigVersion() should have created a version")
	}

	net, err := sm.GetNetworkByName("hnet")
	if err != nil {
		t.Fatalf("GetNetworkByName() error = %v", err)
	}
	hash, err := sm.GetConfigHashByVersion(net.ID, version.Version)
	if err != nil {
		t.Fatalf("GetConfigHashByVersion() error = %v", err)
	}
	if hash != version.ContentHash {
		t.Errorf("GetConfigHashByVersion() = %q, want %q", hash, version.ContentHash)
	}

	if _, err := sm.GetConfigHashByVersion(net.ID, 999); err == nil {
		t.Error("GetConfigHashByVersion() for a missing version should fail")
	}
}

func TestConfigGenerator_ErrorPaths(t *testing.T) {
	vnm, sm := newTestManager(t)
	gen := NewWireGuardConfigGenerator(sm)

	// Unknown network.
	if _, _, err := gen.GenerateConfigs("ghost", sm); err == nil {
		t.Error("GenerateConfigs(ghost) should fail")
	}
	if _, err := gen.GetConfigHistory("ghost"); err == nil {
		t.Error("GetConfigHistory(ghost) should fail")
	}
	if _, err := gen.GetConfig("ghost", 1); err == nil {
		t.Error("GetConfig(ghost) should fail")
	}

	// Network with no server.
	if _, err := vnm.CreateVirtualNetwork("noserver", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, _, err := gen.GenerateConfigs("noserver", sm); err == nil {
		t.Error("GenerateConfigs() with no server should fail")
	}
	if _, _, err := gen.SaveConfigVersion("noserver"); err == nil {
		t.Error("SaveConfigVersion() with no server should fail")
	}
}
