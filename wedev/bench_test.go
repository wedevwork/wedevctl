package wedev

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/wedevctl/util"
)

// benchNetwork builds a network with a server and n peer nodes on a temp-file
// BoltDB, returning the manager and storage ready for measurement.
func benchNetwork(b *testing.B, n int) (*VirtualNetworkManager, *StorageManager) {
	b.Helper()

	dbPath := filepath.Join(b.TempDir(), "bench.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		b.Fatalf("NewStorageManager() error = %v", err)
	}
	b.Cleanup(func() { sm.Close() })

	vnm, err := NewVirtualNetworkManager(sm, util.NewDefaultIPValidator())
	if err != nil {
		b.Fatalf("NewVirtualNetworkManager() error = %v", err)
	}
	if _, err := vnm.CreateVirtualNetwork("benchnet", "10.0.0.0/16"); err != nil {
		b.Fatalf("CreateVirtualNetwork() error = %v", err)
	}
	if _, err := vnm.CreateServer("benchnet", "srv", "vpn.example.com", 51820); err != nil {
		b.Fatalf("CreateServer() error = %v", err)
	}
	for j := 0; j < n; j++ {
		if _, err := vnm.CreateNode("benchnet", fmt.Sprintf("node%d", j), "1.2.3.4", 51820, NodeTypePeer); err != nil {
			b.Fatalf("CreateNode() error = %v", err)
		}
	}
	return vnm, sm
}

// BenchmarkGenerateConfigs measures full-network config generation. Each node's
// config enumerates the other peer nodes, so cost grows quadratically with the
// node count — this benchmark guards that hot path.
func BenchmarkGenerateConfigs(b *testing.B) {
	for _, n := range []int{10, 50, 200} {
		b.Run(fmt.Sprintf("nodes=%d", n), func(b *testing.B) {
			_, sm := benchNetwork(b, n)
			gen := NewWireGuardConfigGenerator(sm)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := gen.GenerateConfigs("benchnet", sm); err != nil {
					b.Fatalf("GenerateConfigs() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkCreateNode measures a single node creation (IP allocation, key
// generation, storage write, IP-pool persistence) against a populated network.
func BenchmarkCreateNode(b *testing.B) {
	vnm, _ := benchNetwork(b, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := vnm.CreateNode("benchnet", fmt.Sprintf("extra%d", i), "1.2.3.4", 51820, NodeTypePeer); err != nil {
			b.Fatalf("CreateNode() error = %v", err)
		}
	}
}

// BenchmarkListNodesByNetworkID measures the indexed node lookup that backs
// node listing and config generation.
func BenchmarkListNodesByNetworkID(b *testing.B) {
	_, sm := benchNetwork(b, 200)
	net, err := sm.GetNetworkByName("benchnet")
	if err != nil {
		b.Fatalf("GetNetworkByName() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sm.ListNodesByNetworkID(net.ID); err != nil {
			b.Fatalf("ListNodesByNetworkID() error = %v", err)
		}
	}
}

// seedNetworks creates n networks, each with a server, on a temp-file BoltDB
// and returns the storage plus the ID of the last network created.
func seedNetworks(b *testing.B, n int) (sm *StorageManager, lastID string) {
	b.Helper()
	dbPath := filepath.Join(b.TempDir(), "bench.db")

	var err error
	sm, err = NewStorageManager(dbPath)
	if err != nil {
		b.Fatalf("NewStorageManager() error = %v", err)
	}
	b.Cleanup(func() { sm.Close() })

	for i := 0; i < n; i++ {
		net, err := sm.CreateNetwork(fmt.Sprintf("net%d", i), "10.0.0.0/24")
		if err != nil {
			b.Fatalf("CreateNetwork() error = %v", err)
		}
		if _, err := sm.CreateServer(net.ID, "srv", "vpn.example.com", 51820, "10.0.0.1", "priv", "pub"); err != nil {
			b.Fatalf("CreateServer() error = %v", err)
		}
		lastID = net.ID
	}
	return sm, lastID
}

// BenchmarkGetServerByNetworkID measures the indexed server lookup with many
// networks present — it should stay flat regardless of the database size.
func BenchmarkGetServerByNetworkID(b *testing.B) {
	for _, n := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("networks=%d", n), func(b *testing.B) {
			sm, target := seedNetworks(b, n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := sm.GetServerByNetworkID(target); err != nil {
					b.Fatalf("GetServerByNetworkID() error = %v", err)
				}
			}
		})
	}
}

// BenchmarkSaveConfigVersion measures appending a config version to a network
// that already holds many versions.
func BenchmarkSaveConfigVersion(b *testing.B) {
	for _, n := range []int{10, 100, 500} {
		b.Run(fmt.Sprintf("existing=%d", n), func(b *testing.B) {
			dbPath := filepath.Join(b.TempDir(), "bench.db")
			sm, err := NewStorageManager(dbPath)
			if err != nil {
				b.Fatalf("NewStorageManager() error = %v", err)
			}
			b.Cleanup(func() { sm.Close() })

			net, err := sm.CreateNetwork("benchnet", "10.0.0.0/24")
			if err != nil {
				b.Fatalf("CreateNetwork() error = %v", err)
			}
			for i := 0; i < n; i++ {
				if _, err := sm.SaveConfigVersion(net.ID, fmt.Sprintf("h%d", i), map[string]string{"srv": "config"}); err != nil {
					b.Fatalf("SaveConfigVersion() error = %v", err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := sm.SaveConfigVersion(net.ID, fmt.Sprintf("x%d", i), map[string]string{"srv": "config"}); err != nil {
					b.Fatalf("SaveConfigVersion() error = %v", err)
				}
			}
		})
	}
}
