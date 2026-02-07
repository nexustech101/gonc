#!/bin/sh
# Enable TCP forwarding for integration tests
# The linuxserver image uses /config/sshd/sshd_config (not /etc/ssh/)
for f in /etc/ssh/sshd_config /config/sshd/sshd_config; do
  [ -f "$f" ] && {
    sed -i 's/^AllowTcpForwarding no/AllowTcpForwarding yes/' "$f"
    sed -i 's/^GatewayPorts no/GatewayPorts yes/' "$f"
    echo "[custom-init] patched $f"
  }
done
echo "[custom-init] TCP forwarding enabled"
