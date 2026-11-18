'use strict';
// The terminal frame and its owner are served from the same HTTPS origin.
const epoch = new URLSearchParams(location.search).get('connection');
window.terminalBridge = {
  input: data => window.parent.terminalHost.input(data, epoch),
  resize: (cols, rows) => window.parent.terminalHost.resize(cols, rows, epoch),
  notice: text => window.parent.terminalHost.notice(text, epoch),
  ready: () => window.parent.terminalHost.ready(epoch)
};
