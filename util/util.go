// Package util provides utility functions and validators for wedevctl.
package util

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// IPValidator validates network names and IP addresses
type IPValidator interface {
	IsValidNetworkName(name string) error
	IsValidCIDR(cidr string) error
	IsValidPublicAddress(addr string) error
}

// DefaultIPValidator provides default validation logic
type DefaultIPValidator struct{}

// IsValidNetworkName validates network names according to design:
// - Only alphanumeric characters
// - First character must be a letter
func (v *DefaultIPValidator) IsValidNetworkName(name string) error {
	if name == "" {
		return fmt.Errorf("network name cannot be empty")
	}
	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`).MatchString(name) {
		return fmt.Errorf("network name must start with a letter and contain only alphanumeric characters")
	}
	return nil
}

// IsValidCIDR validates CIDR notation
func (v *DefaultIPValidator) IsValidCIDR(cidr string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR notation: %w", err)
	}
	if ipnet == nil {
		return fmt.Errorf("invalid CIDR: network is nil")
	}
	return nil
}

// IsValidPublicAddress validates public address (domain or IP)
func (v *DefaultIPValidator) IsValidPublicAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("public address cannot be empty")
	}
	// Try to parse as IP first
	if ip := net.ParseIP(addr); ip != nil {
		return nil
	}
	// Otherwise, it should be a valid domain name
	// Basic validation: no spaces, contains at least one dot or is localhost
	if strings.Contains(addr, " ") {
		return fmt.Errorf("public address cannot contain spaces")
	}
	if !strings.Contains(addr, ".") && addr != "localhost" {
		return fmt.Errorf("public address must be a valid IP or domain name")
	}
	return nil
}

// NewDefaultIPValidator creates a new DefaultIPValidator
func NewDefaultIPValidator() *DefaultIPValidator {
	return &DefaultIPValidator{}
}

// IPPool manages IP allocation for a virtual network
type IPPool struct {
	networkCIDR string          // Network CIDR (e.g., "10.0.0.0/24")
	serverIP    string          // Reserved server IP (first usable)
	allocated   map[string]bool // Current allocated IPs: ip -> true
	recycled    []string        // Recycled IPs (for reuse)
	nextIndex   int             // Next index to allocate from
	firstUsable string
	lastUsable  string
	totalUsable int
}

// NewIPPool creates a new IP pool
func NewIPPool(networkCIDR string) (*IPPool, error) {
	// Parse CIDR
	_, ipnet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	// Ensure IPv4 only
	if ipnet.IP.To4() == nil {
		return nil, fmt.Errorf("only IPv4 subnets are supported")
	}

	// Calculate usable IPs (all IPs except network address and broadcast)
	ones, bits := ipnet.Mask.Size()
	// #nosec G115
	shift := uint(bits - ones)
	totalIPs := int64(1) << shift
	totalIPsInt := int(totalIPs)

	// For IPv4 subnets with more than 2 IPs, first IP is network, last is broadcast
	// For /31 and /32, all are usable
	if totalIPsInt > 2 {
		// First usable IP is network + 1. lastUsable mirrors the previous
		// computation (network + totalIPs - 1); both are O(1) arithmetic
		// instead of walking every address in the subnet.
		networkVal, ok := ipToUint32(ipnet.IP.String())
		if !ok {
			return nil, fmt.Errorf("invalid network address: %s", ipnet.IP.String())
		}
		firstUsable := uint32ToIP(networkVal + 1)
		// #nosec G115 -- totalIPsInt is bounded by the subnet size, far below uint32 max.
		lastUsable := uint32ToIP(networkVal + uint32(totalIPsInt) - 1)

		return &IPPool{
			networkCIDR: networkCIDR,
			serverIP:    firstUsable,
			allocated:   make(map[string]bool),
			recycled:    []string{},
			nextIndex:   1, // Start after server IP (index 0)
			firstUsable: firstUsable,
			lastUsable:  lastUsable,
			totalUsable: totalIPsInt - 2, // Exclude network and broadcast
		}, nil
	}

	return nil, fmt.Errorf("network must have at least 3 usable IPs")
}

// ipToUint32 converts a dotted-quad IPv4 string to its uint32 value.
// The boolean result is false if the string is not a valid IPv4 address.
func ipToUint32(s string) (uint32, bool) {
	ip := net.ParseIP(s)
	if ip == nil {
		return 0, false
	}
	ip = ip.To4()
	if ip == nil {
		return 0, false
	}
	return binary.BigEndian.Uint32(ip), true
}

// uint32ToIP converts a uint32 value to its dotted-quad IPv4 string.
func uint32ToIP(v uint32) string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return net.IP(b[:]).String()
}

// GetServerIP returns the reserved server IP (first usable IP)
func (p *IPPool) GetServerIP() string {
	return p.serverIP
}

// AllocateNodeIP allocates the next available IP for a node
// Returns the IP or an error if no IPs are available
func (p *IPPool) AllocateNodeIP() (string, error) {
	// Try to reuse recycled IP first
	if len(p.recycled) > 0 {
		ip := p.recycled[0]
		p.recycled = p.recycled[1:]
		p.allocated[ip] = true
		return ip, nil
	}

	// Allocate new IP if index doesn't exceed total
	if p.nextIndex >= p.totalUsable {
		return "", fmt.Errorf("no available IPs in pool (total usable: %d, allocated: %d)",
			p.totalUsable, len(p.allocated))
	}

	// The IP at nextIndex is firstUsable + nextIndex — O(1) arithmetic.
	firstVal, ok := ipToUint32(p.firstUsable)
	if !ok {
		return "", fmt.Errorf("invalid first usable IP: %s", p.firstUsable)
	}
	p.nextIndex++

	// #nosec G115 -- nextIndex is bounded by totalUsable, far below uint32 max.
	ip := uint32ToIP(firstVal + uint32(p.nextIndex-1))
	p.allocated[ip] = true
	return ip, nil
}

// MarkIPAllocated marks an existing IP as allocated in the pool.
// This is used when reconstructing the pool from existing database records.
func (p *IPPool) MarkIPAllocated(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP cannot be empty")
	}
	if p.allocated[ip] {
		return fmt.Errorf("IP %s is already allocated", ip)
	}
	p.allocated[ip] = true
	return nil
}

// SyncNextIndex updates nextIndex to point past all currently allocated IPs.
// This should be called after marking existing IPs as allocated when reconstructing the pool.
func (p *IPPool) SyncNextIndex() {
	maxIndex := 0

	firstVal, ok := ipToUint32(p.firstUsable)
	if !ok {
		return
	}

	for allocatedIP := range p.allocated {
		if allocatedIP == p.serverIP {
			continue // Server IP is at index 0, skip it
		}

		// The index of an allocated IP is its offset from the first usable
		// IP — O(1) arithmetic instead of walking the pool.
		allocVal, valid := ipToUint32(allocatedIP)
		if !valid {
			continue
		}
		index := int(allocVal - firstVal)
		if index < 0 || index > p.totalUsable {
			continue
		}

		if index > maxIndex {
			maxIndex = index
		}
	}

	// Set nextIndex to one past the highest allocated index
	p.nextIndex = maxIndex + 1
}

// ReleaseNodeIP returns an IP to the pool for recycling
func (p *IPPool) ReleaseNodeIP(ip string) error {
	if ip == p.serverIP {
		return fmt.Errorf("cannot release server IP")
	}
	if !p.allocated[ip] {
		return fmt.Errorf("IP %s is not allocated", ip)
	}

	delete(p.allocated, ip)
	p.recycled = append(p.recycled, ip)
	return nil
}

// GetAllocatedIPs returns a copy of all allocated IPs
func (p *IPPool) GetAllocatedIPs() map[string]bool {
	result := make(map[string]bool)
	for ip := range p.allocated {
		result[ip] = true
	}
	return result
}

// GetState returns current state for persistence
func (p *IPPool) GetState() *IPPoolState {
	allocated := make([]string, 0, len(p.allocated))
	for ip := range p.allocated {
		if ip != p.serverIP { // Don't include server IP in the state
			allocated = append(allocated, ip)
		}
	}
	return &IPPoolState{
		NetworkCIDR: p.networkCIDR,
		ServerIP:    p.serverIP,
		Allocated:   allocated,
		Recycled:    p.recycled,
		NextIndex:   p.nextIndex,
	}
}

// IPPoolState represents the persistent state of an IP pool
type IPPoolState struct {
	NetworkCIDR string   `json:"network_cidr"`
	ServerIP    string   `json:"server_ip"`
	Allocated   []string `json:"allocated"`
	Recycled    []string `json:"recycled"`
	NextIndex   int      `json:"next_index"`
}

// RestoreIPPool creates an IP pool from saved state.
func RestoreIPPool(state *IPPoolState) (*IPPool, error) {
	pool, err := NewIPPool(state.NetworkCIDR)
	if err != nil {
		return nil, err
	}

	// Mark server IP as allocated (server IP is always allocated)
	pool.allocated[state.ServerIP] = true

	// Restore allocated IPs
	for _, ip := range state.Allocated {
		pool.allocated[ip] = true
	}

	// Restore recycled IPs
	pool.recycled = state.Recycled

	// Restore next index
	pool.nextIndex = state.NextIndex

	return pool, nil
}

// WireGuardKeyPair represents a WireGuard key pair
type WireGuardKeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateWireGuardKeys generates a WireGuard X25519 (Curve25519) key pair.
//
// The private key is 32 cryptographically secure random bytes, clamped per
// RFC 7748 §5 (the same clamping `wg genkey` applies). The public key is the
// Curve25519 point derived from the private key, so the two keys are a real,
// mathematically linked key pair usable by WireGuard — not independent random
// values.
func GenerateWireGuardKeys() (*WireGuardKeyPair, error) {
	// Generate 32 cryptographically secure random bytes for the private key.
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp the scalar per RFC 7748 §5, matching `wg genkey`.
	privateKeyBytes[0] &= 248
	privateKeyBytes[31] &= 127
	privateKeyBytes[31] |= 64

	// Derive the public key as the X25519 point of the private key.
	priv, err := ecdh.X25519().NewPrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to construct private key: %w", err)
	}

	return &WireGuardKeyPair{
		PrivateKey: base64.StdEncoding.EncodeToString(priv.Bytes()),
		PublicKey:  base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()),
	}, nil
}

// ValidateEndpoint validates endpoint format: address:port
func ValidateEndpoint(address string, port int) error {
	if address == "" {
		return fmt.Errorf("endpoint address cannot be empty")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}
	return nil
}

// FormatEndpoint formats endpoint as address:port
func FormatEndpoint(address string, port int) string {
	return fmt.Sprintf("%s:%d", address, port)
}
