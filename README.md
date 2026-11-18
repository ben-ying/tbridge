# TBridge

[English](README.md) · [简体中文](README.zh-CN.md) · [繁體中文](README.zh-TW.md)

Control persistent Linux/macOS terminal sessions from a desktop or mobile web browser. Select a computer and session, send shell commands, and interact with Cursor CLI, Codex CLI and other terminal AI agents.

## Features and requirements

- Web: responsive browser client served by the same server, with a bundled xterm.js terminal.
- Server: Go standard library, tmux and PTY; Linux/macOS on amd64 or arm64.
- Create, rename and end sessions; live output, Chinese input, multiline prompts, shortcuts and terminal helper keys.
- Clients share persistent tmux sessions. Disconnecting a client leaves tasks running; reconnecting restores the screen and up to 1,000 history lines. Restarting the computer ends sessions.

TBridge manages its own tmux sessions. It cannot select an arbitrary existing desktop Terminal window or type into the Cursor editor's chat box. Cursor and Codex integration means running their CLI in a managed session.

Running a server binary requires **tmux 3.2+** on that computer. Install it with `sudo apt install tmux` on Ubuntu or `brew install tmux` on macOS. Install and authenticate AI CLIs separately under the server's user account.

Building from source requires Go 1.22+, Node.js 20+ and npm. The server runs as your normal user; terminal commands have that user's permissions.

## Build and install

Run these commands from the project root. Dependencies are downloaded normally by npm:

~~~sh
cd web
npm ci --ignore-scripts --no-audit --no-fund
cd ..
bash scripts/build-server.sh
~~~

Choose the server binary for the **server computer**, not the phone:

| Server OS / architecture | Binary |
| --- | --- |
| Linux x86_64 | `dist/tbridge-linux-amd64` |
| Linux aarch64 / arm64 | `dist/tbridge-linux-arm64` |
| macOS Intel | `dist/tbridge-darwin-amd64` |
| macOS Apple Silicon | `dist/tbridge-darwin-arm64` |

Use `uname -m` to check the computer architecture. Set `GO_BIN` if Go is not on PATH. Server builds embed the web client; after changing web assets, rebuild the server. Before building Go directly, run `bash scripts/prepare-web.sh`.

## Start the server

The following installation example uses Linux x86_64. Replace the source binary path using the table above for other platforms:

~~~sh
mkdir -p "$HOME/.local/bin"
cp dist/tbridge-linux-amd64 "$HOME/.local/bin/tbridge"
chmod +x "$HOME/.local/bin/tbridge"
"$HOME/.local/bin/tbridge" serve --listen 0.0.0.0:8443
~~~

Keep this terminal open while using a client. **`serve` must remain running**; `--listen` is its listening-address option, not a separate command. Use a background service below if you want to close the terminal. Keep the computer awake and connected to the network.

Without `--listen`, the default is `127.0.0.1:8443`, which only the server computer can access. `0.0.0.0:8443` listens on all IPv4 interfaces, including LAN and Tailscale; restrict access with your firewall/network policy. Never enter `0.0.0.0` as the client address.

## Connect over LAN or Tailscale

### Same LAN

Use the server computer's LAN IP, for example `https://192.168.1.100:8443`. Allow TCP 8443 from the intended LAN. The phone and computer must have a route to each other.

### External phone using Tailscale

No public IP or router port forwarding is needed when both devices can communicate through Tailscale.

1. Install [Tailscale](https://tailscale.com/download) on the server computer and client device.
2. Sign in and connect both devices to the same tailnet. Check any device approval or access rules managed by your organization.
3. On the **server computer**, get its Tailscale IPv4 address:

~~~sh
tailscale ip -4
~~~

4. Use that actual address in pairing and in the browser. The examples below use **`100.99.206.33` as an example server address**; replace it if your command prints another address. Do not use the phone's IP.
5. Keep Tailscale connected on the phone when switching to mobile data or another Wi-Fi network. Keep Tailscale and TBridge running on the computer.
6. Confirm the firewall and tailnet rules allow the phone to reach the computer on TCP 8443. If Ubuntu UFW is enabled and your policy allows it, an example rule is:

~~~sh
sudo ufw allow in on tailscale0 to any port 8443 proto tcp
~~~

To listen only on Tailscale, stop the foreground server with Ctrl+C, then run this alternative instead of the all-interface command:

~~~sh
"$HOME/.local/bin/tbridge" serve --listen 100.99.206.33:8443
~~~

Tailscale must already be connected before binding its IP. TBridge itself does not provide a public relay or NAT traversal; Tailscale provides connectivity between the devices.

## Pair clients and reconnect later

With the server still running, open **another terminal on the server computer**, using the same OS user, and run:

~~~sh
"$HOME/.local/bin/tbridge" pair --address https://100.99.206.33:8443
~~~

For LAN use, replace the address with your LAN HTTPS address. Always include `https://` and the correct port. The output contains a certificate SHA-256 fingerprint and a complete connection string beginning with `tbridge:`. Copy the entire string, without the surrounding explanatory lines.

- A pairing code expires after **five minutes** and can successfully pair **one client only**.
- Run `pair` again for each browser profile. A new code replaces the previous unused code, so finish one pairing before generating the next.
- Generating a new code does **not** revoke clients already paired.
- After pairing, clients save their own credentials. Normal reconnects, code expiration and server restarts do not require pairing again.
- Pair again after clearing/expiring browser cookies, revoking the device, or replacing server certificates. The browser cookie currently lasts 30 days.
- `pair` exits after printing its output; only `serve` needs to keep running. Treat the connection string as sensitive while valid.

### Web client

1. On the client device, open `https://100.99.206.33:8443/` in a browser. External devices also need Tailscale connectivity.
2. The server generates a self-signed certificate. Compare the browser certificate's SHA-256 fingerprint with the local `pair` output before accepting a certificate exception or trusting it. TLS trust is handled by the browser; JavaScript cannot pin the certificate.
3. Generate a **new unused** pairing string for this browser. Enter a **Device name**, paste the string into **Connection string**, then click **Pair browser**.
4. Click **New session**, or open an existing session. You can share sessions across browsers.
5. Enter `pwd` in the editor, click **Insert into terminal**, then **Enter**. You can also type directly into the terminal area.
6. Use the session actions, **Reconnect**, helper keys and **Save shortcut** as needed. **Unpair browser** revokes this browser's access.

**Enter sends only the terminal return key; it does not insert the editor draft.** Use **Insert into terminal**, then **Enter**. In the editor, Enter inserts a newline. Wait for **Connected** and a visible terminal prompt before sending commands.

Use **Refresh** to reload sessions, **Rename** to change a session name and **End** to terminate the session and its tasks. Closing the browser tab leaves tasks running. Phones use the same web page; no client application installation is required.

**Computers → Add** saves links to other servers and opens each in its own tab. Each HTTPS origin/browser profile pairs separately; this is not a cross-server proxy. Web shortcuts and computer links stay in the current browser. Authentication uses a Secure, HttpOnly, SameSite=Strict cookie; tokens are not stored in JavaScript storage or URL parameters.

## Commands, shortcuts and AI agents

Input goes to the selected session's **current foreground program**, whether a shell, agent or confirmation prompt. Verify the computer and session name before sending input.

To use an agent, first change to your project directory in the terminal, execute that command, then start the installed CLI:

~~~sh
cd /absolute/path/to/project
codex
~~~

Send each command with Insert followed by Enter. For Cursor, use the command provided by your installed Cursor CLI version. Install/login to the agent on the server computer first. TBridge does not install agents, provide API keys or change agent authorization settings.

Once the agent is running, use the same editor for prompts and respond to its confirmation dialogs. The web client exposes Esc, Tab, Backspace, Delete, Home, End, PgUp/PgDn, arrows, Ctrl, Ctrl+C, Ctrl+D and **Clear (Ctrl+L)**. Backspace sends DEL; Clear sends Ctrl+L to the foreground program to clear/redraw its screen (behavior depends on that program). **Clear input** only empties the draft and sends nothing. Ctrl arms the next terminal character as a control key. Ctrl+C usually interrupts the foreground task; Ctrl+D sends EOF and may exit a shell.

- Multiline prompts require the foreground program to enable **bracketed paste**. If blocked, use single-line input or enable support in that program; pressing Enter in the editor only inserts a newline.
- The editor rejects terminal control characters. Use helper keys or the physical keyboard for those.
- Web **Save shortcut** saves the draft; selecting one fills the editor. Insert and Enter remain separate.
- Output is a continuous terminal stream, not a per-command result/exit-code API. A server acknowledgement means the PTY accepted input, not that the command succeeded.
- Multiple clients on the same session share the shell, current directory and input stream. Coordinate simultaneous typing.
- Avoid storing secrets in browser shortcuts.

To join the same session on the computer, copy the displayed `tmux ... attach-session ...` command. On the web, expand **Open this session on the computer**. Detach with **Ctrl+B, then D**; press Ctrl+B twice to send literal Ctrl+B to the foreground program. Do not rename internal tmux session IDs or add windows/panes to managed sessions; use TBridge's session actions.

## Devices and state directory

List authorized devices and revoke a selected device using its printed ID:

~~~sh
"$HOME/.local/bin/tbridge" devices
"$HOME/.local/bin/tbridge" revoke --id DEVICE_ID
~~~

Revocation closes active connections within approximately ten seconds. Revoke the browser device on the server to remove its access.

Default state directories are `~/.config/tbridge` on Linux and `~/Library/Application Support/tbridge` on macOS. State includes the certificate, device records, pending pairing code and tmux socket. Preserve it across server upgrades and run all commands as the same OS user. Do not delete it while tasks are running.

For a custom directory, pass the **same absolute path to every command**, for example:

~~~sh
"$HOME/.local/bin/tbridge" serve --state /absolute/path/tbridge-state --listen 0.0.0.0:8443
~~~

In another terminal:

~~~sh
"$HOME/.local/bin/tbridge" pair --state /absolute/path/tbridge-state --address https://100.99.206.33:8443
"$HOME/.local/bin/tbridge" devices --state /absolute/path/tbridge-state
~~~

## Run the server in the background

Stop the foreground server before starting a service on the same port. The templates below listen on all IPv4 interfaces; review their listen address and PATH before enabling them. Include tmux and your AI CLI executables in the service PATH. User services may not load your interactive shell environment.

### Linux: systemd user service

After installing `~/.local/bin/tbridge`, copy the template from the project root:

~~~sh
mkdir -p "$HOME/.config/systemd/user"
cp deploy/tbridge.service "$HOME/.config/systemd/user/tbridge.service"
~~~

Edit that copy if you want to restrict `--listen`, extend PATH or add a custom `--state`. Then enable and inspect it:

~~~sh
systemctl --user daemon-reload
systemctl --user enable --now tbridge.service
systemctl --user status tbridge.service
journalctl --user -u tbridge.service -n 100 --no-pager
~~~

Manage the service:

~~~sh
systemctl --user restart tbridge.service
systemctl --user stop tbridge.service
systemctl --user disable tbridge.service
~~~

An enabled user service starts with your user manager, normally at login. If it must run before login and after logout, an administrator can enable lingering:

~~~sh
sudo loginctl enable-linger "$USER"
~~~

### macOS: LaunchAgent

Copy the template:

~~~sh
mkdir -p "$HOME/Library/LaunchAgents"
cp deploy/dev.tbridge.agent.plist "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
~~~

Edit the copied file: replace **every `/Users/YOU`** with your home path, check PATH (`/opt/homebrew/bin` for Apple Silicon Homebrew), and adjust the listen address/state path if needed. Then validate and load it:

~~~sh
plutil -lint "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
launchctl print "gui/$(id -u)/dev.tbridge.agent"
~~~

The LaunchAgent runs at user login and KeepAlive restarts it on exit. To stop it before editing/upgrading:

~~~sh
launchctl bootout "gui/$(id -u)" "$HOME/Library/LaunchAgents/dev.tbridge.agent.plist"
~~~

Load it again with `bootstrap` after the change. For rollback, stop the service and replace the binary. Existing tmux tasks normally survive a TBridge service restart; a computer restart ends them. Preserve the state directory to retain client credentials.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| `Client sent an HTTP request to an HTTPS server.` | Use `https://`, not `http://`, in browser and pairing addresses. |
| Phone cannot connect | Confirm serve is running, the computer is awake, the IP belongs to the server, and it is not listening only on 127.0.0.1. Check firewall, port and LAN/Tailscale reachability. |
| Works on Wi-Fi but not mobile data | Connect phone Tailscale; check that both devices are in the intended tailnet and access rules permit TCP 8443. |
| Pairing fails | Generate a new code, use it within five minutes, and do not reuse a code already consumed by another client. Use the same server user/state directory. |
| Certificate error | Verify the fingerprint locally. After replacing a certificate, verify its new fingerprint and pair the browser again. Do not disable TLS verification globally. |
| Connected, but Insert shows nothing | Use a single-line `pwd`. Check the status text after Insert: queued means waiting for acknowledgement; server accepted means PTY delivery, not command completion. Renderer/input errors indicate a client-side failure. Also try direct terminal typing or another new session and record the status/error. |
| Enter has no effect on editor text | First press Insert (web: Insert into terminal), then Enter. Enter alone does not send the editor draft. |
| Multiline paste blocked | The foreground program must enable bracketed paste. |
| Web terminal missing after a source update | Run `bash scripts/prepare-web.sh` and rebuild/restart the server; web assets are embedded at build time. |
| Agent command missing or not logged in | Check the service user's PATH, agent installation/login and project permissions. |
| Address already in use | Stop the existing foreground server or service before starting another on the same port. |
| Disconnected / uncertain input delivery | Inspect the terminal before retrying. Inputs are never automatically replayed, to avoid duplicate commands. |

## Verification

~~~sh
cd server
go test -race -v ./...
go vet ./...
cd ..
node tests/terminal.test.cjs
node tests/browser.test.cjs
~~~

Tests cover pairing, revocation, browser cookies/origins, WebSocket framing, PTY input/resize and terminal paste/keys. The tmux integration test explicitly skips when Unix sockets are unavailable; run it on the real server. See [VALIDATION.md](VALIDATION.md) for executed checks and remaining host/device verification.

macOS runtime behavior, mobile browser input and actual Cursor/Codex interaction still require device acceptance checks. Review remote command execution code before production use. Dependency licenses are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
