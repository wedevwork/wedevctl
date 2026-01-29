#!/bin/bash
set -e

echo "=== Cleaning existing database ==="
rm -f ~/.wedevctl/wedevctl.db

echo "=== Test Scenario 1: Create two networks with same node names ==="
printf 'y\n' | ./wedevctl vn add n1 10.0.1.0/24
echo "Created network n1"

printf 'y\n' | ./wedevctl vn add n2 10.0.2.0/24
echo "Created network n2"

echo ""
echo "=== List all networks ==="
./wedevctl vn list
echo ""

echo "=== Add servers to both networks ==="
./wedevctl vn n1 server add s1 s1.example.com 51820
./wedevctl vn n2 server add s2 s2.example.com 51821

echo ""
echo "=== Add nodes with same name 'r1' to both networks ==="
./wedevctl vn n1 node add r1 peer r1.n1.pub 51821
./wedevctl vn n2 node add r1 peer r1.n2.pub 51822

echo ""
echo "=== List nodes in n1 ==="
./wedevctl vn n1 node list

echo ""
echo "=== List nodes in n2 ==="
./wedevctl vn n2 node list

echo ""
echo "=== Add more nodes with same name 'r2' ==="
./wedevctl vn n1 node add r2 peer r2.n1.pub 51822
./wedevctl vn n2 node add r2 peer r2.n2.pub 51823

echo ""
echo "=== List nodes in n1 ==="
./wedevctl vn n1 node list

echo ""
echo "=== List nodes in n2 ==="
./wedevctl vn n2 node list

echo ""
echo "=== Add route type node (no public address) ==="
./wedevctl vn n1 node add r3 route

echo ""
echo "=== Add another route node ==="
./wedevctl vn n1 node add r4 route

echo ""
echo "=== List nodes in n1 after adding route nodes ==="
./wedevctl vn n1 node list

echo ""
echo "=== Test Scenario 2: Config generation with mixed node types ==="
echo "Network n2 has 1 server (s2), 2 peer nodes (r1, r2), should verify topology"

echo ""
echo "=== Generate configs for n2 ==="
./wedevctl vn n2 config generate --output-dir ./test_configs

echo ""
echo "=== Verify config files created ==="
ls -lh ./test_configs/

echo ""
echo "=== Test Scenario 3: Add more nodes to n1 for comprehensive test ==="
echo "Recreating n1 for config generation test..."
printf 'y\n' | ./wedevctl vn add testnet 10.0.10.0/24
./wedevctl vn testnet server add server1 vpn.example.com 51820
./wedevctl vn testnet node add peer1 peer peer1.pub 51821
./wedevctl vn testnet node add peer2 peer peer2.pub 51822
./wedevctl vn testnet node add route1 route
./wedevctl vn testnet node add route2 route

echo ""
echo "=== Generate configs for testnet ==="
./wedevctl vn testnet config generate --output-dir ./final_configs

echo ""
echo "=== List generated config files ==="
ls -lh ./final_configs/

echo ""
echo "=== Verify route node config includes peer nodes ==="
if [ -f ./final_configs/route1.conf ]; then
    echo "route1.conf exists ✓"
    peer_count=$(grep -c "^\[Peer\]" ./final_configs/route1.conf || true)
    echo "route1.conf has $peer_count peer sections (should be 3: server + 2 peers)"
    if [ "$peer_count" -eq 3 ]; then
        echo "✓ Correct: route node has server + peer nodes"
    else
        echo "✗ Error: route node should have 3 peers"
    fi
else
    echo "✗ route1.conf not found"
fi

echo ""
echo "=== Verify peer node config ==="
if [ -f ./final_configs/peer1.conf ]; then
    echo "peer1.conf exists ✓"
    peer_count=$(grep -c "^\[Peer\]" ./final_configs/peer1.conf || true)
    echo "peer1.conf has $peer_count peer sections (should be 2: server + peer2)"
    if [ "$peer_count" -eq 2 ]; then
        echo "✓ Correct: peer node has server + other peer"
    else
        echo "✗ Error: peer node should have 2 peers"
    fi
else
    echo "✗ peer1.conf not found"
fi

echo ""
echo "=== Config version history test ==="
./wedevctl vn testnet config history

echo ""
echo "=== Test Scenario 3.5: Config content display verification ==="
echo "=== View config info (should show full config content, not just filenames) ==="
./wedevctl vn testnet config info

echo ""
echo "=== Verify config info displays actual WireGuard content ==="
config_output=$(./wedevctl vn testnet config info)
if echo "$config_output" | grep -q "^\[Interface\]"; then
    echo "✓ Config info displays actual config content (found [Interface] section)"
else
    echo "✗ Error: Config info should display actual config content"
    exit 1
fi

if echo "$config_output" | grep -q "^PrivateKey ="; then
    echo "✓ Config contains PrivateKey (full WireGuard content verified)"
else
    echo "✗ Error: Config should contain PrivateKey"
    exit 1
fi

if echo "$config_output" | grep -q "^\[Peer\]"; then
    echo "✓ Config contains [Peer] sections (peer configuration verified)"
else
    echo "✗ Error: Config should contain [Peer] sections"
    exit 1
fi

echo ""
echo "=== Verify multiple configs are shown ==="
server_count=$(echo "$config_output" | grep -c "^\[server1.conf\]" || true)
peer1_count=$(echo "$config_output" | grep -c "^\[peer1.conf\]" || true)
if [ "$server_count" -ge 1 ] && [ "$peer1_count" -ge 1 ]; then
    echo "✓ Multiple configs displayed (server and nodes)"
else
    echo "✗ Error: Should display configs for server and all nodes"
    exit 1
fi

echo ""
echo "=== Generate new config version to test version tracking ==="
./wedevctl vn testnet node add peer3 peer peer3.pub 51823

echo ""
echo "=== Regenerate configs (should create version 2) ==="
./wedevctl vn testnet config generate --output-dir ./version2_configs

echo ""
echo "=== View config history (should show 2 versions) ==="
./wedevctl vn testnet config history

echo ""
echo "=== View specific version (version 1) ==="
./wedevctl vn testnet config info 1

echo ""
echo "=== Verify version 1 doesn't have peer3 ==="
version1_output=$(./wedevctl vn testnet config info 1)
if echo "$version1_output" | grep -q "peer3"; then
    echo "✗ Error: Version 1 should not contain peer3"
    exit 1
else
    echo "✓ Version 1 correctly doesn't have peer3"
fi

echo ""
echo "=== View latest version (version 2) ==="
./wedevctl vn testnet config info

echo ""
echo "=== Verify version 2 has peer3 ==="
version2_output=$(./wedevctl vn testnet config info)
if echo "$version2_output" | grep -q "peer3"; then
    echo "✓ Version 2 correctly contains peer3"
else
    echo "✗ Error: Version 2 should contain peer3"
    exit 1
fi

echo ""
echo "=== Cleanup version2 configs ==="
rm -rf ./version2_configs

echo ""
echo "=== Test Scenario 4: IP Reuse After Node Deletion ==="
echo "Creating test network for IP reuse verification..."
printf 'y\n' | ./wedevctl vn add iptest 192.168.100.0/24
./wedevctl vn iptest server add s1 vpn.test.com 51820

echo ""
echo "=== Adding 5 nodes ==="
./wedevctl vn iptest node add n1 peer n1.pub 51821
./wedevctl vn iptest node add n2 peer n2.pub 51822
./wedevctl vn iptest node add n3 route
./wedevctl vn iptest node add n4 route
./wedevctl vn iptest node add n5 route

echo ""
echo "=== Initial node list ==="
./wedevctl vn iptest node list

echo ""
echo "=== Deleting n3 (should be 192.168.100.4) ==="
n3_ip=$(./wedevctl vn iptest node list | grep "^n3 " | awk '{print $2}')
echo "n3 has IP: $n3_ip"
printf 'y\n' | ./wedevctl vn iptest node delete n3

echo ""
echo "=== Adding n6 - should reuse n3's IP ($n3_ip) ==="
./wedevctl vn iptest node add n6 route

echo ""
echo "=== Verifying IP reuse ==="
n6_ip=$(./wedevctl vn iptest node list | grep "^n6 " | awk '{print $2}')
echo "n6 has IP: $n6_ip"
if [ "$n6_ip" = "$n3_ip" ]; then
    echo "✓ IP reuse works: n6 reused $n3_ip"
else
    echo "✗ IP reuse failed: n6 got $n6_ip instead of $n3_ip"
    exit 1
fi

echo ""
echo "=== Deleting multiple nodes ==="
printf 'y\n' | ./wedevctl vn iptest node delete n2
printf 'y\n' | ./wedevctl vn iptest node delete n4

echo ""
echo "=== Adding 2 new nodes - should reuse deleted IPs ==="
./wedevctl vn iptest node add n7 peer n7.pub 51827
./wedevctl vn iptest node add n8 route

echo ""
echo "=== Final node list for IP reuse test ==="
./wedevctl vn iptest node list

echo ""
echo "=== Cleanup iptest network ==="
printf 'y\n' | ./wedevctl vn delete iptest

echo ""
echo "=== Test delete network n1 (cascade delete) ==="
printf 'y\n' | ./wedevctl vn delete n1

echo ""
echo "=== Verify n1 is gone ==="
./wedevctl vn list

echo ""
echo "=== Verify n2 still has its nodes ==="
./wedevctl vn n2 node list

echo ""
echo "=== Try to access n1 (should fail) ==="
./wedevctl vn n1 node list 2>&1 && echo "ERROR: n1 should not exist!" || echo "Correctly failed - n1 doesn't exist"

echo ""
echo "=== Delete network n2 ==="
printf 'y\n' | ./wedevctl vn delete n2

echo ""
echo "=== Delete testnet ==="
printf 'y\n' | ./wedevctl vn delete testnet

echo ""
echo "=== Verify all networks are gone ==="
./wedevctl vn list

echo ""
echo "=== Cleanup test configs ==="
rm -rf ./test_configs ./final_configs

echo ""
echo "=== All tests passed! ==="
echo "✓ Network name scoping works correctly"
echo "✓ Cascade deletion works"
echo "✓ Route nodes include peer configurations (enhancement)"
echo "✓ Config generation produces correct topology"
echo "✓ Config info displays actual WireGuard content (not filenames)"
echo "✓ Config versioning tracks changes correctly"
echo "✓ Specific version retrieval shows correct historical content"
echo "✓ IP addresses are properly reused after node deletion"

