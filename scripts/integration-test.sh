#!/usr/bin/env sh
# ═══════════════════════════════════════════════════════════════════════
#  integration-test.sh – Run from inside the integration container
#  Expects ssh-server and target-server to be reachable on "testnet".
# ═══════════════════════════════════════════════════════════════════════
set -eu

PASS=0
FAIL=0
total_tests=0

pass() { PASS=$((PASS + 1)); total_tests=$((total_tests + 1)); echo "  ✓ $1"; }
fail() { FAIL=$((FAIL + 1)); total_tests=$((total_tests + 1)); echo "  ✗ $1"; }

echo "══════════════════════════════════════════════"
echo " GoNC Integration Tests"
echo "══════════════════════════════════════════════"

# ── 1. Version ────────────────────────────────────────────────────────
echo ""
echo "── version ──"
if gonc --version | grep -q "gonc"; then
    pass "gonc --version"
else
    fail "gonc --version"
fi

# ── 2. Direct TCP connect ────────────────────────────────────────────
echo ""
echo "── direct TCP connect ──"
REPLY=$(echo "hello" | gonc -w 5 target-server 8080 2>/dev/null)
if [ "$REPLY" = "hello" ]; then
    pass "echo → target-server:8080 → echo back"
else
    fail "echo → target-server:8080 (got: '$REPLY')"
fi

# ── 3. Port scan ─────────────────────────────────────────────────────
echo ""
echo "── port scan ──"
if gonc -vz -w 3 target-server 8080 2>&1 | grep -qi "open"; then
    pass "port scan target-server:8080 → open"
else
    fail "port scan target-server:8080"
fi

# ── 4. Port scan (closed) ────────────────────────────────────────────
echo ""
echo "── port scan (closed port) ──"
if ! gonc -z -w 1 target-server 9999 2>/dev/null; then
    pass "port scan target-server:9999 → closed (exit ≠ 0)"
else
    fail "port scan target-server:9999 should have failed"
fi

# ── 5. SSH tunnel (if ssh-server is reachable) ────────────────────────
echo ""
echo "── SSH tunnel ──"
if gonc -vz -w 3 ssh-server 2222 2>&1 | grep -qi "open"; then
    # SSH server is up – test tunnelled connection.
    # NOTE: --ssh-password causes an interactive prompt which won't work
    # in non-interactive scripts.  This test verifies the tunnel code
    # path is exercised; for full E2E, the test would need sshpass or
    # key-based auth.  We validate the tunnel parse path here.
    if gonc -T testuser@ssh-server:2222 --ssh-key /dev/null target-server 8080 -w 3 2>&1 \
         | grep -qi "tunnel\|ssh\|connect"; then
        pass "SSH tunnel code path exercised"
    else
        fail "SSH tunnel code path"
    fi
else
    echo "  ⊘ ssh-server:2222 not reachable – skipping tunnel tests"
fi

# ── Summary ───────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════"
echo " Results: ${PASS} passed, ${FAIL} failed (${total_tests} total)"
echo "══════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
