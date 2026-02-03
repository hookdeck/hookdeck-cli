#!/usr/bin/env node
const { execFileSync } = require('child_process');
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

try {
  execFileSync(binaryPath, process.argv.slice(2), { stdio: 'inherit' });
} catch (error) {
  // execFileSync will exit with the same code as the binary
  // If there's an error executing, exit with code 1
  process.exit(error.status ?? 1);
}
