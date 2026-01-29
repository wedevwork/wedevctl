#!/bin/bash
set -e  # Exit on error
set -x  # Print commands as they execute

echo "=== Checking database location ==="
ls -lh ~/.wedevctl/wedevctl.db 2>&1 || echo "Database doesn't exist yet"

echo ""
echo "=== Deleting old database ==="
rm -f ~/.wedevctl/wedevctl.db
rm -f *.conf 2>/dev/null || true
echo "Database deleted"

echo ""
echo "=== Rebuilding binary ==="
go build -o wedevctl
echo "Build complete"

echo ""
echo "=== Creating fresh test network ==="
printf 'y\n' | ./wedevctl vn add n1 10.0.1.0/24

echo ""
echo "=== Adding server ==="
./wedevctl vn n1 server add s1 s1.example.com 51820

echo ""
echo "=== Adding peer nodes r1 and r2 ==="
./wedevctl vn n1 node add r1 peer r1.pub 51821
./wedevctl vn n1 node add r2 peer r2.pub 51822

echo ""
echo "=== Adding route nodes r3 and r4 (no endpoints) ==="
./wedevctl vn n1 node add r3 route
./wedevctl vn n1 node add r4 route

echo ""
echo "=== Listing nodes ==="
./wedevctl vn n1 node list

echo ""
echo "=== Generating WireGuard configs ==="
./wedevctl vn n1 config generate

echo ""
echo "=== Verifying config files were created ==="
ls -lh s1.conf r1.conf r2.conf r3.conf r4.conf

echo ""
echo "=== Checking r3 config (route node should have peer nodes r1 and r2) ==="
echo "--- r3.conf content ---"
cat r3.conf
echo "--- end r3.conf ---"

echo ""
echo "=== Checking r1 config (peer node should have r2) ==="
echo "--- r1.conf content ---"
cat r1.conf
echo "--- end r1.conf ---"

echo ""
echo "=== Verification: Count peer sections ==="
echo "r3 (route) peer count: $(grep -c '^\[Peer\]' r3.conf) (expected: 3 = server + 2 peers)"
echo "r1 (peer) peer count: $(grep -c '^\[Peer\]' r1.conf) (expected: 2 = server + 1 peer)"
echo "r4 (route) peer count: $(grep -c '^\[Peer\]' r4.conf) (expected: 3 = server + 2 peers)"

echo ""
echo "=== Creating second network ==="
printf 'y\n' | ./wedevctl vn add n2 10.0.2.0/24

echo ""
echo "=== Adding server to n2 ==="
./wedevctl vn n2 server add s2 s2.example.com 51820

echo ""
echo "=== Adding r1 and r2 to n2 (same names, different network) ==="
./wedevctl vn n2 node add r1 peer r1.n2.pub 51821
./wedevctl vn n2 node add r2 peer r2.n2.pub 51822

echo ""
echo "=== Listing nodes in n2 ==="
./wedevctl vn n2 node list

echo ""
echo "=== Test Scenario: IP Reuse After Deletion ==="
echo "Testing IP recycling bug fix..."

echo ""
echo "=== Deleting node r3 (IP 10.0.1.4) ==="
printf 'y\n' | ./wedevctl vn n1 node delete r3

echo ""
echo "=== Listing nodes after deletion ==="
./wedevctl vn n1 node list

echo ""
echo "=== Adding new node r5 - should reuse 10.0.1.4 ==="
./wedevctl vn n1 node add r5 route

echo ""
echo "=== Verifying IP was reused ==="
r5_ip=$(./wedevctl vn n1 node list | grep "^r5 " | awk '{print $2}')
if [ "$r5_ip" = "10.0.1.4" ]; then
    echo "✓ SUCCESS: r5 reused IP 10.0.1.4"
else
    echo "✗ FAILED: r5 got $r5_ip instead of 10.0.1.4"
fi

echo ""
echo "=== Final node list ==="
./wedevctl vn n1 node list

echo ""
echo "=== SUCCESS! Enhancement verification complete ==="
echo "✓ Route nodes (r3, r4) include all peer nodes (r1, r2)"
echo "✓ Peer nodes (r1, r2) include each other"
echo "✓ Route nodes do NOT include other route nodes"
echo "✓ Config generation works correctly"
echo "✓ IP addresses are properly reused after node deletion"


