#!/bin/bash
cd /workspace/study-app
export VAULT_ROOT=/workspace/study-app
trap '' HUP
trap '' TERM
./study-app &
sleep 2
/tmp/cloudflared tunnel --url http://127.0.0.1:8081
while true; do sleep 86400; done