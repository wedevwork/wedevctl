package wedev

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wedevctl/util"
	"go.etcd.io/bbolt"
)

const (
	// BucketNetworks is the BoltDB bucket for network data.
	BucketNetworks = "networks"
	// BucketNetworksByName is the index bucket for networks by name.
	BucketNetworksByName = "networks_by_name"
	// BucketServers is the BoltDB bucket for server data.
	BucketServers = "servers"
	// BucketServersByName is the index bucket for servers by name (networkID:name -> server ID).
	BucketServersByName = "servers_by_name"
	// BucketServersByNetwork is the index bucket for the server of a network (networkID -> server ID).
	BucketServersByNetwork = "servers_by_network"
	// BucketNodes is the BoltDB bucket for node data.
	BucketNodes = "nodes"
	// BucketNodesByName is the index bucket for nodes by name (networkID:name -> node ID).
	BucketNodesByName = "nodes_by_name"
	// BucketNodesByNetwork is the index bucket for nodes by network (networkID:nodeID -> node ID).
	BucketNodesByNetwork = "nodes_by_network"
	// BucketConfigs is the BoltDB bucket for config data.
	BucketConfigs = "configs"
	// BucketConfigsByVer is the index bucket for config versions (networkID:paddedVersion -> config ID).
	BucketConfigsByVer = "configs_by_version"
	// BucketIPPools is the BoltDB bucket for IP pool data.
	BucketIPPools = "ip_pools"
)

// VirtualNetwork represents a virtual network
type VirtualNetwork struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CIDR      string    `json:"cidr"`
	CreatedAt time.Time `json:"created_at"`
}

// Server represents a WireGuard server
type Server struct {
	ID            string    `json:"id"`
	NetworkID     string    `json:"network_id"`
	Name          string    `json:"name"`
	PublicAddress string    `json:"public_address"`
	Port          int       `json:"port"`
	VirtualIP     string    `json:"virtual_ip"`
	PrivateKey    string    `json:"private_key"`
	PublicKey     string    `json:"public_key"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NodeType represents the type of node
type NodeType string

const (
	// NodeTypePeer represents a peer node.
	NodeTypePeer NodeType = "peer"
	// NodeTypeRoute represents a route node.
	NodeTypeRoute NodeType = "route"
)

// Node represents a node in the network
type Node struct {
	ID            string    `json:"id"`
	NetworkID     string    `json:"network_id"`
	Name          string    `json:"name"`
	PublicAddress string    `json:"public_address"`
	Port          int       `json:"port"`
	VirtualIP     string    `json:"virtual_ip"`
	Type          NodeType  `json:"type"`
	PrivateKey    string    `json:"private_key"`
	PublicKey     string    `json:"public_key"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ConfigVersion represents a snapshot of WireGuard configurations
type ConfigVersion struct {
	ID          string            `json:"id"`
	NetworkID   string            `json:"network_id"`
	Version     int               `json:"version"`
	ContentHash string            `json:"content_hash"`
	Configs     map[string]string `json:"configs"` // name -> config content
	CreatedAt   time.Time         `json:"created_at"`
}

// StorageManager handles all BoltDB operations
type StorageManager struct {
	db *bbolt.DB
}

// NewStorageManager creates a new storage manager.
func NewStorageManager(dbPath string) (*StorageManager, error) {
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	if err := db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{
			BucketNetworks, BucketNetworksByName,
			BucketServers, BucketServersByName, BucketServersByNetwork,
			BucketNodes, BucketNodesByName, BucketNodesByNetwork,
			BucketConfigs, BucketConfigsByVer,
			BucketIPPools,
		}
		for _, bucketName := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
			}
		}
		return nil
	}); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to close database after init error: %w", closeErr)
		}
		return nil, err
	}

	return &StorageManager{db: db}, nil
}

// Close closes the database
func (sm *StorageManager) Close() error {
	return sm.db.Close()
}

// padVersion formats a version number into a fixed-width, lexically sortable
// string for use in composite index keys.
func padVersion(version int) string {
	return fmt.Sprintf("%020d", version)
}

// forEachWithPrefix invokes fn for every key/value in bucket whose key starts
// with prefix, in ascending key order. It uses a cursor seek, so cost is
// proportional to the number of matching keys rather than the bucket size.
func forEachWithPrefix(bucket *bbolt.Bucket, prefix []byte, fn func(k, v []byte) error) error {
	c := bucket.Cursor()
	for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

// ========== VirtualNetwork Operations ==========

// CreateNetwork creates a new virtual network.
func (sm *StorageManager) CreateNetwork(name, cidr string) (*VirtualNetwork, error) {
	var network *VirtualNetwork

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		// Check if name already exists
		nameIdx := tx.Bucket([]byte(BucketNetworksByName))
		if nameIdx.Get([]byte(name)) != nil {
			return fmt.Errorf("network name %q already exists", name)
		}

		network = &VirtualNetwork{
			ID:        uuid.New().String(),
			Name:      name,
			CIDR:      cidr,
			CreatedAt: time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(network)
		if err != nil {
			return fmt.Errorf("failed to marshal network: %w", err)
		}
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if err := networksBucket.Put([]byte(network.ID), data); err != nil {
			return fmt.Errorf("failed to save network: %w", err)
		}

		// Save to secondary bucket (name -> id)
		nameIdx = tx.Bucket([]byte(BucketNetworksByName))
		if err := nameIdx.Put([]byte(name), []byte(network.ID)); err != nil {
			return fmt.Errorf("failed to save name index: %w", err)
		}

		return nil
	})

	return network, err
}

// GetNetworkByName retrieves a network by name
func (sm *StorageManager) GetNetworkByName(name string) (*VirtualNetwork, error) {
	var network *VirtualNetwork

	err := sm.db.View(func(tx *bbolt.Tx) error {
		// Get ID from name index
		nameIdx := tx.Bucket([]byte(BucketNetworksByName))
		id := nameIdx.Get([]byte(name))
		if id == nil {
			return fmt.Errorf("network %q not found", name)
		}

		// Get network from primary bucket
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		data := networksBucket.Get(id)
		if data == nil {
			return fmt.Errorf("network data not found")
		}

		network = &VirtualNetwork{}
		if err := json.Unmarshal(data, network); err != nil {
			return fmt.Errorf("failed to unmarshal network: %w", err)
		}

		return nil
	})

	return network, err
}

// GetNetworkByID retrieves a network by ID
func (sm *StorageManager) GetNetworkByID(id string) (*VirtualNetwork, error) {
	var network *VirtualNetwork

	err := sm.db.View(func(tx *bbolt.Tx) error {
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		data := networksBucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("network %q not found", id)
		}

		network = &VirtualNetwork{}
		if err := json.Unmarshal(data, network); err != nil {
			return fmt.Errorf("failed to unmarshal network: %w", err)
		}

		return nil
	})

	return network, err
}

// ListNetworks lists all networks
func (sm *StorageManager) ListNetworks() ([]*VirtualNetwork, error) {
	var networks []*VirtualNetwork

	err := sm.db.View(func(tx *bbolt.Tx) error {
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		return networksBucket.ForEach(func(_, v []byte) error {
			network := &VirtualNetwork{}
			if err := json.Unmarshal(v, network); err != nil {
				return err
			}
			networks = append(networks, network)
			return nil
		})
	})

	return networks, err
}

// DeleteNetwork deletes a network and all its associated resources
func (sm *StorageManager) DeleteNetwork(name string) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		// Get network ID
		nameIdx := tx.Bucket([]byte(BucketNetworksByName))
		id := nameIdx.Get([]byte(name))
		if id == nil {
			return fmt.Errorf("network %q not found", name)
		}
		idStr := string(id)
		prefix := []byte(idStr + ":")

		// Delete the server (one per network) via the by-network index.
		serversBucket := tx.Bucket([]byte(BucketServers))
		serversByName := tx.Bucket([]byte(BucketServersByName))
		serversByNetwork := tx.Bucket([]byte(BucketServersByNetwork))
		if serverID := serversByNetwork.Get([]byte(idStr)); serverID != nil {
			serverID = append([]byte(nil), serverID...)
			if data := serversBucket.Get(serverID); data != nil {
				server := &Server{}
				if err := json.Unmarshal(data, server); err != nil {
					return err
				}
				if err := serversByName.Delete([]byte(idStr + ":" + server.Name)); err != nil {
					return err
				}
			}
			if err := serversBucket.Delete(serverID); err != nil {
				return err
			}
			if err := serversByNetwork.Delete([]byte(idStr)); err != nil {
				return err
			}
		}

		// Delete all nodes via the by-network index prefix.
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		nodesByNetwork := tx.Bucket([]byte(BucketNodesByNetwork))
		var nodeIdxKeys, nodeIDs [][]byte
		if err := forEachWithPrefix(nodesByNetwork, prefix, func(k, v []byte) error {
			nodeIdxKeys = append(nodeIdxKeys, append([]byte(nil), k...))
			nodeIDs = append(nodeIDs, append([]byte(nil), v...))
			return nil
		}); err != nil {
			return err
		}
		for i, nodeID := range nodeIDs {
			if data := nodesBucket.Get(nodeID); data != nil {
				node := &Node{}
				if err := json.Unmarshal(data, node); err != nil {
					return err
				}
				if err := nodesByName.Delete([]byte(idStr + ":" + node.Name)); err != nil {
					return err
				}
			}
			if err := nodesBucket.Delete(nodeID); err != nil {
				return err
			}
			if err := nodesByNetwork.Delete(nodeIdxKeys[i]); err != nil {
				return err
			}
		}

		// Delete all config versions via the by-network index prefix.
		configsBucket := tx.Bucket([]byte(BucketConfigs))
		configsByVer := tx.Bucket([]byte(BucketConfigsByVer))
		var cfgIdxKeys, cfgIDs [][]byte
		if err := forEachWithPrefix(configsByVer, prefix, func(k, v []byte) error {
			cfgIdxKeys = append(cfgIdxKeys, append([]byte(nil), k...))
			cfgIDs = append(cfgIDs, append([]byte(nil), v...))
			return nil
		}); err != nil {
			return err
		}
		for i, cfgID := range cfgIDs {
			if err := configsBucket.Delete(cfgID); err != nil {
				return err
			}
			if err := configsByVer.Delete(cfgIdxKeys[i]); err != nil {
				return err
			}
		}

		// Delete IP pool
		ipPoolsBucket := tx.Bucket([]byte(BucketIPPools))
		if err := ipPoolsBucket.Delete([]byte(idStr)); err != nil {
			return err
		}

		// Delete network
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if err := networksBucket.Delete([]byte(idStr)); err != nil {
			return err
		}
		return nameIdx.Delete([]byte(name))
	})
}

// ========== Server Operations ==========

// CreateServer creates a new server.
func (sm *StorageManager) CreateServer(networkID, name, publicAddress string, port int, virtualIP, privateKey, publicKey string) (*Server, error) {
	var server *Server

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		// Get network to verify it exists
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if networksBucket.Get([]byte(networkID)) == nil {
			return fmt.Errorf("network %q not found", networkID)
		}

		// One server per network — O(1) check via the by-network index.
		serversByNetwork := tx.Bucket([]byte(BucketServersByNetwork))
		if serversByNetwork.Get([]byte(networkID)) != nil {
			return fmt.Errorf("server already exists for network %q", networkID)
		}

		// Check if name already exists in this network
		serversByName := tx.Bucket([]byte(BucketServersByName))
		nameKey := networkID + ":" + name
		if serversByName.Get([]byte(nameKey)) != nil {
			return fmt.Errorf("server name %q already exists", name)
		}

		server = &Server{
			ID:            uuid.New().String(),
			NetworkID:     networkID,
			Name:          name,
			PublicAddress: publicAddress,
			Port:          port,
			VirtualIP:     virtualIP,
			PrivateKey:    privateKey,
			PublicKey:     publicKey,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("failed to marshal server: %w", err)
		}
		serversBucket := tx.Bucket([]byte(BucketServers))
		if err := serversBucket.Put([]byte(server.ID), data); err != nil {
			return fmt.Errorf("failed to save server: %w", err)
		}

		// Save to index buckets (name -> id, networkID -> id)
		if err := serversByName.Put([]byte(nameKey), []byte(server.ID)); err != nil {
			return fmt.Errorf("failed to save name index: %w", err)
		}
		if err := serversByNetwork.Put([]byte(networkID), []byte(server.ID)); err != nil {
			return fmt.Errorf("failed to save network index: %w", err)
		}

		return nil
	})

	return server, err
}

// GetServerByName retrieves a server by name within a network
func (sm *StorageManager) GetServerByName(networkID, name string) (*Server, error) {
	var server *Server

	err := sm.db.View(func(tx *bbolt.Tx) error {
		serversByName := tx.Bucket([]byte(BucketServersByName))
		nameKey := networkID + ":" + name
		id := serversByName.Get([]byte(nameKey))
		if id == nil {
			return fmt.Errorf("server %q not found", name)
		}

		serversBucket := tx.Bucket([]byte(BucketServers))
		data := serversBucket.Get(id)
		if data == nil {
			return fmt.Errorf("server data not found")
		}

		server = &Server{}
		if err := json.Unmarshal(data, server); err != nil {
			return fmt.Errorf("failed to unmarshal server: %w", err)
		}

		return nil
	})

	return server, err
}

// GetServerByNetworkID retrieves the server for a network
func (sm *StorageManager) GetServerByNetworkID(networkID string) (*Server, error) {
	var server *Server

	err := sm.db.View(func(tx *bbolt.Tx) error {
		serversByNetwork := tx.Bucket([]byte(BucketServersByNetwork))
		id := serversByNetwork.Get([]byte(networkID))
		if id == nil {
			return fmt.Errorf("no server found for network %q", networkID)
		}

		serversBucket := tx.Bucket([]byte(BucketServers))
		data := serversBucket.Get(id)
		if data == nil {
			return fmt.Errorf("no server found for network %q", networkID)
		}

		server = &Server{}
		if err := json.Unmarshal(data, server); err != nil {
			return fmt.Errorf("failed to unmarshal server: %w", err)
		}
		return nil
	})

	return server, err
}

// UpdateServer updates server information.
func (sm *StorageManager) UpdateServer(id, publicAddress string, port int) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		serversBucket := tx.Bucket([]byte(BucketServers))
		data := serversBucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("server not found")
		}

		server := &Server{}
		if err := json.Unmarshal(data, server); err != nil {
			return err
		}

		server.PublicAddress = publicAddress
		server.Port = port
		server.UpdatedAt = time.Now()

		updated, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("failed to marshal server: %w", err)
		}
		return serversBucket.Put([]byte(id), updated)
	})
}

// DeleteServer deletes a server
func (sm *StorageManager) DeleteServer(networkID string) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		serversByNetwork := tx.Bucket([]byte(BucketServersByNetwork))
		id := serversByNetwork.Get([]byte(networkID))
		if id == nil {
			return fmt.Errorf("server not found for network")
		}
		id = append([]byte(nil), id...)

		serversBucket := tx.Bucket([]byte(BucketServers))
		serversByName := tx.Bucket([]byte(BucketServersByName))

		if data := serversBucket.Get(id); data != nil {
			server := &Server{}
			if err := json.Unmarshal(data, server); err != nil {
				return err
			}
			if err := serversByName.Delete([]byte(networkID + ":" + server.Name)); err != nil {
				return err
			}
		}
		if err := serversBucket.Delete(id); err != nil {
			return err
		}
		return serversByNetwork.Delete([]byte(networkID))
	})
}

// ========== Node Operations ==========

// CreateNode creates a new node.
func (sm *StorageManager) CreateNode(networkID, name, publicAddress string, port int, virtualIP string, nodeType NodeType, privateKey, publicKey string) (*Node, error) {
	var node *Node

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		// Get network to verify it exists
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if networksBucket.Get([]byte(networkID)) == nil {
			return fmt.Errorf("network %q not found", networkID)
		}

		// Check if name already exists in this network
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		nameKey := networkID + ":" + name
		if nodesByName.Get([]byte(nameKey)) != nil {
			return fmt.Errorf("node name %q already exists", name)
		}

		node = &Node{
			ID:            uuid.New().String(),
			NetworkID:     networkID,
			Name:          name,
			PublicAddress: publicAddress,
			Port:          port,
			VirtualIP:     virtualIP,
			Type:          nodeType,
			PrivateKey:    privateKey,
			PublicKey:     publicKey,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("failed to marshal node: %w", err)
		}
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		if err := nodesBucket.Put([]byte(node.ID), data); err != nil {
			return fmt.Errorf("failed to save node: %w", err)
		}

		// Save to index buckets (name -> id, networkID:nodeID -> id)
		if err := nodesByName.Put([]byte(nameKey), []byte(node.ID)); err != nil {
			return fmt.Errorf("failed to save name index: %w", err)
		}
		nodesByNetwork := tx.Bucket([]byte(BucketNodesByNetwork))
		if err := nodesByNetwork.Put([]byte(networkID+":"+node.ID), []byte(node.ID)); err != nil {
			return fmt.Errorf("failed to save network index: %w", err)
		}

		return nil
	})

	return node, err
}

// GetNodeByName retrieves a node by name within a specific network
func (sm *StorageManager) GetNodeByName(networkID, name string) (*Node, error) {
	var node *Node

	err := sm.db.View(func(tx *bbolt.Tx) error {
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		nameKey := networkID + ":" + name
		id := nodesByName.Get([]byte(nameKey))
		if id == nil {
			return fmt.Errorf("node %q not found", name)
		}

		nodesBucket := tx.Bucket([]byte(BucketNodes))
		data := nodesBucket.Get(id)
		if data == nil {
			return fmt.Errorf("node data not found")
		}

		node = &Node{}
		if err := json.Unmarshal(data, node); err != nil {
			return fmt.Errorf("failed to unmarshal node: %w", err)
		}

		return nil
	})

	return node, err
}

// ListNodesByNetworkID lists all nodes in a network
func (sm *StorageManager) ListNodesByNetworkID(networkID string) ([]*Node, error) {
	var nodes []*Node

	err := sm.db.View(func(tx *bbolt.Tx) error {
		nodesByNetwork := tx.Bucket([]byte(BucketNodesByNetwork))
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		return forEachWithPrefix(nodesByNetwork, []byte(networkID+":"), func(_, v []byte) error {
			data := nodesBucket.Get(v)
			if data == nil {
				return nil
			}
			node := &Node{}
			if err := json.Unmarshal(data, node); err != nil {
				return err
			}
			nodes = append(nodes, node)
			return nil
		})
	})

	return nodes, err
}

// UpdateNode updates node information.
func (sm *StorageManager) UpdateNode(id, publicAddress string, port int, nodeType NodeType) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		data := nodesBucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("node not found")
		}

		node := &Node{}
		if err := json.Unmarshal(data, node); err != nil {
			return err
		}

		node.PublicAddress = publicAddress
		node.Port = port
		node.Type = nodeType
		node.UpdatedAt = time.Now()

		updated, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("failed to marshal node: %w", err)
		}
		return nodesBucket.Put([]byte(id), updated)
	})
}

// DeleteNode deletes a node
func (sm *StorageManager) DeleteNode(networkID, name string) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		nameKey := networkID + ":" + name
		id := nodesByName.Get([]byte(nameKey))
		if id == nil {
			return fmt.Errorf("node %q not found", name)
		}
		idStr := string(id)

		nodesBucket := tx.Bucket([]byte(BucketNodes))
		if err := nodesBucket.Delete([]byte(idStr)); err != nil {
			return err
		}
		if err := nodesByName.Delete([]byte(nameKey)); err != nil {
			return err
		}
		nodesByNetwork := tx.Bucket([]byte(BucketNodesByNetwork))
		return nodesByNetwork.Delete([]byte(networkID + ":" + idStr))
	})
}

// ========== Config Operations ==========

// SaveConfigVersion saves a new config version.
func (sm *StorageManager) SaveConfigVersion(networkID, contentHash string, configs map[string]string) (*ConfigVersion, error) {
	var config *ConfigVersion

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		configsBucket := tx.Bucket([]byte(BucketConfigs))
		configsByVer := tx.Bucket([]byte(BucketConfigsByVer))

		// Next version = highest existing version for this network + 1.
		// The version index is keyed networkID:paddedVersion, so the last
		// matching key holds the maximum version — only this network's
		// (tiny) index entries are scanned.
		nextVer := 1
		prefix := []byte(networkID + ":")
		c := configsByVer.Cursor()
		var lastKey []byte
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			lastKey = k
		}
		if lastKey != nil {
			var maxVer int
			if _, err := fmt.Sscanf(string(lastKey[len(prefix):]), "%d", &maxVer); err == nil {
				nextVer = maxVer + 1
			}
		}

		config = &ConfigVersion{
			ID:          uuid.New().String(),
			NetworkID:   networkID,
			Version:     nextVer,
			ContentHash: contentHash,
			Configs:     configs,
			CreatedAt:   time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		if err := configsBucket.Put([]byte(config.ID), data); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		// Save to version index (networkID:paddedVersion -> id)
		if err := configsByVer.Put([]byte(networkID+":"+padVersion(config.Version)), []byte(config.ID)); err != nil {
			return fmt.Errorf("failed to save version index: %w", err)
		}

		return nil
	})

	return config, err
}

// GetLatestConfigVersion retrieves the latest config version for a network
func (sm *StorageManager) GetLatestConfigVersion(networkID string) (*ConfigVersion, error) {
	var latestConfig *ConfigVersion

	err := sm.db.View(func(tx *bbolt.Tx) error {
		configsByVer := tx.Bucket([]byte(BucketConfigsByVer))
		configsBucket := tx.Bucket([]byte(BucketConfigs))

		// The version index is sorted, so the last key for this network's
		// prefix points at the highest version.
		prefix := []byte(networkID + ":")
		c := configsByVer.Cursor()
		var lastID []byte
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			lastID = v
		}
		if lastID == nil {
			return fmt.Errorf("no config version found for network %q", networkID)
		}

		data := configsBucket.Get(lastID)
		if data == nil {
			return fmt.Errorf("no config version found for network %q", networkID)
		}
		latestConfig = &ConfigVersion{}
		return json.Unmarshal(data, latestConfig)
	})

	return latestConfig, err
}

// GetConfigVersion retrieves a specific config version
func (sm *StorageManager) GetConfigVersion(networkID string, version int) (*ConfigVersion, error) {
	var config *ConfigVersion

	err := sm.db.View(func(tx *bbolt.Tx) error {
		configsByVer := tx.Bucket([]byte(BucketConfigsByVer))
		id := configsByVer.Get([]byte(networkID + ":" + padVersion(version)))
		if id == nil {
			return fmt.Errorf("config version %d not found for network %q", version, networkID)
		}

		data := tx.Bucket([]byte(BucketConfigs)).Get(id)
		if data == nil {
			return fmt.Errorf("config version %d not found for network %q", version, networkID)
		}
		config = &ConfigVersion{}
		return json.Unmarshal(data, config)
	})

	return config, err
}

// ListConfigVersions lists all versions for a network, ordered by version.
func (sm *StorageManager) ListConfigVersions(networkID string) ([]*ConfigVersion, error) {
	var versions []*ConfigVersion

	err := sm.db.View(func(tx *bbolt.Tx) error {
		configsByVer := tx.Bucket([]byte(BucketConfigsByVer))
		configsBucket := tx.Bucket([]byte(BucketConfigs))

		// Index keys are version-ordered, so results come out sorted.
		return forEachWithPrefix(configsByVer, []byte(networkID+":"), func(_, v []byte) error {
			data := configsBucket.Get(v)
			if data == nil {
				return nil
			}
			config := &ConfigVersion{}
			if err := json.Unmarshal(data, config); err != nil {
				return err
			}
			versions = append(versions, config)
			return nil
		})
	})

	return versions, err
}

// GetConfigHashByVersion retrieves the hash of a specific version
func (sm *StorageManager) GetConfigHashByVersion(networkID string, version int) (string, error) {
	config, err := sm.GetConfigVersion(networkID, version)
	if err != nil {
		return "", err
	}
	return config.ContentHash, nil
}

// ========== IP Pool Operations ==========

// SaveIPPoolState persists IP pool state to the database
func (sm *StorageManager) SaveIPPoolState(networkID string, state *util.IPPoolState) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketIPPools))
		data, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("failed to marshal IP pool state: %w", err)
		}
		return bucket.Put([]byte(networkID), data)
	})
}

// GetIPPoolState retrieves IP pool state from the database
func (sm *StorageManager) GetIPPoolState(networkID string) (*util.IPPoolState, error) {
	var state *util.IPPoolState
	err := sm.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketIPPools))
		data := bucket.Get([]byte(networkID))
		if data == nil {
			return fmt.Errorf("IP pool state not found for network %s", networkID)
		}
		state = &util.IPPoolState{}
		return json.Unmarshal(data, state)
	})
	return state, err
}
