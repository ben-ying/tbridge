const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const { test } = require('node:test');
const { webcrypto } = require('node:crypto');

// Exercise the actual browser controller without a network or browser dependency.
// Rendering and real TLS/WebSocket interoperability still require host acceptance.
class Element {
  constructor(tag = 'div') { this.tag = tag; this.listeners = {}; this.children = []; this.value = ''; this.dataset = {}; this.hidden = false; }
  addEventListener(type, callback) { (this.listeners[type] ||= []).push(callback); }
  emit(type) { for (const callback of this.listeners[type] || []) callback({ preventDefault() {} }); }
  click() { this.emit('click'); }
  append(...nodes) { this.children.push(...nodes); }
  replaceChildren(...nodes) { this.children = nodes; }
  setAttribute() {} removeAttribute() {}
  showModal() { this.open = true; } close() { this.open = false; }
}
const settle = async () => { for (let i = 0; i < 8; i++) await new Promise(resolve => setImmediate(resolve)); };
function find(node, text) {
  if (node.tag === 'button' && node.textContent === text) return node;
  for (const child of node.children) { const result = find(child, text); if (result) return result; }
}
function browser(authenticated = true) {
  const nodes = {};
  const html = fs.readFileSync('server/ui/index.html', 'utf8');
  for (const match of html.matchAll(/\bid="([^"]+)"/g)) nodes[match[1]] = new Element();
  const keyButtons = [...html.matchAll(/data-key="([^"]+)"/g)].map(match => {
    const node = new Element('button'); node.dataset.key = match[1]; return node;
  });
  const requests = [], sockets = [], terminalCalls = [], storage = new Map();
  const sessions = [{ id: 'tb-' + 'a'.repeat(24), name: 'Codex', attach: 'tmux attach one' }, { id: 'tb-' + 'b'.repeat(24), name: 'Cursor', attach: 'tmux attach two' }];
  let paired = authenticated;
  class WebSocket {
    static OPEN = 1;
    constructor(url) { this.url = String(url); this.readyState = 1; this.bufferedAmount = 0; this.sent = []; sockets.push(this); }
    send(text) { this.sent.push(JSON.parse(text)); }
    close() { this.readyState = 3; queueMicrotask(() => this.onclose?.()); }
    message(value) { this.onmessage?.({ data: JSON.stringify(value) }); }
  }
  const terminal = {};
  for (const method of ['connection', 'fit', 'output', 'paste', 'key', 'control']) terminal[method] = (...args) => terminalCalls.push([method, ...args]);
  nodes['terminal-frame'].contentWindow = { tbridge: terminal };
  let frameURL = '';
  Object.defineProperty(nodes['terminal-frame'], 'src', {
    get: () => frameURL,
    set: value => { frameURL = value; const epoch = new URL(value, 'https://server.test:8443').searchParams.get('connection'); queueMicrotask(() => context.window.terminalHost.ready(epoch)); }
  });
  const context = {
    document: { getElementById: id => nodes[id], createElement: tag => new Element(tag), body: new Element(),
      querySelectorAll: selector => selector === '[data-key]' ? keyButtons : [...keyButtons, nodes.insert, nodes.enter, nodes.control] },
    window: { addEventListener() {} }, location: new URL('https://server.test:8443'),
    localStorage: { getItem: key => storage.get(key) || null, setItem: (key, value) => storage.set(key, value) },
    TextEncoder, TextDecoder, Uint8Array, URL, AbortSignal, crypto: webcrypto, WebSocket,
    confirm: () => true,
    btoa: text => Buffer.from(text, 'binary').toString('base64'), atob: text => Buffer.from(text, 'base64').toString('binary'),
    fetch: async (url, options) => {
      requests.push({ url, ...options });
      if (url.endsWith('/browser/pair')) { paired = true; return { ok: true, json: async () => ({ deviceId: 'browser' }) }; }
      if (!paired) return { ok: false, status: 401, json: async () => ({ error: 'device authentication failed' }) };
      return { ok: true, json: async () => sessions.map(s => ({ ...s })) };
    }
  };
  vm.runInNewContext(fs.readFileSync('server/ui/app.js', 'utf8'), context);
  return { nodes, requests, sockets, terminalCalls, storage, sessions, context, keyButtons,
    epoch: () => new URL(nodes['terminal-frame'].src, 'https://server.test:8443').searchParams.get('connection') };
}

test('web session selection directs input only to the chosen target', async () => {
  const b = browser(); await settle();
  find(b.nodes.sessions.children[0], 'Open').click(); await settle();
  assert.match(b.sockets[0].url, new RegExp(b.sessions[0].id));
  b.sockets[0].message({ type: 'ready' });
  const oldEpoch = b.epoch();
  const oldMessage = b.sockets[0].onmessage;
  b.context.window.terminalHost.input('YQ==', oldEpoch);
  assert.equal(b.sockets[0].sent.at(-1).type, 'input');
  find(b.nodes.sessions.children[1], 'Open').click(); await settle();
  assert.match(b.sockets[1].url, new RegExp(b.sessions[1].id));
  b.sockets[1].message({ type: 'ready' });
  const before = b.sockets[1].sent.length;
  b.context.window.terminalHost.input('b2xk', oldEpoch);
  oldMessage({ data: JSON.stringify({ type: 'output', data: 'b2xk' }) });
  assert.equal(b.sockets[1].sent.length, before);
  assert.ok(!b.terminalCalls.some(call => call[0] === 'output' && call[1] === 'b2xk'));
  b.context.window.terminalHost.input('5Lit5paH', b.epoch());
  assert.equal(b.sockets[1].sent.at(-1).data, '5Lit5paH');
});
test('web disconnect reports uncertain input and never replays it', async () => {
  const b = browser(); await settle(); find(b.nodes.sessions.children[0], 'Open').click(); await settle();
  b.sockets[0].message({ type: 'ready' });
  b.context.window.terminalHost.input('YQ==', b.epoch());
  b.sockets[0].onerror(); b.sockets[0].onclose();
  assert.match(b.nodes.status.textContent, /unknown delivery status/);
  b.nodes.reconnect.click(); await settle(); b.sockets[1].message({ type: 'ready' });
  assert.ok(!b.sockets[1].sent.some(message => message.type === 'input'));
});
test('web shortcuts fill the editor without submitting terminal input', async () => {
  const b = browser(); await settle();
  b.nodes.prompt.value = '多行\n提示词'; b.nodes['save-shortcut'].click();
  b.nodes['editor-name'].value = 'Explain'; b.nodes['editor-form'].emit('submit'); await settle();
  const saved = JSON.parse(b.storage.get('tbridge-preferences'));
  assert.equal(saved.shortcuts[0].text, '多行\n提示词');
  b.nodes.prompt.value = ''; find(b.nodes.shortcuts, 'Explain').click(); await settle();
  assert.equal(b.nodes.prompt.value, '多行\n提示词'); assert.equal(b.sockets.length, 0);
});
test('browser pairing rejects a connection string for another server', async () => {
  const b = browser(false); await settle();
  b.nodes['pair-string'].value = 'tbridge:' + Buffer.from(JSON.stringify({ address: 'https://other.test:8443', fingerprint: 'a'.repeat(64), code: 'b'.repeat(48) })).toString('base64url');
  b.nodes['device-name'].value = 'Web'; b.nodes['pair-form'].emit('submit'); await settle();
  assert.match(b.nodes.status.textContent, /Open that computer's page/);
  assert.ok(!b.requests.some(request => request.url.endsWith('/browser/pair')));
});
test('browser pairing uses same-origin cookies without bearer tokens in JavaScript', async () => {
  const b = browser(false); await settle();
  b.nodes['pair-string'].value = 'tbridge:' + Buffer.from(JSON.stringify({ address: 'https://server.test:8443', fingerprint: 'a'.repeat(64), code: 'b'.repeat(48) })).toString('base64url');
  b.nodes['device-name'].value = 'Web'; b.nodes['pair-form'].emit('submit'); await settle();
  const request = b.requests.find(request => request.url.endsWith('/browser/pair'));
  assert.equal(request.credentials, 'same-origin'); assert.equal(request.headers.Authorization, undefined);
  assert.equal(b.nodes['pair-string'].value, ''); assert.equal(b.nodes['pair-panel'].hidden, true);
  assert.ok(![...b.storage.values()].some(value => value.includes('token')));
});

test('web exposes editing keys and draft clearing stays local', async () => {
  const b = browser(); await settle();
  find(b.nodes.sessions.children[0], 'Open').click(); await settle();
  b.sockets[0].message({ type: 'ready' });
  const keys = { Backspace: '\x7f', Delete: '\x1b[3~', Home: '\x1b[H', End: '\x1b[F', PageUp: '\x1b[5~', PageDown: '\x1b[6~', Clear: '\x0c' };
  for (const [name, sequence] of Object.entries(keys)) {
    const button = b.keyButtons.find(node => node.dataset.key === name);
    assert.ok(button, `${name} is visible in the web toolbar`);
    button.click();
    const call = b.terminalCalls.at(-1);
    assert.equal(call[0], 'key');
    assert.equal(Buffer.from(call[1], 'base64').toString(), sequence);
  }
  b.nodes.prompt.value = 'do not execute';
  const before = b.terminalCalls.length;
  b.nodes['clear-prompt'].click();
  assert.equal(b.nodes.prompt.value, '');
  assert.equal(b.terminalCalls.length, before);
});
