#!/bin/bash
cd /workspace/study-app
export VAULT_ROOT=/workspace/study-app
trap '' HUP
./study-app &
APP_PID=$!
echo "study-app PID: $APP_PID" >&2
sleep 2
exec /tmp/cloudflared tunnel --url http://127.0.0.1:8081