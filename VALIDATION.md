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

1. Run the Go suite on a real Linux/macOS host and confirm tmux integration passes without a skip. macOS binaries were cross-compiled, not run here.
2. Replace and restart the server binary, retaining its state directory. Refresh the web page. Pair desktop and mobile browsers using separate codes; compare the certificate fingerprint locally before trusting it.
3. Create two sessions, send `pwd` using Insert followed by Enter, and check Chinese input, shortcuts, Backspace, Clear (Ctrl+L), Clear input and target isolation. Confirm Clear input only clears the draft.
4. Open the same session from the computer using the displayed tmux attach command. Exercise installed Cursor/Codex CLIs, multiline prompts, TUI redraws and confirmation dialogs.
5. Check mobile portrait/landscape layouts and soft-keyboard behavior, favicon and touch icon. Disconnect/reconnect, refresh the page and restart the service; confirm tasks persist and input is never replayed.
6. Revoke a browser and confirm its connection closes. Verify expired/reused codes fail and End terminates only the selected session. Check Tailscale access on mobile data.

This execution environment prohibits network sockets; HTTPS/WSS end-to-end tests, real tmux integration and real browser rendering remain unverified. Remote commands run with the server user's permissions. Review that behavior before production deployment. Roll back by stopping the service and replacing its binary, preserving the state directory and existing tmux tasks.
