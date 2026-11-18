const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const { test } = require('node:test');

function terminal() {
  let term;
  const sent = [], notices = [], output = [], pasted = [], refreshed = [], sizes = [];
  class Terminal {
    constructor() { term = this; this.modes = { bracketedPasteMode: false }; this.cols = 80; this.rows = 24; this.textarea = { focus: options => { this.focused = true; this.focusOptions = options; } }; }
    loadAddon() {} open() {} focus() { this.focused = true; } dispose() {}
    onData(callback) { this.data = callback; }
    onBinary(callback) { this.binary = callback; }
    onResize(callback) { this.resize = callback; }
    write(bytes) { output.push(Buffer.from(bytes)); }
    refresh(start, end) { refreshed.push([start, end]); }
    paste(text) {
      pasted.push(text);
      this.data(this.modes.bracketedPasteMode ? '\x1b[200~' + text + '\x1b[201~' : text);
    }
  }
  const context = { Terminal, FitAddon: { FitAddon: class { fit() {} } },
    TextEncoder, TextDecoder, Uint8Array,
    btoa: value => Buffer.from(value, 'binary').toString('base64'),
    atob: value => Buffer.from(value, 'base64').toString('binary'),
    ResizeObserver: class { observe() {} disconnect() {} },
    document: { getElementById: () => ({}) }, window: {},
    terminalBridge: { input: data => sent.push(Buffer.from(data, 'base64')), notice: text => notices.push(text), resize: (cols, rows) => sizes.push([cols, rows]), ready() {} }
  };
  vm.runInNewContext(fs.readFileSync('web/terminal/terminal.js', 'utf8'), context);
  return { bridge: context.window.tbridge, term, sent, notices, output, pasted, refreshed, sizes };
}
const encode = text => Buffer.from(text).toString('base64');

test('disconnected input is never replayed', () => {
  const t = terminal();
  t.bridge.paste(encode('codex'));
  t.bridge.key(encode('\r'));
  assert.equal(t.sent.length, 0);
  t.bridge.connection(true);
  t.bridge.focus();
  assert.equal(t.term.focused, true);
  assert.equal(t.term.focusOptions.preventScroll, true);
  assert.equal(t.sent.length, 0);
});
test('Chinese prompts and Enter are separate input operations', () => {
  const t = terminal(); t.bridge.connection(true);
  t.bridge.paste(encode('解释这个项目'));
  assert.equal(t.sent[0].toString(), '解释这个项目');
  t.bridge.key(encode('\r'));
  assert.equal(t.sent[1].toString(), '\r');
});
test('multiline requires bracketed paste and retains delimiters', () => {
  const t = terminal(); t.bridge.connection(true);
  t.bridge.paste(encode('第一行\n第二行'));
  assert.equal(t.sent.length, 0);
  assert.match(t.notices.at(-1), /Multiline paste blocked/);
  t.term.modes.bracketedPasteMode = true;
  t.bridge.paste(encode('第一行\n第二行'));
  assert.equal(t.sent[0].toString(), '\x1b[200~第一行\n第二行\x1b[201~');
});
test('paste rejects terminal control injection but helper keys work', () => {
  const t = terminal(); t.bridge.connection(true);
  t.bridge.paste(encode('text\x1b[201~\rwhoami'));
  assert.equal(t.sent.length, 0);
  t.bridge.key(encode('\x03'));
  assert.equal(t.sent[0][0], 3);
  t.bridge.control(); t.term.data('d');
  assert.equal(t.sent[1][0], 4);
});
test('UTF-8 and binary terminal bytes are preserved', () => {
  const t = terminal(); t.bridge.connection(true);
  t.bridge.output(encode('\x1b[31m中文'));
  assert.equal(t.output[0].toString(), '\x1b[31m中文');
  t.term.binary('\xff');
  assert.deepEqual(t.sent[0], Buffer.from([255]));
});
test('auxiliary arrows honor application cursor mode', () => {
  const t = terminal(); t.bridge.connection(true);
  t.bridge.key(encode('\x1b[A'));
  assert.equal(t.sent[0].toString(), '\x1b[A');
  t.term.modes.applicationCursorKeysMode = true;
  t.bridge.key(encode('\x1b[A'));
  assert.equal(t.sent[1].toString(), '\x1bOA');
  t.bridge.key(encode('\x1b[H'));
  t.bridge.key(encode('\x1b[F'));
  assert.equal(t.sent[2].toString(), '\x1bOH');
  assert.equal(t.sent[3].toString(), '\x1bOF');
});

test('revealing the same-size viewport redraws buffered output without replaying input', () => {
  const t = terminal();
  t.bridge.connection(true);
  t.bridge.output(encode('restored prompt $ '));
  t.bridge.fit();
  t.bridge.fit();
  assert.deepEqual(t.refreshed, [[0, 23], [0, 23]]);
  assert.deepEqual(t.sizes, [[80, 24], [80, 24]]);
  assert.equal(t.output[0].toString(), 'restored prompt $ ');
  assert.equal(t.sent.length, 0);
});
