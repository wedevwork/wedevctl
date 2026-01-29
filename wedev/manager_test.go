package wedev

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wedevctl/util"
)

func TestCreateVirtualNetwork_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	net, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	if err != nil {
		t.Errorf("CreateVirtualNetwork() error = %v", err)
		return
	}

	if net.Name != "testnet" || net.CIDR != "10.0.0.0/24" {
		t.Errorf("CreateVirtualNetwork() returned unexpected values")
	}
}

func TestCreateVirtualNetwork_InvalidName(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	tests := []struct {
		name  string
		cidr  string
		valid bool
	}{
		{"1invalid", "10.0.0.0/24", false},
		{"_invalid", "10.0.0.0/24", false},
		{"valid", "10.0.0.0/24", true},
		{"valid123", "10.0.0.0/24", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := vnm.CreateVirtualNetwork(tt.name, tt.cidr)
			if (err != nil) == tt.valid {
				t.Errorf("CreateVirtualNetwork(%q) got unexpected error: %v", tt.name, err)
			}
		})
	}
}

func TestCreateServer_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	server, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)

	if err != nil {
		t.Errorf("CreateServer() error = %v", err)
		return
	}

	if server.VirtualIP != "10.0.0.1" {
		t.Errorf("CreateServer() assigned IP = %s, want 10.0.0.1", server.VirtualIP)
	}

	if server.Port != 51820 {
		t.Errorf("CreateServer() port = %d, want 51820", server.Port)
	}
}

func TestCreateServer_DefaultPort(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	server, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 0)

	if err != nil {
		t.Errorf("CreateServer() error = %v", err)
		return
	}

	if server.Port != 51820 {
		t.Errorf("CreateServer() default port = %d, want 51820", server.Port)
	}
}

func TestCreateNode_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	node, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)

	if err != nil {
		t.Errorf("CreateNode() error = %v", err)
		return
	}

	if node.VirtualIP != "10.0.0.2" {
		t.Errorf("CreateNode() assigned IP = %s, want 10.0.0.2", node.VirtualIP)
	}

	if node.Type != NodeTypePeer {
		t.Errorf("CreateNode() type = %v, want peer", node.Type)
	}
}

func TestCreateNode_DefaultType(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	node, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, "")

	if err != nil {
		t.Errorf("CreateNode() error = %v", err)
		return
	}

	if node.Type != NodeTypePeer {
		t.Errorf("CreateNode() default type = %v, want peer", node.Type)
	}
}

func TestDeleteNode_IPRecycling(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	node1, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node1) error = %v", err)
	}
	if _, err := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode(node2) error = %v", err)
	}

	// Delete node1
	err = vnm.DeleteNode("testnet", "node1")
	if err != nil {
		t.Errorf("DeleteNode() error = %v", err)
		return
	}

	// Create another node - should reuse node1's IP
	node3, err := vnm.CreateNode("testnet", "node3", "192.168.1.4", 51823, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node3) error = %v", err)
	}

	if node3.VirtualIP != node1.VirtualIP {
		t.Errorf("CreateNode() should reuse recycled IP %s, got %s", node1.VirtualIP, node3.VirtualIP)
	}
}

func TestIPReuseAcrossManagerRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// First manager instance
	storage1, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}

	validator := util.NewDefaultIPValidator()
	vnm1, err := NewVirtualNetworkManager(storage1, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	// Create network and nodes
	if _, err := vnm1.CreateVirtualNetwork("testnet", "192.168.1.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm1.CreateServer("testnet", "server1", "192.168.1.100", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	node1, err := vnm1.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node1) error = %v", err)
	}
	if node1.VirtualIP != "192.168.1.2" {
		t.Errorf("Expected node1 IP 192.168.1.2, got %s", node1.VirtualIP)
	}

	node2, err := vnm1.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node2) error = %v", err)
	}
	if node2.VirtualIP != "192.168.1.3" {
		t.Errorf("Expected node2 IP 192.168.1.3, got %s", node2.VirtualIP)
	}

	node3, err := vnm1.CreateNode("testnet", "node3", "192.168.1.4", 51823, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node3) error = %v", err)
	}
	if node3.VirtualIP != "192.168.1.4" {
		t.Errorf("Expected node3 IP 192.168.1.4, got %s", node3.VirtualIP)
	}

	// Delete node2 (192.168.1.3)
	if err := vnm1.DeleteNode("testnet", "node2"); err != nil {
		t.Fatalf("DeleteNode(node2) error = %v", err)
	}

	// Close first manager
	if err := storage1.Close(); err != nil {
		t.Fatalf("storage.Close() error = %v", err)
	}

	// Create second manager instance (simulating restart)
	storage2, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage2.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	vnm2, err := NewVirtualNetworkManager(storage2, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	// Create new node - should reuse 192.168.1.3 (the deleted node2's IP)
	node4, err := vnm2.CreateNode("testnet", "node4", "192.168.1.5", 51824, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node4) error = %v", err)
	}

	if node4.VirtualIP != "192.168.1.3" {
		t.Errorf("Expected node4 to reuse IP 192.168.1.3, got %s", node4.VirtualIP)
	}
}

func TestGenerateServerConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	server, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	node1, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node1) error = %v", err)
	}
	node2, err := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypeRoute)
	if err != nil {
		t.Fatalf("CreateNode(node2) error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)
	configs, _, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Errorf("GenerateConfigs() error = %v", err)
		return
	}

	serverConfig := configs[server.Name]

	// Verify essential server config elements
	if !strings.Contains(serverConfig, "PrivateKey") {
		t.Errorf("Server config missing PrivateKey")
	}
	if !strings.Contains(serverConfig, "PostUp = sysctl -w net.ipv4.ip_forward=1") {
		t.Errorf("Server config missing PostUp directive")
	}
	if !strings.Contains(serverConfig, "PostDown = sysctl -w net.ipv4.ip_forward=0") {
		t.Errorf("Server config missing PostDown directive")
	}

	// Server should have peers for both nodes
	if !strings.Contains(serverConfig, node1.PublicKey) {
		t.Errorf("Server config missing peer for node1")
	}
	if !strings.Contains(serverConfig, node2.PublicKey) {
		t.Errorf("Server config missing peer for node2")
	}
}

func TestGeneratePeerNodeConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	server, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	node1, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node1) error = %v", err)
	}
	node2, err := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node2) error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)
	configs, _, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Fatalf("GenerateConfigs() error = %v", err)
	}

	node1Config := configs[node1.Name]

	// Peer node should have server peer
	if !strings.Contains(node1Config, server.PublicKey) {
		t.Errorf("Peer node config missing server peer")
	}

	// Peer node should have other peer node as peer
	if !strings.Contains(node1Config, node2.PublicKey) {
		t.Errorf("Peer node config should include other peer node")
	}

	// Should have correct Endpoint format
	if !strings.Contains(node1Config, "192.168.1.1:51820") {
		t.Errorf("Peer node config missing correct server endpoint")
	}
}

func TestGenerateRouteNodeConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	server, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	node1, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypeRoute)
	if err != nil {
		t.Fatalf("CreateNode(node1) error = %v", err)
	}
	node2, err := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(node2) error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)
	configs, _, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Fatalf("GenerateConfigs() error = %v", err)
	}

	routeNodeConfig := configs[node1.Name]

	// Route node should have server peer
	if !strings.Contains(routeNodeConfig, server.PublicKey) {
		t.Errorf("Route node config missing server peer")
	}

	// Route node SHOULD have peer type nodes as peers (enhancement)
	if !strings.Contains(routeNodeConfig, node2.PublicKey) {
		t.Errorf("Route node config should include peer type nodes for direct communication")
	}

	// Route node should have peer endpoint
	if !strings.Contains(routeNodeConfig, "192.168.1.3:51822") {
		t.Errorf("Route node config missing peer node endpoint")
	}
}

func TestRouteNodeWithMultiplePeerNodes(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	// Create network with server, 2 peer nodes, and 2 route nodes
	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.1.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	server, err := vnm.CreateServer("testnet", "s1", "s1.example.com", 51820)
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	// Create peer nodes with endpoints
	peer1, err := vnm.CreateNode("testnet", "r1", "r1.pub", 51821, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(r1) error = %v", err)
	}
	peer2, err := vnm.CreateNode("testnet", "r2", "r2.pub", 51822, NodeTypePeer)
	if err != nil {
		t.Fatalf("CreateNode(r2) error = %v", err)
	}

	// Create route nodes without endpoints
	route1, err := vnm.CreateNode("testnet", "r3", "", 51820, NodeTypeRoute)
	if err != nil {
		t.Fatalf("CreateNode(r3) error = %v", err)
	}
	route2, err := vnm.CreateNode("testnet", "r4", "", 51820, NodeTypeRoute)
	if err != nil {
		t.Fatalf("CreateNode(r4) error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)
	configs, _, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Fatalf("GenerateConfigs() error = %v", err)
	}

	// Test route node r3 config
	route1Config := configs[route1.Name]

	// Route node should have server
	if !strings.Contains(route1Config, server.PublicKey) {
		t.Errorf("Route node r3 config missing server peer")
	}

	// Route node should have both peer nodes
	if !strings.Contains(route1Config, peer1.PublicKey) {
		t.Errorf("Route node r3 config missing peer node r1")
	}
	if !strings.Contains(route1Config, peer2.PublicKey) {
		t.Errorf("Route node r3 config missing peer node r2")
	}

	// Route node should NOT have other route nodes
	if strings.Contains(route1Config, route2.PublicKey) {
		t.Errorf("Route node r3 config should NOT include other route node r4")
	}

	// Verify peer endpoints are included
	if !strings.Contains(route1Config, "r1.pub:51821") {
		t.Errorf("Route node r3 config missing peer r1 endpoint")
	}
	if !strings.Contains(route1Config, "r2.pub:51822") {
		t.Errorf("Route node r3 config missing peer r2 endpoint")
	}

	// Test route node r4 config
	route2Config := configs[route2.Name]

	// Route node r4 should have both peer nodes
	if !strings.Contains(route2Config, peer1.PublicKey) {
		t.Errorf("Route node r4 config missing peer node r1")
	}
	if !strings.Contains(route2Config, peer2.PublicKey) {
		t.Errorf("Route node r4 config missing peer node r2")
	}

	// Route node r4 should NOT have route node r3
	if strings.Contains(route2Config, route1.PublicKey) {
		t.Errorf("Route node r4 config should NOT include other route node r3")
	}

	// Test peer node configs remain unchanged
	peer1Config := configs[peer1.Name]

	// Peer should have server
	if !strings.Contains(peer1Config, server.PublicKey) {
		t.Errorf("Peer node r1 config missing server peer")
	}

	// Peer should have other peer
	if !strings.Contains(peer1Config, peer2.PublicKey) {
		t.Errorf("Peer node r1 config missing other peer node r2")
	}

	// Peer should NOT have route nodes (peers only connect to server and other peers)
	if strings.Contains(peer1Config, route1.PublicKey) {
		t.Errorf("Peer node r1 config should NOT include route nodes")
	}
	if strings.Contains(peer1Config, route2.PublicKey) {
		t.Errorf("Peer node r1 config should NOT include route nodes")
	}
}

func TestConfigVersionManagement(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)

	// First save - should create v1
	config1, created1, err := generator.SaveConfigVersion("testnet")
	if err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}
	if !created1 || config1.Version != 1 {
		t.Errorf("SaveConfigVersion() should create v1")
	}

	// Second save without changes - should not create new version
	config2, created2, err := generator.SaveConfigVersion("testnet")
	if err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}
	if created2 {
		t.Errorf("SaveConfigVersion() should not create new version when content unchanged")
	}
	if config2.Version != 1 {
		t.Errorf("SaveConfigVersion() returned wrong version")
	}

	// Add node - should create v2
	if _, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	config3, created3, err := generator.SaveConfigVersion("testnet")
	if err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}
	if !created3 || config3.Version != 2 {
		t.Errorf("SaveConfigVersion() should create v2 after node added")
	}
}

func TestConfigHistory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)

	// Create multiple versions
	if _, _, err := generator.SaveConfigVersion("testnet"); err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}
	if _, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode(node1) error = %v", err)
	}
	if _, _, err := generator.SaveConfigVersion("testnet"); err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}
	if _, err := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode(node2) error = %v", err)
	}
	if _, _, err := generator.SaveConfigVersion("testnet"); err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}

	// Get history
	history, err := generator.GetConfigHistory("testnet")
	if err != nil {
		t.Errorf("GetConfigHistory() error = %v", err)
		return
	}

	if len(history) != 3 {
		t.Errorf("GetConfigHistory() returned %d versions, want 3", len(history))
	}

	// Verify versions are in order
	for i, config := range history {
		if config.Version != i+1 {
			t.Errorf("GetConfigHistory() version %d at position %d", config.Version, i)
		}
	}
}

func TestGetSpecificConfigVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)

	// Create multiple versions
	if _, _, err := generator.SaveConfigVersion("testnet"); err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}
	if _, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	if _, _, err := generator.SaveConfigVersion("testnet"); err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}

	// Get specific version
	config, err := generator.GetConfig("testnet", 1)
	if err != nil {
		t.Errorf("GetConfig() error = %v", err)
		return
	}

	if config.Version != 1 {
		t.Errorf("GetConfig() returned version %d, want 1", config.Version)
	}

	// Try to get non-existent version
	_, err = generator.GetConfig("testnet", 99)
	if err == nil {
		t.Errorf("GetConfig() should return error for non-existent version")
	}
}

func TestContentHashConsistency(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := storage.Close(); err != nil {
			t.Errorf("storage.Close() error = %v", err)
		}
	}()

	validator := util.NewDefaultIPValidator()
	vnm, err := NewVirtualNetworkManager(storage, validator)
	if err != nil {
		t.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}

	if _, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	generator := NewWireGuardConfigGenerator(storage)

	// Generate configs twice without changes
	_, hash1, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Fatalf("GenerateConfigs() error = %v", err)
	}
	_, hash2, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Fatalf("GenerateConfigs() error = %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Content hash should be consistent: %s != %s", hash1, hash2)
	}

	// Add node and regenerate
	if _, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer); err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}
	_, hash3, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Fatalf("GenerateConfigs() error = %v", err)
	}

	if hash1 == hash3 {
		t.Errorf("Content hash should change when config changes")
	}
}
