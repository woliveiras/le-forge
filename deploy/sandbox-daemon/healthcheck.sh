#!/bin/sh
# Healthy only when the daemon answers over TLS *and* the sandbox firewall
# rules are in place (see entrypoint.sh).
set -eu
IPT=$(cat /run/leforge-iptables 2>/dev/null) || { echo "firewall not installed yet" >&2; exit 1; }
"$IPT" -C DOCKER-USER -i docker0 ! -o docker0 -d 172.16.0.0/12 -j DROP
"$IPT" -C INPUT -i docker0 -m conntrack ! --ctstate ESTABLISHED,RELATED -j DROP
docker --host tcp://127.0.0.1:2376 --tlsverify \
	--tlscacert /certs/client/ca.pem --tlscert /certs/client/cert.pem --tlskey /certs/client/key.pem \
	version --format '{{.Server.Version}}' >/dev/null
