package wedev

import (
	"fmt"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

func TestCreateNetwork(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net1, err := sm.CreateNetwork("network1", "10.0.0.0/24")
	if err != nil {
		t.Errorf("CreateNetwork() error = %v", err)
		return
	}
	if net1.Name != "network1" || net1.CIDR != "10.0.0.0/24" {
		t.Errorf("CreateNetwork() returned unexpected values")
	}

	// Try to create duplicate - should fail
	_, err = sm.CreateNetwork("network1", "10.1.0.0/24")
	if err == nil {
		t.Errorf("CreateNetwork() should fail with duplicate name")
	}
}

func TestGetNetworkByName(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := sm.Close(); err != nil {
			t.Errorf("sm.Close() error = %v", err)
		}
	}()

	// Create a network
	expected, err := sm.CreateNetwork("testnet", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}

	// Retrieve it
	got, err := sm.GetNetworkByName("testnet")
	if err != nil {
		t.Errorf("GetNetworkByName() error = %v", err)
		return
	}
	if got.ID != expected.ID {
		t.Errorf("GetNetworkByName() returned wrong network")
	}

	// Try to get non-existent network
	_, err = sm.GetNetworkByName("nonexistent")
	if err == nil {
		t.Errorf("GetNetworkByName() should return error for non-existent network")
	}
}

func TestListNetworks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := sm.Close(); err != nil {
			t.Errorf("sm.Close() error = %v", err)
		}
	}()

	// Create multiple networks
	if _, err := sm.CreateNetwork("net1", "10.0.0.0/24"); err != nil {
		t.Fatalf("CreateNetwork(net1) error = %v", err)
	}
	if _, err := sm.CreateNetwork("net2", "10.1.0.0/24"); err != nil {
		t.Fatalf("CreateNetwork(net2) error = %v", err)
	}
	if _, err := sm.CreateNetwork("net3", "10.2.0.0/24"); err != nil {
		t.Fatalf("CreateNetwork(net3) error = %v", err)
	}

	// List them
	networks, err := sm.ListNetworks()
	if err != nil {
		t.Errorf("ListNetworks() error = %v", err)
		return
	}
	if len(networks) != 3 {
		t.Errorf("ListNetworks() returned %d networks, want 3", len(networks))
	}
}

func TestDeleteNetwork_CascadeDelete(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := sm.Close(); err != nil {
			t.Errorf("sm.Close() error = %v", err)
		}
	}()

	// Create network with server and node
	net, err := sm.CreateNetwork("testnet", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}
	if _, err := sm.CreateServer(net.ID, "server1", "192.168.1.1", 51820, "10.0.0.1", "pk1", "pub1"); err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}
	if _, err := sm.CreateNode(net.ID, "node1", "192.168.1.2", 51821, "10.0.0.2", NodeTypePeer, "pk2", "pub2"); err != nil {
		t.Fatalf("CreateNode() error = %v", err)
	}

	// Delete network
	err = sm.DeleteNetwork("testnet")
	if err != nil {
		t.Errorf("DeleteNetwork() error = %v", err)
		return
	}

	// Verify network is gone
	_, err = sm.GetNetworkByName("testnet")
	if err == nil {
		t.Errorf("DeleteNetwork() should remove the network")
	}

	// Verify server is gone
	_, err = sm.GetServerByName(net.ID, "server1")
	if err == nil {
		t.Errorf("DeleteNetwork() should cascade delete server")
	}

	// Verify node is gone
	_, err = sm.GetNodeByName(net.ID, "node1")
	if err == nil {
		t.Errorf("DeleteNetwork() should cascade delete node")
	}
}

func TestDeleteNetwork_CascadeDeleteMultipleNodesAndServers(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create network
	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	// Create server
	sm.CreateServer(net.ID, "server1", "192.168.1.1", 51820, "10.0.0.1", "pk1", "pub1")

	// Create multiple nodes with different types
	sm.CreateNode(net.ID, "node1", "192.168.1.2", 51821, "10.0.0.2", NodeTypePeer, "pk2", "pub2")
	sm.CreateNode(net.ID, "node2", "192.168.1.3", 51822, "10.0.0.3", NodeTypePeer, "pk3", "pub3")
	sm.CreateNode(net.ID, "node3", "192.168.1.4", 51823, "10.0.0.4", NodeTypeRoute, "pk4", "pub4")

	// Verify all resources exist before deletion
	nodes, err := sm.ListNodesByNetworkID(net.ID)
	if err != nil || len(nodes) != 3 {
		t.Fatalf("Expected 3 nodes, got %d", len(nodes))
	}

	// Delete network
	err = sm.DeleteNetwork("testnet")
	if err != nil {
		t.Fatalf("DeleteNetwork() error = %v", err)
	}

	// Verify network is gone
	_, err = sm.GetNetworkByName("testnet")
	if err == nil {
		t.Errorf("DeleteNetwork() should remove the network")
	}

	// Verify all nodes are gone
	for _, nodeName := range []string{"node1", "node2", "node3"} {
		_, err = sm.GetNodeByName(net.ID, nodeName)
		if err == nil {
			t.Errorf("DeleteNetwork() should cascade delete %s", nodeName)
		}
	}

	// Verify server is gone
	_, err = sm.GetServerByName(net.ID, "server1")
	if err == nil {
		t.Errorf("DeleteNetwork() should cascade delete server")
	}
}

func TestDeleteNetwork_EmptyNetwork(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create empty network (no servers, no nodes)
	_, err = sm.CreateNetwork("emptynet", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}

	// Delete empty network - should succeed
	err = sm.DeleteNetwork("emptynet")
	if err != nil {
		t.Errorf("DeleteNetwork() should delete empty network, got error: %v", err)
	}

	// Verify network is gone
	_, err = sm.GetNetworkByName("emptynet")
	if err == nil {
		t.Errorf("DeleteNetwork() should remove the network")
	}
}

func TestDeleteNetwork_MultipleNetworks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create two networks with same-named resources
	net1, _ := sm.CreateNetwork("network1", "10.0.1.0/24")
	net2, _ := sm.CreateNetwork("network2", "10.0.2.0/24")

	// Add servers and nodes to both
	sm.CreateServer(net1.ID, "server1", "192.168.1.1", 51820, "10.0.1.1", "pk1", "pub1")
	sm.CreateServer(net2.ID, "server1", "192.168.2.1", 51820, "10.0.2.1", "pk2", "pub2")

	sm.CreateNode(net1.ID, "node1", "192.168.1.2", 51821, "10.0.1.2", NodeTypePeer, "pk3", "pub3")
	sm.CreateNode(net2.ID, "node1", "192.168.2.2", 51821, "10.0.2.2", NodeTypePeer, "pk4", "pub4")

	// Delete first network
	err = sm.DeleteNetwork("network1")
	if err != nil {
		t.Fatalf("DeleteNetwork(network1) error = %v", err)
	}

	// Verify net1 resources are gone
	_, err = sm.GetNetworkByName("network1")
	if err == nil {
		t.Errorf("network1 should be deleted")
	}
	_, err = sm.GetServerByName(net1.ID, "server1")
	if err == nil {
		t.Errorf("server1 from network1 should be deleted")
	}
	_, err = sm.GetNodeByName(net1.ID, "node1")
	if err == nil {
		t.Errorf("node1 from network1 should be deleted")
	}

	// Verify net2 resources still exist
	net2Check, err := sm.GetNetworkByName("network2")
	if err != nil || net2Check == nil {
		t.Errorf("network2 should still exist")
	}
	server2, err := sm.GetServerByName(net2.ID, "server1")
	if err != nil || server2 == nil {
		t.Errorf("server1 from network2 should still exist")
	}
	node2, err := sm.GetNodeByName(net2.ID, "node1")
	if err != nil || node2 == nil {
		t.Errorf("node1 from network2 should still exist")
	}
}

func TestDeleteNode_NameIndexCleanup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	sm.CreateNode(net.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk", "pub")

	// Delete node
	err = sm.DeleteNode(net.ID, "node1")
	if err != nil {
		t.Errorf("DeleteNode() error = %v", err)
	}

	// Verify node is gone
	_, err = sm.GetNodeByName(net.ID, "node1")
	if err == nil {
		t.Errorf("DeleteNode() should remove the node")
	}

	// Should be able to create a new node with same name after deletion
	_, err = sm.CreateNode(net.ID, "node1", "192.168.1.2", 51822, "10.0.0.3", NodeTypeRoute, "pk2", "pub2")
	if err != nil {
		t.Errorf("CreateNode() should succeed after deletion, got error: %v", err)
	}
}

func TestCreateServer(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := sm.Close(); err != nil {
			t.Errorf("sm.Close() error = %v", err)
		}
	}()

	net, err := sm.CreateNetwork("testnet", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}

	tests := []struct {
		name    string
		srvName string
		wantErr bool
	}{
		{"create valid server", "server1", false},
		{"create duplicate server", "server1", true},
		{"create second server same network", "server2", true},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.CreateServer(net.ID, tt.srvName, "192.168.1.1", 51820, "10.0.0.1", "pk", "pub")
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateServer() iteration %d error = %v, wantErr %v", i, err, tt.wantErr)
			}
		})
	}
}

func TestGetServerByNetworkID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer func() {
		if err := sm.Close(); err != nil {
			t.Errorf("sm.Close() error = %v", err)
		}
	}()

	net, err := sm.CreateNetwork("testnet", "10.0.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork() error = %v", err)
	}
	expected, err := sm.CreateServer(net.ID, "server1", "192.168.1.1", 51820, "10.0.0.1", "pk", "pub")
	if err != nil {
		t.Fatalf("CreateServer() error = %v", err)
	}

	got, err := sm.GetServerByNetworkID(net.ID)
	if err != nil {
		t.Errorf("GetServerByNetworkID() error = %v", err)
		return
	}
	if got.ID != expected.ID {
		t.Errorf("GetServerByNetworkID() returned wrong server")
	}

	// Try to get server for network with no server
	net2, err := sm.CreateNetwork("testnet2", "10.1.0.0/24")
	if err != nil {
		t.Fatalf("CreateNetwork(testnet2) error = %v", err)
	}
	_, err = sm.GetServerByNetworkID(net2.ID)
	if err == nil {
		t.Errorf("GetServerByNetworkID() should return error when no server exists")
	}
}

func TestCreateNode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	tests := []struct {
		name     string
		nodeName string
		nodeType NodeType
		wantErr  bool
	}{
		{"create peer node", "node1", NodeTypePeer, false},
		{"create route node", "node2", NodeTypeRoute, false},
		{"duplicate name", "node1", NodeTypePeer, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.CreateNode(net.ID, tt.nodeName, "192.168.1.1", 51821, "10.0.0.2", tt.nodeType, "pk", "pub")
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListNodesByNetworkID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net1, _ := sm.CreateNetwork("net1", "10.0.0.0/24")
	net2, _ := sm.CreateNetwork("net2", "10.1.0.0/24")

	if _, err := sm.CreateNode(net1.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk1", "pub1"); err != nil {
		t.Fatalf("CreateNode(node1) error = %v", err)
	}
	if _, err := sm.CreateNode(net1.ID, "node2", "192.168.1.2", 51822, "10.0.0.3", NodeTypePeer, "pk2", "pub2"); err != nil {
		t.Fatalf("CreateNode(node2) error = %v", err)
	}
	if _, err := sm.CreateNode(net2.ID, "node3", "192.168.1.3", 51823, "10.1.0.2", NodeTypePeer, "pk3", "pub3"); err != nil {
		t.Fatalf("CreateNode(node3) error = %v", err)
	}

	nodes, err := sm.ListNodesByNetworkID(net1.ID)
	if err != nil {
		t.Errorf("ListNodesByNetworkID() error = %v", err)
		return
	}
	if len(nodes) != 2 {
		t.Errorf("ListNodesByNetworkID() returned %d nodes, want 2", len(nodes))
	}
}

func TestDeleteNode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	sm.CreateNode(net.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk", "pub")

	err = sm.DeleteNode(net.ID, "node1")
	if err != nil {
		t.Errorf("DeleteNode() error = %v", err)
		return
	}

	_, err = sm.GetNodeByName(net.ID, "node1")
	if err == nil {
		t.Errorf("DeleteNode() should remove the node")
	}
}

func TestSaveConfigVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	configs := map[string]string{
		"server.conf": "[Interface]\nPrivateKey = test",
		"node1.conf":  "[Interface]\nPrivateKey = test2",
	}

	config, err := sm.SaveConfigVersion(net.ID, "hash1", configs)
	if err != nil {
		t.Errorf("SaveConfigVersion() error = %v", err)
		return
	}

	if config.Version != 1 {
		t.Errorf("SaveConfigVersion() version = %v, want 1", config.Version)
	}

	// Save another version
	config2, _ := sm.SaveConfigVersion(net.ID, "hash2", configs)
	if config2.Version != 2 {
		t.Errorf("SaveConfigVersion() second version = %v, want 2", config2.Version)
	}
}

func TestGetLatestConfigVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	configs := map[string]string{"server.conf": "test"}

	if _, err := sm.SaveConfigVersion(net.ID, "hash1", configs); err != nil {
		t.Fatalf("SaveConfigVersion(hash1) error = %v", err)
	}
	if _, err := sm.SaveConfigVersion(net.ID, "hash2", configs); err != nil {
		t.Fatalf("SaveConfigVersion(hash2) error = %v", err)
	}
	if _, err := sm.SaveConfigVersion(net.ID, "hash3", configs); err != nil {
		t.Fatalf("SaveConfigVersion(hash3) error = %v", err)
	}

	latest, err := sm.GetLatestConfigVersion(net.ID)
	if err != nil {
		t.Errorf("GetLatestConfigVersion() error = %v", err)
		return
	}

	if latest.Version != 3 {
		t.Errorf("GetLatestConfigVersion() version = %v, want 3", latest.Version)
	}
}

func TestListConfigVersions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	configs := map[string]string{"server.conf": "test"}

	sm.SaveConfigVersion(net.ID, "hash1", configs)
	sm.SaveConfigVersion(net.ID, "hash2", configs)

	versions, err := sm.ListConfigVersions(net.ID)
	if err != nil {
		t.Errorf("ListConfigVersions() error = %v", err)
		return
	}

	if len(versions) != 2 {
		t.Errorf("ListConfigVersions() returned %d versions, want 2", len(versions))
	}
}

func TestConfigVersionStoresContent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	// Create configs with actual WireGuard content
	configs := map[string]string{
		"server": "[Interface]\nPrivateKey = server_private_key\nAddress = 10.0.0.1/32\nListenPort = 51820\n",
		"node1":  "[Interface]\nPrivateKey = node1_private_key\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = server_public_key\nEndpoint = server.com:51820\n",
		"node2":  "[Interface]\nPrivateKey = node2_private_key\nAddress = 10.0.0.3/32\n[Peer]\nPublicKey = server_public_key\nEndpoint = server.com:51820\n",
	}

	// Save config version
	version, err := sm.SaveConfigVersion(net.ID, "test_hash", configs)
	if err != nil {
		t.Fatalf("SaveConfigVersion() error = %v", err)
	}

	// Verify configs are stored
	if len(version.Configs) != 3 {
		t.Errorf("SaveConfigVersion() stored %d configs, want 3", len(version.Configs))
	}

	// Verify each config content is stored correctly
	for name, expectedContent := range configs {
		actualContent, exists := version.Configs[name]
		if !exists {
			t.Errorf("Config %q not found in stored version", name)
			continue
		}
		if actualContent != expectedContent {
			t.Errorf("Config %q content mismatch:\ngot:  %q\nwant: %q", name, actualContent, expectedContent)
		}
	}

	// Retrieve config version and verify content is persisted
	retrieved, err := sm.GetConfigVersion(net.ID, version.Version)
	if err != nil {
		t.Fatalf("GetConfigVersion() error = %v", err)
	}

	if len(retrieved.Configs) != 3 {
		t.Errorf("Retrieved version has %d configs, want 3", len(retrieved.Configs))
	}

	// Verify retrieved configs match original
	for name, expectedContent := range configs {
		actualContent, exists := retrieved.Configs[name]
		if !exists {
			t.Errorf("Config %q not found in retrieved version", name)
			continue
		}
		if actualContent != expectedContent {
			t.Errorf("Retrieved config %q content mismatch:\ngot:  %q\nwant: %q", name, actualContent, expectedContent)
		}
	}
}

func TestTransactionConsistency(t *testing.T) {
	// Test that BoltDB transactions maintain consistency between primary and secondary buckets
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := bbolt.Open(dbPath, 0o600, nil)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize buckets
	if err := db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("items")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("items_by_name")); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("Failed to initialize buckets: %v", err)
	}

	// Test successful transaction
	err = db.Update(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		itemsByName := tx.Bucket([]byte("items_by_name"))

		data := []byte(`{"id":"1","name":"test"}`)
		items.Put([]byte("1"), data)
		itemsByName.Put([]byte("test"), []byte("1"))
		return nil
	})

	if err != nil {
		t.Errorf("Transaction failed: %v", err)
	}

	// Verify both buckets have data
	if err := db.View(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		itemsByName := tx.Bucket([]byte("items_by_name"))

		if items.Get([]byte("1")) == nil {
			t.Errorf("Primary bucket missing data after transaction")
		}
		if itemsByName.Get([]byte("test")) == nil {
			t.Errorf("Secondary bucket missing data after transaction")
		}

		return nil
	}); err != nil {
		t.Errorf("View transaction failed: %v", err)
	}

	// Test failed transaction (return error)
	err = db.Update(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		itemsByName := tx.Bucket([]byte("items_by_name"))

		data := []byte(`{"id":"2","name":"test2"}`)
		items.Put([]byte("2"), data)
		itemsByName.Put([]byte("test2"), []byte("2"))
		return fmt.Errorf("simulated error")
	})

	if err == nil {
		t.Errorf("Transaction should have failed")
	}

	// Verify failed transaction didn't write anything
	if err := db.View(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		if items.Get([]byte("2")) != nil {
			t.Errorf("Failed transaction should not persist primary bucket changes")
		}
		return nil
	}); err != nil {
		t.Errorf("View transaction failed: %v", err)
	}
}

func TestUpdateServer(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	server, _ := sm.CreateServer(net.ID, "server1", "192.168.1.1", 51820, "10.0.0.1", "pk", "pub")

	err = sm.UpdateServer(server.ID, "192.168.1.100", 51821)
	if err != nil {
		t.Errorf("UpdateServer() error = %v", err)
		return
	}

	updated, _ := sm.GetServerByName(net.ID, "server1")
	if updated.PublicAddress != "192.168.1.100" || updated.Port != 51821 {
		t.Errorf("UpdateServer() did not update values correctly")
	}
}

func TestUpdateNode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	node, _ := sm.CreateNode(net.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk", "pub")

	err = sm.UpdateNode(node.ID, "192.168.1.100", 51822, NodeTypeRoute)
	if err != nil {
		t.Errorf("UpdateNode() error = %v", err)
		return
	}

	updated, _ := sm.GetNodeByName(net.ID, "node1")
	if updated.PublicAddress != "192.168.1.100" || updated.Port != 51822 || updated.Type != NodeTypeRoute {
		t.Errorf("UpdateNode() did not update values correctly")
	}
}

// TestServerNameScopedToNetwork verifies server names are scoped per network
func TestServerNameScopedToNetwork(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create two networks
	net1, _ := sm.CreateNetwork("network1", "10.0.1.0/24")
	net2, _ := sm.CreateNetwork("network2", "10.0.2.0/24")

	// Create server with same name in both networks - should succeed
	server1, err := sm.CreateServer(net1.ID, "server1", "s1.net1.com", 51820, "10.0.1.1", "pk1", "pub1")
	if err != nil {
		t.Errorf("CreateServer() in net1 error = %v", err)
	}

	server2, err := sm.CreateServer(net2.ID, "server1", "s1.net2.com", 51820, "10.0.2.1", "pk2", "pub2")
	if err != nil {
		t.Errorf("CreateServer() in net2 error = %v", err)
	}

	// Verify both servers exist and have different IDs and addresses
	if server1.ID == server2.ID {
		t.Errorf("Servers in different networks should have different IDs")
	}
	if server1.PublicAddress == server2.PublicAddress {
		t.Errorf("Servers should have different public addresses")
	}

	// Verify we can retrieve each by name within its network
	retrieved1, err := sm.GetServerByName(net1.ID, "server1")
	if err != nil || retrieved1.PublicAddress != "s1.net1.com" {
		t.Errorf("Failed to retrieve server1 from net1")
	}

	retrieved2, err := sm.GetServerByName(net2.ID, "server1")
	if err != nil || retrieved2.PublicAddress != "s1.net2.com" {
		t.Errorf("Failed to retrieve server1 from net2")
	}

	// Verify duplicate server name in same network fails
	_, err = sm.CreateServer(net1.ID, "server1", "s1-dup.net1.com", 51821, "10.0.1.2", "pk3", "pub3")
	if err == nil {
		t.Errorf("CreateServer() should fail with duplicate name in same network")
	}
}

// TestNodeNameScopedToNetwork verifies node names are scoped per network
func TestNodeNameScopedToNetwork(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create two networks
	net1, _ := sm.CreateNetwork("network1", "10.0.1.0/24")
	net2, _ := sm.CreateNetwork("network2", "10.0.2.0/24")

	// Create node with same name in both networks - should succeed
	node1, err := sm.CreateNode(net1.ID, "node1", "192.168.1.1", 51821, "10.0.1.2", NodeTypePeer, "pk1", "pub1")
	if err != nil {
		t.Errorf("CreateNode() in net1 error = %v", err)
	}

	node2, err := sm.CreateNode(net2.ID, "node1", "192.168.2.1", 51821, "10.0.2.2", NodeTypePeer, "pk2", "pub2")
	if err != nil {
		t.Errorf("CreateNode() in net2 error = %v", err)
	}

	// Verify both nodes exist and have different IDs and addresses
	if node1.ID == node2.ID {
		t.Errorf("Nodes in different networks should have different IDs")
	}
	if node1.VirtualIP == node2.VirtualIP {
		t.Errorf("Nodes in different networks should have different virtual IPs")
	}

	// Verify we can retrieve each by name within its network
	retrieved1, err := sm.GetNodeByName(net1.ID, "node1")
	if err != nil || retrieved1.PublicAddress != "192.168.1.1" {
		t.Errorf("Failed to retrieve node1 from net1")
	}

	retrieved2, err := sm.GetNodeByName(net2.ID, "node1")
	if err != nil || retrieved2.PublicAddress != "192.168.2.1" {
		t.Errorf("Failed to retrieve node1 from net2")
	}

	// Verify duplicate node name in same network fails
	_, err = sm.CreateNode(net1.ID, "node1", "192.168.1.2", 51822, "10.0.1.3", NodeTypePeer, "pk3", "pub3")
	if err == nil {
		t.Errorf("CreateNode() should fail with duplicate name in same network")
	}
}

// TestNetworkDeletionCleansUpNameIndices verifies network deletion removes name indices
func TestNetworkDeletionCleansUpNameIndices(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create network with server and node
	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	sm.CreateServer(net.ID, "server1", "s1.com", 51820, "10.0.0.1", "pk1", "pub1")
	sm.CreateNode(net.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk2", "pub2")

	// Delete network
	err = sm.DeleteNetwork("testnet")
	if err != nil {
		t.Fatalf("DeleteNetwork() error = %v", err)
	}

	// Create new network with same server and node names - should succeed
	net2, _ := sm.CreateNetwork("testnet2", "10.0.1.0/24")
	_, err = sm.CreateServer(net2.ID, "server1", "s1.new.com", 51820, "10.0.1.1", "pk3", "pub3")
	if err != nil {
		t.Errorf("CreateServer() should succeed after network deletion, got error: %v", err)
	}

	_, err = sm.CreateNode(net2.ID, "node1", "192.168.2.1", 51821, "10.0.1.2", NodeTypePeer, "pk4", "pub4")
	if err != nil {
		t.Errorf("CreateNode() should succeed after network deletion, got error: %v", err)
	}

	// Verify old entries are gone by checking name index directly
	err = sm.db.View(func(tx *bbolt.Tx) error {
		serversByName := tx.Bucket([]byte(BucketServersByName))
		nodesByName := tx.Bucket([]byte(BucketNodesByName))

		// Old keys should not exist
		oldServerKey := net.ID + ":server1"
		oldNodeKey := net.ID + ":node1"

		if serversByName.Get([]byte(oldServerKey)) != nil {
			t.Errorf("Old server name index should be deleted")
		}
		if nodesByName.Get([]byte(oldNodeKey)) != nil {
			t.Errorf("Old node name index should be deleted")
		}

		// New keys should exist
		newServerKey := net2.ID + ":server1"
		newNodeKey := net2.ID + ":node1"

		if serversByName.Get([]byte(newServerKey)) == nil {
			t.Errorf("New server name index should exist")
		}
		if nodesByName.Get([]byte(newNodeKey)) == nil {
			t.Errorf("New node name index should exist")
		}

		return nil
	})
	if err != nil {
		t.Errorf("Database view error: %v", err)
	}
}
