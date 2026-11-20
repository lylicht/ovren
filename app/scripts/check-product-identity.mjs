/**
 * Verifies that public product names, bundle identifiers, binary names, and
 * required brand assets remain consistent across the frontend, Tauri, and CLI
 * entry points.
 *
 * Run with `pnpm check:identity`. The command throws with the mismatched field
 * or missing asset when a product identity regression is detected.
 */
import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const projectRoot = process.cwd();

function readJson(relativePath) {
  return JSON.parse(readFileSync(join(projectRoot, relativePath), 'utf8'));
}

function readText(relativePath) {
  return readFileSync(join(projectRoot, relativePath), 'utf8');
}

function assertEqual(actual, expected, label) {
  if (actual !== expected) {
    throw new Error(`${label} must be ${JSON.stringify(expected)}, received ${JSON.stringify(actual)}.`);
  }
}

function assertContains(content, expected, label) {
  if (!content.includes(expected)) {
    throw new Error(`${label} must contain ${JSON.stringify(expected)}.`);
  }
}

function assertExists(relativePath) {
  if (!existsSync(join(projectRoot, relativePath))) {
    throw new Error(`Required product asset is missing: ${relativePath}.`);
  }
}

const packageJson = readJson('package.json');
const tauriConfig = readJson('src-tauri/tauri.conf.json');
const linuxTauriConfig = readJson('src-tauri/tauri.linux.conf.json');
const tauriManifest = readText('src-tauri/Cargo.toml');
const cliManifest = readText('src-tauri/crates/mangodisk-cli/Cargo.toml');
const readme = readText('../README.md');
const indexHtml = readText('index.html');
const applicationShell = readText('src/lib/models/shell/application-shell.ts');
const tauriLibrary = readText('src-tauri/src/lib.rs');
const coreLibrary = readText('src-tauri/crates/mangodisk-core/src/lib.rs');
const tauriMain = readText('src-tauri/src/main.rs');
const macosChangeTracking = readText('src-tauri/crates/mangodisk-platform/src/macos/change_tracking.rs');

assertEqual(packageJson.name, 'ovren', 'npm package name');
assertEqual(tauriConfig.productName, 'Ovren', 'Tauri product name');
assertEqual(tauriConfig.identifier, 'dev.licht.ovren', 'Tauri bundle identifier');
assertEqual(tauriConfig.bundle?.publisher, 'Licht', 'Tauri publisher');
assertContains(
  coreLibrary,
  'pub const APPLICATION_IDENTIFIER: &str = "dev.licht.ovren";',
  'Core application identifier'
);
assertEqual(tauriConfig.mainBinaryName, 'Ovren', 'Tauri binary name');
assertEqual(tauriConfig.app?.windows?.[0]?.title, 'Ovren', 'main window title');
assertEqual(linuxTauriConfig.app?.windows?.[0]?.title, 'Ovren', 'Linux main window title');
assertContains(applicationShell, "APP_NAME = 'Ovren'", 'frontend product name');
assertContains(applicationShell, "APP_PUBLISHER = 'Licht'", 'frontend publisher');
assertContains(applicationShell, "APP_ICON_PATH = '/ovren-icon.png'", 'frontend icon path');

assertContains(tauriManifest, 'name = "mangodisk"', 'Tauri package manifest');
assertContains(tauriManifest, 'name = "mangodisk_lib"', 'Tauri library manifest');
assertContains(cliManifest, 'name = "mangodisk-cli"', 'CLI package manifest');
assertContains(cliManifest, 'name = "mangodisk"', 'CLI binary manifest');

assertExists('public/ovren-icon.png');
assertExists('public/ovren-mark.png');
for (const filename of [
  '32x32.png',
  '128x128.png',
  '128x128@2x.png',
  'icon.icns',
  'icon.ico',
  'windows/icon.ico',
  'tray-color.png',
  'tray-template.png',
]) {
  assertExists(`src-tauri/icons/${filename}`);
}
assertExists('src/components/icons/md-icon-ovren.vue');

assertContains(readme, '<h1 align="center">Ovren</h1>', 'README product name');
assertContains(readme, 'app/public/ovren-icon.png', 'README app icon');
assertContains(indexHtml, 'Ovren', 'HTML application shell');
assertContains(tauriLibrary, 'Ovren', 'Tauri library');
assertContains(tauriMain, 'mangodisk_lib', 'Tauri entry point');
assertContains(macosChangeTracking, 'app.mangodisk.cache-dirty-monitor', 'macOS cache monitor identity');

console.log('Product identity is consistent.');
