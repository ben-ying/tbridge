'use strict';
(() => {
  const $ = id => document.getElementById(id);
  const encoder = new TextEncoder();
  const frame = $('terminal-frame');
  let sessions = [], selected = null, socket = null, connected = false;
  let generation = 0, busy = false, pending = new Set(), editorAction = null;
  let authenticated = false;
  let failureShown = false;
  let preferences = { computers: [], shortcuts: [] };
  let storageWarning = '';
  try {
    const saved = JSON.parse(localStorage.getItem('tbridge-preferences') || '{}');
    if (Array.isArray(saved.computers)) preferences.computers = saved.computers.filter(c => typeof c.name === 'string' && typeof c.address === 'string');
    if (Array.isArray(saved.shortcuts)) preferences.shortcuts = saved.shortcuts.filter(s => typeof s.id === 'string' && typeof s.name === 'string' && typeof s.text === 'string');
  } catch (error) { storageWarning = 'Browser preferences could not be loaded; changes may not persist.'; }
  const notify = text => { $('status').textContent = String(text); };
  function savePreferences() {
    try { localStorage.setItem('tbridge-preferences', JSON.stringify(preferences)); }
    catch (error) { notify('Browser storage unavailable or full. Preferences have not been saved.'); }
  }
  function encode(text) {
    let raw = '';
    for (const byte of encoder.encode(text)) raw += String.fromCharCode(byte);
    return btoa(raw);
  }
  function address(value) {
    const u = new URL(value);
    if (u.protocol !== 'https:' || u.username || u.password || u.pathname !== '/' || u.search || u.hash) throw Error('Use an HTTPS address without a path.');
    return u.origin;
  }
  function element(tag, text, className) {
    const node = document.createElement(tag); if (text !== undefined) node.textContent = text;
    if (className) node.className = className; return node;
  }
  function button(text, action, className) {
    const node = element('button', text, className); node.type = 'button';
    node.addEventListener('click', () => { void run(action); }); return node;
  }
  function terminal() {
    try { return frame.contentWindow?.tbridge; } catch (error) { return null; }
  }
  function state() {
    $('pair-panel').hidden = authenticated;
    $('session-panel').hidden = !authenticated;
    $('logout').hidden = !authenticated;
    $('empty-panel').hidden = !authenticated || selected !== null;
    $('terminal-panel').hidden = !authenticated || selected === null;
    $('connection-state').textContent = connected ? '● Connected' : '○ Disconnected';
    for (const node of document.querySelectorAll('#terminal-keys button, #insert, #enter')) node.disabled = !connected;
  }
  async function run(action) {
    if (busy) return;
    busy = true; document.body.setAttribute('aria-busy', 'true');
    try { await action(); }
    catch (error) { notify(error.message || 'Operation failed.'); }
    finally { busy = false; document.body.removeAttribute('aria-busy'); }
  }
  async function api(method, path, body) {
    const response = await fetch('/api/v1/' + path, {
      method, credentials: 'same-origin', cache: 'no-store', redirect: 'error',
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(15000)
    });
    const result = await response.json();
    if (!response.ok) {
      if (response.status === 401 && path !== 'browser/pair') { authenticated = false; stopTerminal(); selected = null; state(); }
      throw Error(result.error || `HTTP ${response.status}`);
    }
    return result;
  }
  function edit(title, name, value, valueTitle, action) {
    $('editor-title').textContent = title;
    $('editor-name').value = name;
    $('editor-value').value = value || '';
    $('editor-value-label').hidden = valueTitle === null;
    $('editor-value-title').textContent = valueTitle || '';
    editorAction = action;
    $('editor').showModal();
  }
  $('editor-cancel').addEventListener('click', () => $('editor').close());
  $('editor-form').addEventListener('submit', event => {
    event.preventDefault();
    void run(async () => {
      const name = $('editor-name').value.trim();
      if (!name) throw Error('Enter a name.');
      await editorAction(name, $('editor-value').value);
      $('editor').close();
    });
  });
  function renderComputers() {
    const nodes = [];
    const current = element('li'); current.append(element('span', 'This computer', 'selected'), element('small', location.origin)); nodes.push(current);
    for (const computer of preferences.computers) {
      let url; try { url = address(computer.address); } catch (error) { continue; }
      const li = element('li'), link = element('a', computer.name);
      link.href = url; link.target = '_blank'; link.rel = 'noopener noreferrer';
      li.append(link, element('small', url));
      const actions = element('div', undefined, 'toolbar');
      actions.append(button('Edit', () => edit('Edit computer', computer.name, url, 'HTTPS address', (name, value) => {
        computer.name = name; computer.address = address(value); savePreferences(); renderComputers();
      })), button('Remove', () => {
        if (!confirm(`Remove ${computer.name} from this browser's list?`)) return;
        preferences.computers = preferences.computers.filter(item => item !== computer); savePreferences(); renderComputers();
      })); li.append(actions); nodes.push(li);
    }
    $('computers').replaceChildren(...nodes);
  }
  $('add-computer').addEventListener('click', () => edit('Add computer', '', 'https://', 'HTTPS address', (name, value) => {
    const url = address(value);
    const existing = preferences.computers.find(item => item.address === url);
    if (existing) existing.name = name; else preferences.computers.push({ name, address: url });
    savePreferences(); renderComputers();
  }));

  function renderSessions() {
    const nodes = sessions.map(session => {
      const li = element('li');
      li.append(element('span', session.name, selected?.id === session.id ? 'selected' : ''));
      const actions = element('div', undefined, 'toolbar');
      actions.append(button('Open', () => openSession(session)), button('Rename', () => edit('Rename session', session.name, '', null, async name => {
        await api('PATCH', `sessions/${encodeURIComponent(session.id)}`, { name });
        if (selected?.id === session.id) { selected.name = name; $('target-name').textContent = name; }
        await loadSessions();
      })), button('End', async () => {
        if (!confirm(`End ${session.name}? This terminates the shell and its running tasks.`)) return;
        await api('DELETE', `sessions/${encodeURIComponent(session.id)}`);
        if (selected?.id === session.id) { stopTerminal(); selected = null; state(); }
        await loadSessions();
      }, 'danger'));
      li.append(actions); return li;
    });
    $('sessions').replaceChildren(...nodes);
  }
  async function loadSessions() {
    sessions = await api('GET', 'sessions'); authenticated = true;
    renderSessions(); state();
  }
  $('refresh').addEventListener('click', () => { void run(loadSessions); });
  $('create-session').addEventListener('click', () => edit('New session', '', '', null, async name => {
    const session = await api('POST', 'sessions', { name });
    await loadSessions(); openSession(sessions.find(item => item.id === session.id) || session);
  }));
  $('pair-form').addEventListener('submit', event => {
    event.preventDefault();
    void run(async () => {
      const input = $('pair-string').value.trim();
      if (!input.startsWith('tbridge:') || input.length > 4096) throw Error('Paste a valid TBridge connection string.');
      const raw = input.slice(8).replace(/-/g, '+').replace(/_/g, '/');
      const pairing = JSON.parse(new TextDecoder().decode(Uint8Array.from(atob(raw), c => c.charCodeAt(0))));
      if (address(pairing.address) !== location.origin) throw Error(`This pairing belongs to ${address(pairing.address)}. Open that computer's page and pair there.`);
      if (!/^[a-f0-9]{64}$/.test(pairing.fingerprint) || !/^[a-f0-9]{48}$/.test(pairing.code)) throw Error('Invalid pairing data.');
      // Browser TLS validation is handled by the browser, not by JavaScript certificate pinning.
      await api('POST', 'browser/pair', { code: pairing.code, name: $('device-name').value.trim() });
      $('pair-string').value = '';
      await loadSessions(); notify('Browser paired. Open or create a session.');
    });
  });
  $('logout').addEventListener('click', () => { void run(async () => {
    if (!confirm('Revoke this browser connection? Terminal tasks will keep running.')) return;
    await api('POST', 'browser/logout', {}); stopTerminal(); authenticated = false; selected = null; sessions = [];
    state(); notify('Browser unpaired. Terminal tasks are still running.');
  }); });

  function stopTerminal() {
    generation++; connected = false;
    const hadPending = pending.size > 0;
    if (socket) { socket.onclose = socket.onerror = socket.onmessage = null; socket.close(); socket = null; }
    terminal()?.connection(false); pending.clear();
    if (hadPending) notify('Some input has unknown delivery status. It will not be resent.');
    state(); return hadPending;
  }
  function openSession(session) {
    const unknown = stopTerminal(); selected = session;
    failureShown = false;
    $('target-name').textContent = session.name;
    $('attach-command').textContent = session.attach || 'Refresh the session list to see its attach command.';
    state(); renderSessions();
    notify(`Connecting to ${location.host} / ${session.name}…${unknown ? ' Previous input has unknown delivery status; it will not be resent.' : ''}`);
    frame.src = '/terminal-frame.html?connection=' + generation;
  }
  function lost(reason) {
    if (failureShown) return;
    failureShown = true;
    connected = false; terminal()?.connection(false);
    const unknown = pending.size > 0; pending.clear(); state();
    notify(reason + (unknown ? ' Some input has unknown delivery status; it will not be resent.' : ''));
  }
  function send(message) {
    if (!connected || socket?.readyState !== WebSocket.OPEN) { notify('Disconnected: input was not sent.'); return; }
    if (socket.bufferedAmount > 1024 * 1024 || pending.size >= 1024) { socket.close(); lost('Input queue full. Reconnect.'); return; }
    socket.send(JSON.stringify(message));
  }
  window.terminalHost = {
    input(data, epoch) {
      if (String(generation) !== epoch) return;
      if (!connected) { notify('Disconnected: input was not sent.'); return; }
      if (data.length > 87384) { notify('Input limit: 64 KiB.'); return; }
      if (pending.size >= 1024 || socket?.bufferedAmount > 1024 * 1024) { socket?.close(); lost('Input queue full. Reconnect.'); return; }
      const id = crypto.randomUUID(); pending.add(id);
      send({ type: 'input', id, data });
    },
    resize(cols, rows, epoch) {
      if (String(generation) !== epoch) return;
      if (connected && cols >= 2 && cols <= 500 && rows >= 2 && rows <= 300) send({ type: 'resize', cols, rows });
    },
    notice(text, epoch) { if (String(generation) === epoch) notify(text); },
    ready(epoch) {
      if (String(generation) !== epoch || !selected || !authenticated || socket !== null) return;
      const current = generation;
      const url = new URL(`/api/v1/sessions/${encodeURIComponent(selected.id)}/terminal`, location.origin); url.protocol = 'wss:';
      socket = new WebSocket(url);
      socket.onmessage = event => {
        if (current !== generation || failureShown) return;
        try {
          if (typeof event.data !== 'string' || event.data.length > 128 * 1024) throw Error('Invalid terminal message.');
          const message = JSON.parse(event.data);
          if (message.type === 'output') terminal()?.output(message.data);
          else if (message.type === 'ready') {
            connected = true; terminal()?.connection(true); terminal()?.fit(); state();
            notify(`Connected: ${location.host} / ${selected.name}`);
          } else if (message.type === 'ack') pending.delete(message.id);
          else if (message.type === 'error') { socket.close(); lost(message.message || 'Terminal error.'); }
        } catch (error) { socket.close(); lost('Invalid terminal message. Reconnect.'); }
      };
      socket.onclose = () => { if (current === generation) lost('Terminal disconnected. Check the session and reconnect.'); };
      socket.onerror = () => { if (current === generation) lost('Terminal connection failed. Check HTTPS trust, pairing and network.'); };
    }
  };
  $('reconnect').addEventListener('click', () => { if (selected) openSession(selected); });
  // Clear is a foreground-program key, never an injected shell command; the draft has its own Clear input action.
  const keys = { Escape: '\x1b', Tab: '\t', Backspace: '\x7f', Delete: '\x1b[3~', Home: '\x1b[H', End: '\x1b[F', PageUp: '\x1b[5~', PageDown: '\x1b[6~', ArrowUp: '\x1b[A', ArrowDown: '\x1b[B', ArrowLeft: '\x1b[D', ArrowRight: '\x1b[C', Clear: '\x0c', Interrupt: '\x03', EOF: '\x04' };
  for (const node of document.querySelectorAll('[data-key]')) node.addEventListener('click', () => terminal()?.key(encode(keys[node.dataset.key])));
  $('control').addEventListener('click', () => terminal()?.control());
  $('insert').addEventListener('click', () => {
    const text = $('prompt').value;
    if (encoder.encode(text).length > 65536) { notify('Prompt limit: 64 KiB.'); return; }
    terminal()?.paste(encode(text));
  });
  $('enter').addEventListener('click', () => terminal()?.key(encode('\r')));
  $('clear-prompt').addEventListener('click', () => { $('prompt').value = ''; });
  function renderShortcuts() {
    $('shortcuts').replaceChildren(...preferences.shortcuts.map(value => {
      const li = element('li');
      li.append(button(value.name, () => { $('prompt').value = value.text; }), button('Edit', () => edit('Edit shortcut', value.name, value.text, 'Text', (name, text) => {
        if (encoder.encode(text).length > 65536) throw Error('Shortcut limit: 64 KiB.');
        value.name = name; value.text = text; savePreferences(); renderShortcuts();
      })), button('Remove', () => {
        if (!confirm(`Remove shortcut ${value.name}?`)) return;
        preferences.shortcuts = preferences.shortcuts.filter(item => item !== value); savePreferences(); renderShortcuts();
      })); return li;
    }));
  }
  $('save-shortcut').addEventListener('click', () => edit('Save shortcut', '', $('prompt').value, 'Text', (name, text) => {
    if (encoder.encode(text).length > 65536) throw Error('Shortcut limit: 64 KiB.');
    preferences.shortcuts.push({ id: crypto.randomUUID(), name, text }); savePreferences(); renderShortcuts();
  }));
  window.addEventListener('pagehide', stopTerminal);
  $('host-address').textContent = location.origin;
  renderComputers(); renderShortcuts(); state();
  void run(async () => {
    try { await loadSessions(); notify(storageWarning || 'Choose or create a terminal session.'); }
    catch (error) { notify(error.message.startsWith('device authentication') ? 'Pair this browser to get started.' : error.message); }
  });
})();
