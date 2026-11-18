# Validation record

Date: 2026-11-17. Current scope: web client and Linux/macOS server. Execution environment: Linux amd64, Go 1.22.1 and Node.js 24.18.0. Builds disable VCS stamping because the workspace has no usable Git metadata.

## Executed checks

| Check | Command | Result |
|---|---|---|
| Server tests and race detector | In `server`: `go test -race -count=1 -v ./...` | Passed; tmux integration skipped because this environment prohibits Unix sockets |
| Go static analysis | In `server`: `go vet ./...` | Passed |
| Terminal renderer | `node tests/terminal.test.cjs` | 7 tests passed |
| Browser controller | `node tests/browser.test.cjs` | 6 tests passed |
| Server cross-compilation | `bash scripts/build-server.sh` with `GO_BIN` set to the installed Go toolchain | Passed; Linux/macOS amd64/arm64 binaries generated in `dist/` |
| Shell syntax | `bash -n scripts/*.sh` | Passed |

The renderer now lives in `web/terminal/` and is bundled into the server by `scripts/prepare-web.sh`. Client application sources, application build configuration and application-specific tests have been removed. The browser frame uses `terminalBridge` to communicate with its terminal renderer. The browser authentication protocol and server state format remain unchanged.

Terminal tests cover disconnected input without replay, Chinese text, separate Insert/Enter, bracketed paste, control-character rejection, byte preservation, application cursor keys and viewport redraw. Browser tests cover target isolation, stale frames, uncertain input without replay, shortcuts, origin-specific pairing, cookie credentials and editing keys including Backspace and Clear. These use simulated DOM/transport objects, so passing tests do not establish visual rendering or network interoperability.

The Go suite covers pairing expiry/reuse, revocation, certificate persistence and permissions, browser cookies/logout, origin checks, static assets, WebSocket framing and real PTY input/resize.

## Remaining host verification

### Terminal-first layout update (2026-11-18)

The mobile workspace now precedes navigation, and opening a session scrolls its terminal into view. The terminal receives focus when its connection becomes ready; subsequent output does not move focus or scroll the page. The prompt editor, its actions and shortcuts are grouped in a native details panel that starts collapsed. Terminal helper keys remain outside it. Desktop navigation stays on the left; mobile users can follow Sessions & computers to reach the lists.

Executed after this update: `node tests/terminal.test.cjs` (7 passed), `node tests/browser.test.cjs` (6 passed), `node --check server/ui/app.js`, `bash -n scripts/*.sh` and `scripts/build-server.sh` using the installed Go toolchain (server tests passed and all four binaries rebuilt). The existing controller test now checks initial scroll/focus and confirms subsequent output does not steal focus or scroll; terminal tests check focus without replaying input. Headless Chrome could not start because its crash handler socket operation is prohibited. Real desktop/mobile layout and keyboard behavior still require browser verification.

### Compact header and collapsible terminal keys (2026-11-18)

Removed the brand subtitle and reduced the header padding and icon size. Session names and connection states share one line; long names truncate instead of displacing the status. Esc, Tab and the remaining terminal helper keys now live in a separate details panel, collapsed by default. The prompt editor remains independently collapsible.

Validation: all 13 JavaScript tests passed, including helper-key input and draft clearing. HTML inspection confirmed both panels start collapsed and all 15 data-key buttons are inside the key panel. `git diff --check` passed. The build script passed server tests and rebuilt all four server binaries. Real browser layout remains subject to the environment limits described above.

### Page-top navigation and network liveness (2026-11-18)

Opening a session now explicitly returns to the page top. Initial terminal focus uses the xterm textarea's `focus({preventScroll: true})` instead of scrolling to its cursor. The collapsible panels now use English-only labels: Terminal keys and Edit prompt.

Browser WebSocket control pings are invisible to JavaScript, so the browser now sends an application ping with a random ID, and the server replies with the same ID in a pong. A five-second watchdog waits up to 15 seconds for a matching reply, also timing out connections that never become ready. Silent loss is normally detected within 15–20 seconds on a foreground page; background timer throttling can delay it. Timeout, client offline events and synchronous send failures mark the terminal disconnected, close the socket and clear the watchdog. Reconnect, session switching and page exit clear the old timer. Pending terminal input is never replayed.

Executed checks: 7 terminal tests and 9 browser tests passed. New coverage includes silent loss with an OPEN socket, mismatched pongs, unrelated output, healthy idle connections, connection readiness timeout, offline/page-exit timer cleanup, send failures, disabled input after loss and reconnect without replay. Server race tests and Go vet passed; tmux integration was skipped because Unix sockets are prohibited. All four server binaries were rebuilt. Real phone network interruption and visual scroll behavior remain host acceptance checks.

The requested pre-fix Git commit could not be created: `.git` is read-only and `git add` cannot create `index.lock`. The prior UI diff was saved to `/tmp/tbridge-ui-before-connectivity-fix.patch`; no commit was created.

### Themes, device presence and terminal copying (2026-11-18)

The header theme toggle changes the page and existing xterm renderer together, follows system appearance initially and stores a manual choice in browser preferences. Terminal text and the connection are preserved. The authenticated Devices panel polls every ten seconds and shows only public paired-device metadata. Online means authenticated activity in the last 30 seconds; timestamps are held in server memory and reset on restart. Existing pairing records require no migration. WebSocket application heartbeats also update presence. Multiple tabs sharing a credential count as one device; timer throttling can make a background browser appear offline.

Copy text exports the xterm selection, or the active terminal buffer when no selection exists, into a readonly native textarea. Soft-wrapped rows are joined into logical lines. Clipboard writes require a user click; if access is denied or unavailable, the text remains selectable for manual copying. Copied text is not persisted in browser preferences, and the dialog clears it on close. Ctrl+C keeps its terminal interrupt behavior.

Validation: 12 browser tests and 8 renderer tests passed. Coverage includes saved/system theme choice, forwarding theme to a new terminal without reconnecting, online/offline/current device labels and stale-list errors, selection/scrollback copying, denied clipboard access, wrapped Chinese text and copying without terminal input. Go race tests and Go vet passed. New server tests verify authentication, origin rejection, presence expiry, revocation, concurrent updates and absence of credentials/hashes in device responses. The build script passed server tests and generated all four server binaries. Real desktop/mobile colors, selection gestures, OS clipboard permissions and cross-device presence updates still need host/browser acceptance; automated browser tests use a simulated DOM and transport.

### Host checks

1. Run the Go suite on a real Linux/macOS host and confirm tmux integration passes without a skip. macOS binaries were cross-compiled, not run here.
2. Replace and restart the server binary, retaining its state directory. Refresh the web page. Pair desktop and mobile browsers using separate codes; compare the certificate fingerprint locally before trusting it.
3. Create two sessions, send `pwd` using Insert followed by Enter, and check Chinese input, shortcuts, Backspace, Clear (Ctrl+L), Clear input and target isolation. Confirm Clear input only clears the draft.
4. Open the same session from the computer using the displayed tmux attach command. Exercise installed Cursor/Codex CLIs, multiline prompts, TUI redraws and confirmation dialogs.
5. Check mobile portrait/landscape layouts and soft-keyboard behavior, favicon and touch icon. Disconnect/reconnect, refresh the page and restart the service; confirm tasks persist and input is never replayed.
6. Revoke a browser and confirm its connection closes. Verify expired/reused codes fail and End terminates only the selected session. Check Tailscale access on mobile data.

This execution environment prohibits network sockets; HTTPS/WSS end-to-end tests, real tmux integration and real browser rendering remain unverified. Remote commands run with the server user's permissions. Review that behavior before production deployment. Roll back by stopping the service and replacing its binary, preserving the state directory and existing tmux tasks.
