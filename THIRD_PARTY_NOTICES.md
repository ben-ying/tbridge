# Third-party software

TBridge uses these components; their licenses continue to apply:

| Component | License | Source |
|---|---|---|
| Go toolchain / standard library | BSD-3-Clause | https://go.dev/LICENSE |
| tmux (installed separately) | ISC and BSD | https://github.com/tmux/tmux |
| xterm.js 5.5.0 / addon-fit 0.10.0 | MIT | https://github.com/xtermjs/xterm.js |

The build copies the xterm.js and addon-fit license files into the server web assets. JavaScript dependencies are pinned in `web/package.json` and `web/package-lock.json`. No AI agent executable or proprietary agent source is included.
