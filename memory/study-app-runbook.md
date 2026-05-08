# Study App — Runbook & Observability

## Startup

```bash
cd /workspace/study-app
VAULT_ROOT=/workspace/study-app nohup ./study-app > /tmp/app.log 2>&1 &
disown
```

**CRITICAL:** `VAULT_ROOT=/workspace/study-app` must be set. Without it, the app defaults to `/workspace` and reads from the wrong database (`/workspace/data/study.db` instead of `/workspace/study-app/data/study.db`).

## Tunnel

Binary: `/tmp/cloudflared` (downloaded, not installed system-wide)

**WARNING:** `/tmp/cloudflared` gets deleted between container restarts. Always check if it exists first:
```bash
test -f /tmp/cloudflared || curl -sL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /tmp/cloudflared && chmod +x /tmp/cloudflared
```

### Correct tunnel startup (MUST use this pattern)

```bash
bash -c 'trap "" HUP TERM; /tmp/cloudflared tunnel --url http://127.0.0.1:8081' > /tmp/cf.log 2>&1 &
disown
sleep 10
grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' /tmp/cf.log
```

**CRITICAL:** The `trap "" HUP TERM` and `disown` are required. Without them, the tunnel process gets killed when the bash tool times out or the parent shell exits, causing Error 1033.

### Known tunnel issues

| Error | Cause | Fix |
|-------|-------|-----|
| `1033 Ray ID` | Tunnel process was killed by signal (missing trap/disown) | Kill old tunnel, restart with `trap "" HUP TERM` + `disown` |
| `530 The origin has been unregistered from Argo Tunnel` | Old tunnel process became zombie; Cloudflare dropped the connection | `pkill -9 cloudflared`, start fresh tunnel |
| Tunnel returns 502 | App is not running or not listening on :8081 | Check `curl http://127.0.0.1:8081/`, restart app if needed |
| `npx cloudflared` fails | npx version is experimental, may hang | Use `/tmp/cloudflared` binary directly |
| Tunnel URL returns 000 | Tunnel registered but app is down | Start app first, then restart tunnel |

### Zombie process cleanup

App processes can become `<defunct>` zombies when the parent shell times out:
```bash
ps aux | grep study-app | grep -v grep  # check
# Zombies are harmless but indicate app is dead — restart it
```

## Database

- **Real DB:** `/workspace/study-app/data/study.db`
- **Wrong DB (placeholder):** `/workspace/data/study.db` — created when VAULT_ROOT is unset
- If sessions show "General chat" / "FMEA basics" with no messages, the app is reading the wrong DB

## Health check

```bash
curl -s http://127.0.0.1:8081/api/sessions  # should return sessions with real topics
curl -s http://127.0.0.1:8081/debug/health  # file system health
```
