#!/usr/bin/env bash
set -euo pipefail
/srv/yongbo-oa/current/oa-store &
# Both listeners exist only in this unit's private network namespace.
redis-server --bind 127.0.0.1 --port 6379 --save "" --appendonly no --daemonize no --maxmemory 32mb --dir /var/lib/yongbo-oa >/dev/null &
for ((i=0; i<30; i++)); do
  if redis-cli -h 127.0.0.1 -p 6379 ping >/dev/null 2>&1; then break; fi
  sleep .1
done
exec /srv/yongbo-oa/current/oa-core
