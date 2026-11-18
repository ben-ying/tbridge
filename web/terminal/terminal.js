(() => {
  'use strict';
  const term = new Terminal({
    cursorBlink: true, scrollback: 1000, fontSize: 13,
    fontFamily: 'monospace', allowProposedApi: false,
    theme: { background: '#101820', foreground: '#e2e8f0' }
  });
  const fit = new FitAddon.FitAddon();
  const encoder = new TextEncoder();
  const decoder = new TextDecoder();
  let connected = false;
  let ctrl = false;
  term.loadAddon(fit);
  term.open(document.getElementById('terminal'));
  function encode(text) {
    const bytes = encoder.encode(text);
    let raw = '';
    for (const byte of bytes) raw += String.fromCharCode(byte);
    return btoa(raw);
  }
  function decode(data) { return Uint8Array.from(atob(data), c => c.charCodeAt(0)); }
  function send(text) {
    if (!connected) { terminalBridge.notice('Disconnected: input was not sent.'); return; }
    if (ctrl && /^[a-zA-Z@\[\\\]\^_]$/.test(text)) {
      text = String.fromCharCode(text.toUpperCase().charCodeAt(0) & 31);
    }
    ctrl = false;
    terminalBridge.input(encode(text));
  }
  term.onData(send);
  // onBinary uses byte-valued characters and must not pass through UTF-8 encoding.
  term.onBinary(data => { if (connected) terminalBridge.input(btoa(data)); });
  term.onResize(size => terminalBridge.resize(size.cols, size.rows));
  const observer = new ResizeObserver(() => { fit.fit(); });
  observer.observe(document.getElementById('terminal'));
  window.tbridge = {
    theme: mode => {
      const light = mode === 'light';
      document.documentElement.dataset.theme = light ? 'light' : 'dark';
      // Change the existing renderer instead of recreating it and losing output or selection.
      term.options.theme = light
        ? { background: '#ffffff', foreground: '#172330', cursor: '#0f766e', selectionBackground: '#99f6e4', selectionForeground: '#172330', black: '#172330', red: '#b91c1c', green: '#166534', yellow: '#854d0e', blue: '#1d4ed8', magenta: '#7e22ce', cyan: '#0e7490', white: '#64748b', brightBlack: '#475569', brightRed: '#dc2626', brightGreen: '#15803d', brightYellow: '#a16207', brightBlue: '#2563eb', brightMagenta: '#9333ea', brightCyan: '#0891b2', brightWhite: '#334155' }
        : { background: '#101820', foreground: '#e2e8f0', cursor: '#e2e8f0', selectionBackground: '#36536a' };
    },
    selection: () => term.getSelection(),
    text: () => {
      const buffer = term.buffer.active;
      let text = '';
      for (let i = 0; i < buffer.length; i++) {
        const wrapped = buffer.getLine(i + 1)?.isWrapped;
        // Join soft-wrapped display rows so copied commands retain their original logical lines.
        text += (buffer.getLine(i)?.translateToString(!wrapped) || '') + (wrapped ? '' : '\n');
      }
      return text.replace(/\n+$/, '');
    },
    output: data => term.write(decode(data)),
    // Initial keyboard focus must not scroll the parent page down to the terminal cursor.
    focus: () => term.textarea?.focus({ preventScroll: true }),
    connection: value => { connected = value; if (!value) ctrl = false; },
    key: data => {
      let text = decoder.decode(decode(data));
      // Auxiliary arrows must honor the foreground application's cursor-key mode.
      if (term.modes.applicationCursorKeysMode && /^\x1b\[[ABCDHF]$/.test(text)) text = '\x1bO' + text.at(-1);
      send(text); term.focus();
    },
    control: () => { ctrl = !ctrl; terminalBridge.notice(ctrl ? 'Ctrl armed for next key.' : 'Ctrl released.'); term.focus(); },
    paste: data => {
      if (!connected) { terminalBridge.notice('Disconnected: text was not inserted.'); return; }
      const text = decoder.decode(decode(data));
      const overhead = term.modes.bracketedPasteMode ? 12 : 0;
      if (encoder.encode(text).length + overhead > 65536) {
        terminalBridge.notice('Paste exceeds the 64 KiB terminal input limit.'); return;
      }
      if (/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/.test(text)) {
        terminalBridge.notice('Paste rejected: use terminal keys for control characters.'); return;
      }
      if (/[\r\n]/.test(text) && !term.modes.bracketedPasteMode) {
        terminalBridge.notice('Multiline paste blocked: the foreground program has not enabled bracketed paste.'); return;
      }
      // xterm adds bracketed-paste delimiters only when the foreground app enabled it.
      term.paste(text);
      term.focus();
    },
    fit: () => {
      fit.fit();
      // Browser layout/keyboard changes can reveal a viewport without changing its row count.
      term.refresh(0, term.rows - 1);
      terminalBridge.resize(term.cols, term.rows);
    },
    dispose: () => { observer.disconnect(); term.dispose(); }
  };
  fit.fit();
  terminalBridge.ready();
})();
