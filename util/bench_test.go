package util

import (
	"fmt"
	"testing"
)

// BenchmarkNewIPPool measures pool construction across mask sizes. NewIPPool
// walks every address in the subnet to compute the last usable IP, so cost
// grows with the host-bit count — watch the /16 case.
func BenchmarkNewIPPool(b *testing.B) {
	for _, cidr := range []string{"10.0.0.0/24", "10.0.0.0/20", "10.0.0.0/16"} {
		b.Run(cidr, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := NewIPPool(cidr); err != nil {
					b.Fatalf("NewIPPool(%s) error = %v", cidr, err)
				}
			}
		})
	}
}

// BenchmarkAllocateNodeIP measures filling a pool with N node IPs. Allocation
// currently recomputes the address from the first usable IP on every call,
// so the cost of filling a pool scales worse than linearly — this benchmark
// is the tripwire for that hot path.
func BenchmarkAllocateNodeIP(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				pool, err := NewIPPool("10.0.0.0/16")
				if err != nil {
					b.Fatalf("NewIPPool() error = %v", err)
				}
				b.StartTimer()

				for j := 0; j < n; j++ {
					if _, err := pool.AllocateNodeIP(); err != nil {
						b.Fatalf("AllocateNodeIP() error = %v", err)
					}
				}
			}
		})
	}
}

// BenchmarkSyncNextIndex measures pool-reconstruction bookkeeping, which runs
// every time a manager reloads an existing network's IP pool.
func BenchmarkSyncNextIndex(b *testing.B) {
	for _, n := range []int{100, 1000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			pool, err := NewIPPool("10.0.0.0/16")
			if err != nil {
				b.Fatalf("NewIPPool() error = %v", err)
			}
			for j := 0; j < n; j++ {
				if _, err := pool.AllocateNodeIP(); err != nil {
					b.Fatalf("AllocateNodeIP() error = %v", err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pool.SyncNextIndex()
			}
		})
	}
}

// BenchmarkGenerateWireGuardKeys measures key-pair generation cost.
func BenchmarkGenerateWireGuardKeys(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := GenerateWireGuardKeys(); err != nil {
			b.Fatalf("GenerateWireGuardKeys() error = %v", err)
		}
	}
}
