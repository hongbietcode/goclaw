# GoClaw launchd Service (macOS)

Hướng dẫn chạy goclaw như một user-level launchd service trên macOS để auto-start khi login và auto-restart khi crash.

> **Lưu ý:** Thay `<project_dir>` bằng đường dẫn tuyệt đối đến goclaw repo (vd: `/Users/huutri/code/goclaw`). launchd không expand `~` hay env vars trong plist paths.

## Files

| File | Vị trí | Mô tả |
|------|--------|-------|
| `com.goclaw.gateway.plist` | Repo root (git-ignored) | Template plist, chứa secrets → không commit |
| `com.goclaw.gateway.plist` | `~/Library/LaunchAgents/` | Bản đã load vào launchd |
| `stdout.log` / `stderr.log` | `~/Library/Logs/goclaw/` | Logs chạy |
| Binary | `<project_dir>/goclaw` | Build output |

## Plist cấu hình chính

- `RunAtLoad=true` — start ngay khi load / user login
- `KeepAlive.Crashed=true` — tự restart khi crash (không restart nếu exit 0)
- `ThrottleInterval=10` — cách nhau tối thiểu 10s giữa các lần restart
- `EnvironmentVariables` — inline DSN + `GOCLAW_GATEWAY_TOKEN` + `GOCLAW_ENCRYPTION_KEY` (vì launchd không đọc `.env.local`)
- Listen port: `0.0.0.0:18790`

## Workflow setup từ đầu

```bash
# 1. Build
cd <project_dir>
go build -o goclaw .

# 2. Tạo log dir
mkdir -p ~/Library/Logs/goclaw

# 3. Copy plist vào LaunchAgents
cp com.goclaw.gateway.plist ~/Library/LaunchAgents/

# 4. Upgrade DB schema nếu cần
./goclaw upgrade

# 5. Load service
launchctl load -w ~/Library/LaunchAgents/com.goclaw.gateway.plist
```

## Quản lý service

```bash
# Status (cột 1 = PID, cột 2 = last exit code)
launchctl list | grep goclaw

# Stop / Unload
launchctl unload ~/Library/LaunchAgents/com.goclaw.gateway.plist

# Start / Load
launchctl load -w ~/Library/LaunchAgents/com.goclaw.gateway.plist

# Restart nhanh
launchctl kickstart -k gui/$(id -u)/com.goclaw.gateway

# Tail logs
tail -f ~/Library/Logs/goclaw/stdout.log
tail -f ~/Library/Logs/goclaw/stderr.log

# Kiểm tra port
lsof -iTCP -sTCP:LISTEN -n -P | grep 18790
```

## Sync thay đổi plist

Khi sửa `com.goclaw.gateway.plist` trong repo:

```bash
cp com.goclaw.gateway.plist ~/Library/LaunchAgents/
launchctl unload ~/Library/LaunchAgents/com.goclaw.gateway.plist
launchctl load -w ~/Library/LaunchAgents/com.goclaw.gateway.plist
```

## Rebuild binary

Binary không hot-reload → sau khi rebuild phải restart service:

```bash
# Backend + embedded Web UI (recommended)
make build-full
launchctl kickstart -k gui/$(id -u)/com.goclaw.gateway

# Backend only, KHÔNG có UI (GET / → 404)
go build -o goclaw .
```

**Chú ý:** `go build -o goclaw .` (không tag) sẽ **không embed** `ui/web/dist` → truy cập `/` trả 404. Luôn dùng `make build-full` nếu muốn truy cập Web UI tại `http://localhost:18790`.

## Troubleshooting

| Triệu chứng | Nguyên nhân | Fix |
|-------------|-------------|-----|
| `launchctl list` hiện exit code `1` | Binary crash lúc start | `tail -50 ~/Library/Logs/goclaw/stdout.log` |
| `password authentication failed` | DSN sai | Sửa `GOCLAW_POSTGRES_DSN` trong plist + reload |
| `schema is outdated` | DB chưa migrate | Chạy `./goclaw upgrade` rồi reload |
| Service không restart | `KeepAlive.SuccessfulExit=false` và exit 0 | Check log — goclaw có thể đang shutdown graceful |
| Port 18790 đã bind | Instance cũ còn sống | `lsof -iTCP:18790` rồi `kill` hoặc unload |

## Security notes

- Plist chứa `GOCLAW_GATEWAY_TOKEN`, `GOCLAW_ENCRYPTION_KEY`, DB password → **git-ignored**
- File permission: `chmod 600 ~/Library/LaunchAgents/com.goclaw.gateway.plist`
- Đang có WARN `security.cors_open` — set `GOCLAW_ALLOWED_ORIGINS` nếu expose ra ngoài localhost
- Chỉ bind `0.0.0.0:18790` — cân nhắc đổi sang `127.0.0.1:18790` nếu chỉ dùng local

## Unresolved

- Chưa có log rotation (stdout.log sẽ grow vô hạn) — cân nhắc `newsyslog` hoặc `logrotate` wrapper
- Chưa test `GOCLAW_AUTO_UPGRADE=true` trong plist để tự migrate on startup
