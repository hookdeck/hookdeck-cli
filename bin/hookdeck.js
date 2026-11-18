#!/usr/bin/env node
const { spawn } = require('child_process');
const { existsSync } = require('fs');
const path = require('path');

const platform = process.platform; // 'darwin', 'linux', 'win32'
const arch = process.arch; // 'x64', 'arm64', 'ia32'

// Map Node.js arch to Go arch naming
const archMap = {
  'x64': 'amd64',
  'arm64': 'arm64',
  'ia32': '386'
};
const goArch = archMap[arch] || arch;

// Determine binary directory name and executable name
const binaryDir = `${platform}-${goArch}`;
const binaryName = platform === 'win32' ? 'hookdeck.exe' : 'hookdeck';

// Path to the binary relative to this script
const binaryPath = path.join(__dirname, '..', 'binaries', binaryDir, binaryName);

if (!existsSync(binaryPath)) {
  console.error(`Error: Unsupported platform: ${platform}-${arch}`);
  console.error(`Expected binary at: ${binaryPath}`);
  console.error(`Please report this issue at https://github.com/hookdeck/hookdeck-cli/issues`);
  process.exit(1);
}

// Use spawn (not execFileSync) so the Node event loop stays free to receive signals and
// forward them to the CLI; otherwise `kill <pid>` / `docker stop` would never reach it.
const child = spawn(binaryPath, process.argv.slice(2), { stdio: 'inherit' });

const forwardedSignals = ['SIGINT', 'SIGTERM', 'SIGHUP'];
const handlers = {};
forwardedSignals.forEach((sig) => {
  handlers[sig] = () => {
    try {
      child.kill(sig);
    } catch (e) {
      // child already gone
    }
  };
  process.on(sig, handlers[sig]);
});

child.on('error', (error) => {
  console.error(`Error: failed to run ${binaryPath}: ${error.message}`);
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    // Mirror the child's fate: re-raise the signal with default handling so the parent
    // sees the same termination status.
    forwardedSignals.forEach((sig) => process.removeListener(sig, handlers[sig]));
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 1);
});
