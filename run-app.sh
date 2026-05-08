#!/bin/bash
cd /workspace/study-app
export VAULT_ROOT=/workspace/study-app
export LLM_API_KEY=$OPENCODE_API_KEY
export LLM_MODEL=qwen3.6-plus
exec ./study-app 2>&1
