# GoNC – Usage Examples

## Basic TCP Connection

```bash
# Connect and type interactively
gonc example.com 80

# Send an HTTP request
echo -e "GET / HTTP/1.0\r\nHost: example.com\r\n\r\n" | gonc example.com 80

# Pipe a file
gonc remote-host 9000 < report.pdf

# Receive to file
gonc remote-host 9000 > received.bin
```

## Listening / Server

```bash
# Simple listener
gonc -l -p 8080

# Listen and save incoming data
gonc -l -p 9000 > incoming.tar.gz

# Serve a file
gonc -l -p 9000 < big-file.iso

# Keep-open: accept multiple clients
gonc -lk -p 8080

# Bind a shell (Unix)
gonc -l -p 4444 -e /bin/bash

# Bind a command (Windows)
gonc -l -p 4444 -c "cmd.exe"
```

## Port Scanning

```bash
# Scan a single port
gonc -vz target.host 22

# Scan a range
gonc -vz target.host 20-25

# Scan specific ports
gonc -vz target.host 22 80 443 3306 5432 8080

# Scan with short timeout
gonc -vz -w 1 target.host 1-1024

# Numeric-only (skip DNS)
gonc -vnz 192.168.1.1 22 80 443
```

## SSH Tunnel

```bash
# Connect to an internal database through a bastion
gonc -T admin@bastion.example.com db-internal 5432

# Use a specific SSH key
gonc -T deploy@jump-host --ssh-key ~/.ssh/deploy_ed25519 api-server 8080

# Password authentication
gonc -T root@gateway --ssh-password internal-host 22

# Use SSH agent
gonc -T user@bastion --ssh-agent target 80

# Strict host-key checking
gonc -T user@bastion --strict-hostkey --known-hosts ~/.ssh/known_hosts target 443

# Scan through tunnel
gonc -vz -T user@bastion 10.0.0.5 22 80 443

# File transfer through tunnel
tar czf - /data | gonc -T user@bastion backup-server 9000

# Receive through tunnel
gonc -T user@bastion file-server 9000 > backup.tar.gz
```

## UDP

```bash
# UDP client
gonc -u target.host 5353

# UDP listener
gonc -lu -p 5353

# Send a DNS query (raw)
echo -ne '\x00\x01...' | gonc -u 8.8.8.8 53
```

## Chat Between Two Machines

```bash
# Machine A (listener)
gonc -l -p 12345

# Machine B (connector)
gonc machine-a.local 12345

# Both sides can now type interactively.
```

## Proxy / Relay (with shell piping)

```bash
# Simple TCP relay on Unix using named pipes
mkfifo /tmp/relay
gonc -l -p 8080 < /tmp/relay | gonc remote.host 80 > /tmp/relay
```

## Timeouts

```bash
# 5-second connection timeout
gonc -w 5 slow-host.example.com 80

# 2-second scan timeout per port
gonc -vz -w 2 target 1-100
```

## Combining Flags

```bash
# Verbose + scan + timeout + numeric
gonc -vnz -w 2 192.168.1.1 20-25

# Listen + keep-open + verbose
gonc -lvk -p 8080
```
