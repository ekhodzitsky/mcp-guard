# Sandboxing MCP Servers

mcp-guard runs external processes. For production deployments, sandbox these processes to limit their access to the host system.

## firejail (Linux)

Create a profile at `/etc/firejail/mcp-server.profile`:

```
include /etc/firejail/default.profile
net none
noroot
seccomp
```

Use in `mcp-guard.toml`:

```toml
[server.filesystem]
command = "firejail"
args = ["--profile=/etc/firejail/mcp-server.profile", "npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
```

## bubblewrap (Linux)

```bash
bwrap \
  --ro-bind /usr/bin/npx /usr/bin/npx \
  --ro-bind /usr/bin/node /usr/bin/node \
  --tmpfs /tmp \
  --unshare-all \
  --die-with-parent \
  -- npx -y @modelcontextprotocol/server-filesystem /tmp
```

In `mcp-guard.toml`:

```toml
[server.filesystem]
command = "bwrap"
args = [
  "--ro-bind", "/usr/bin/npx", "/usr/bin/npx",
  "--ro-bind", "/usr/bin/node", "/usr/bin/node",
  "--tmpfs", "/tmp",
  "--unshare-all",
  "--die-with-parent",
  "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp"
]
```

## macOS

Use `sandbox-exec` with a custom profile. Example:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>seatbelt-profiles</key>
    <array>
        <dict>
            <key>debug-mode</key>
            <false/>
            <key>allow</key>
            <array>
                <string>file-read*</string>
                <string>file-write*</string>
            </array>
        </dict>
    </array>
</dict>
</plist>
```

Run with:
```bash
sandbox-exec -f profile.sb npx -y @modelcontextprotocol/server-filesystem /tmp
```

## Recommendations

- **Never** run MCP servers as root.
- Use read-only filesystem bindings where possible.
- Disable network access if the server does not need it (`net none` in firejail, `--unshare-net` in bwrap).
- Use `--die-with-parent` in bubblewrap so the sandbox is cleaned up when mcp-guard exits.
