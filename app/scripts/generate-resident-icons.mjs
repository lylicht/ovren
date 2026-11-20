/**
 * Regenerate the native Ovren desktop and system-tray assets from the approved
 * orbital artwork. Called before packaging so no legacy icon returns on build.
 */
import { mkdtempSync, readFileSync, writeFileSync, mkdirSync, copyFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const root = fileURLToPath(new URL('../', import.meta.url));
const appSource = join(root, 'public/ovren-icon.png');
const markSource = join(root, 'public/ovren-mark.png');
const markTemplateSource = join(root, 'public/ovren-mark-mono.png');
const output = join(root, 'src-tauri/icons');
const cli = join(root, 'node_modules/@tauri-apps/cli/tauri.js');
const pngSignature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);
for (const source of [appSource, markSource, markTemplateSource]) {
  if (!readFileSync(source).subarray(0, 8).equals(pngSignature)) {
    throw new Error(`Invalid Ovren icon source: ${source}`);
  }
}
function generate(input, directory, sizes = []) {
  execFileSync(
    process.execPath,
    [cli, 'icon', input, '--output', directory, ...sizes.flatMap(size => ['--png', String(size)])],
    { cwd: root, stdio: 'inherit' }
  );
}

const temporary = mkdtempSync(join(tmpdir(), 'ovren-icons-'));
try {
  const application = join(temporary, 'application');
  generate(appSource, application);
  // Windows shell tiles, Linux launcher icons, macOS Dock icons, and the NSIS
  // installer icon must all derive from one canonical source image.
  for (const name of [
    '32x32.png',
    '64x64.png',
    '128x128.png',
    '128x128@2x.png',
    'icon.png',
    'icon.icns',
    'Square30x30Logo.png',
    'Square44x44Logo.png',
    'Square71x71Logo.png',
    'Square89x89Logo.png',
    'Square107x107Logo.png',
    'Square142x142Logo.png',
    'Square150x150Logo.png',
    'Square284x284Logo.png',
    'Square310x310Logo.png',
    'StoreLogo.png',
  ]) {
    copyFileSync(join(application, name), join(output, name));
  }

  const colorTray = join(temporary, 'color-tray');
  generate(markSource, colorTray, [64]);
  copyFileSync(join(colorTray, '64x64.png'), join(output, 'tray-color.png'));
  const templateTray = join(temporary, 'template-tray');
  generate(markTemplateSource, templateTray, [64]);
  copyFileSync(join(templateTray, '64x64.png'), join(output, 'tray-template.png'));
  // tray-template.png is a checked-in monochrome mask of the orbital mark.
  // It must not be replaced with a colored PNG on macOS or Linux.

  mkdirSync(join(output, 'windows'), { recursive: true });
  // Keep the 256 px image first for Tauri's live Windows taskbar icon while
  // preserving all ICO image offsets and available resolution entries.
  const ico = readFileSync(join(application, 'icon.ico'));
  const count = ico.readUInt16LE(4);
  const entries = Array.from({ length: count }, (_, index) => ico.subarray(6 + index * 16, 22 + index * 16));
  entries.sort((left, right) => (right[0] || 256) - (left[0] || 256));
  const orderedIco = Buffer.concat([ico.subarray(0, 6), ...entries, ico.subarray(6 + count * 16)]);
  writeFileSync(join(output, 'icon.ico'), orderedIco);
  writeFileSync(join(output, 'windows/icon.ico'), orderedIco);
  console.log('Generated official Ovren orbital desktop and tray icons.');
} finally {
  rmSync(temporary, { recursive: true, force: true });
}
