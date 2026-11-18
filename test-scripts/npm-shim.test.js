// Tests for bin/hookdeck.js (the npm shim). No dependencies: `node --test test-scripts/`
const { test } = require('node:test');
const assert = require('node:assert');
const { spawn } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const archMap = { x64: 'amd64', arm64: 'arm64', ia32: '386' };

// Lay out <tmp>/bin/hookdeck.js next to <tmp>/binaries/<platform>-<arch>/hookdeck,
// where the "binary" is a shell script supplied by the test.
function setup(script) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'hookdeck-shim-'));
  fs.mkdirSync(path.join(dir, 'bin'));
  fs.copyFileSync(path.join(__dirname, '..', 'bin', 'hookdeck.js'), path.join(dir, 'bin', 'hookdeck.js'));
  const binDir = path.join(dir, 'binaries', `${process.platform}-${archMap[process.arch] || process.arch}`);
  fs.mkdirSync(binDir, { recursive: true });
  const bin = path.join(binDir, 'hookdeck');
  fs.writeFileSync(bin, script, { mode: 0o755 });
  return { dir, shim: path.join(dir, 'bin', 'hookdeck.js') };
}

function run(shim, args) {
  const child = spawn(process.execPath, [shim, ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
  let out = '';
  child.stdout.on('data', (d) => (out += d));
  return { child, done: new Promise((resolve) => child.on('close', (code, signal) => resolve({ code, signal, out }))) };
}

test('propagates exit code and arguments', async () => {
  const { dir, shim } = setup('#!/bin/sh\necho "args:$*"\nexit 7\n');
  const { code, out } = await (run(shim, ['listen', '3000']).done);
  fs.rmSync(dir, { recursive: true, force: true });
  assert.strictEqual(code, 7);
  assert.match(out, /args:listen 3000/);
});

for (const sig of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
  test(`forwards ${sig} to the CLI process`, async () => {
    // The fake binary records the signal it receives and exits 0 on it. It also gives up
    // after ~5s so it never outlives the test if the signal is not forwarded.
    const { dir, shim } = setup(
      `#!/bin/sh\ntrap 'echo got-${sig} > "${'$'}(dirname "${'$'}0")/got"; exit 0' ${sig.slice(3)}\n` +
        `echo ready > "${'$'}(dirname "${'$'}0")/ready"\ni=0; while [ $i -lt 50 ]; do sleep 0.1; i=$((i+1)); done\n`
    );
    const got = path.join(dir, 'binaries', `${process.platform}-${archMap[process.arch] || process.arch}`);
    const child = spawn(process.execPath, [shim], { stdio: 'ignore' });
    const exited = new Promise((resolve) => child.on('exit', resolve));
    for (let i = 0; i < 100 && !fs.existsSync(path.join(got, 'ready')); i++) {
      await new Promise((r) => setTimeout(r, 50));
    }
    child.kill(sig);
    await Promise.race([exited, new Promise((r) => setTimeout(r, 3000))]);
    await new Promise((r) => setTimeout(r, 300));
    const received = fs.existsSync(path.join(got, 'got')) ? fs.readFileSync(path.join(got, 'got'), 'utf8').trim() : '';
    // let a non-forwarded fake binary time out on its own before removing its directory
    child.kill('SIGKILL');
    fs.rmSync(dir, { recursive: true, force: true });
    assert.strictEqual(received, `got-${sig}`);
  });
}
