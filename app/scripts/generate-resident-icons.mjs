/**
 * Derive compact system-surface icons from the approved public vector artwork.
 * Run with `node scripts/generate-resident-icons.mjs`; Tauri supplies the rasterizer
 * and ICO encoder, so icon maintenance needs no extra image-tool dependency.
 */
import { mkdtempSync, readFileSync, writeFileSync, mkdirSync, copyFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';

const root = fileURLToPath(new URL('../', import.meta.url));
const source = readFileSync(join(root, 'public/ovren-icon.svg'), 'utf8');
// The desktop app icon is the approved rounded-square orbital artwork.
// Keep the monochrome tray symbol separate for native menu-bar conventions.
const desktopSource = join(root, 'public/ovren-desktop-icon.png');
if (!source.includes('id="ovren-mark"')) {
  throw new Error('The Ovren mark changed; review the tray icon source before regenerating.');
}
const template = `<svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="16 76 480 360"><g fill="none" stroke="black" stroke-linecap="round" stroke-linejoin="round" stroke-width="34"><path d="M256 256c-45-80-109-128-165-83-54 44-43 126 20 166 57 36 119 9 145-83"/><path d="M256 256c45-80 109-128 165-83 54 44 43 126-20 166-57 36-119 9-145-83"/></g></svg>`;
const temporary = mkdtempSync(join(tmpdir(), 'ovren-icons-'));
const output = join(root, 'src-tauri/icons');
const cli = join(root, 'node_modules/@tauri-apps/cli/tauri.js');
function generate(input, directory, sizes = []) {
  execFileSync(
    process.execPath,
    [cli, 'icon', input, '--output', directory, ...sizes.flatMap(size => ['--png', String(size)])],
    {
      cwd: root,
      stdio: 'inherit',
    }
  );
}
try {
  const colorSource = join(temporary, 'color.svg');
  const templateSource = join(temporary, 'template.svg');
  writeFileSync(colorSource, source);
  writeFileSync(templateSource, template);
  generate(templateSource, join(temporary, 'template'), [36]);
  generate(colorSource, join(temporary, 'color'), [64]);
  const application = join(temporary, 'application');
  generate(desktopSource, application);
  for (const name of [
    '32x32.png', '64x64.png', '128x128.png', '128x128@2x.png',
    'icon.png', 'icon.icns',
    'Square30x30Logo.png', 'Square44x44Logo.png', 'Square71x71Logo.png',
    'Square89x89Logo.png', 'Square107x107Logo.png', 'Square142x142Logo.png',
    'Square150x150Logo.png', 'Square284x284Logo.png', 'Square310x310Logo.png',
    'StoreLogo.png',
  ]) {
    copyFileSync(join(application, name), join(output, name));
  }
  copyFileSync(join(temporary, 'template/36x36.png'), join(output, 'tray-template.png'));
  copyFileSync(join(temporary, 'color/64x64.png'), join(output, 'tray-color.png'));
  mkdirSync(join(output, 'windows'), { recursive: true });
  // Tauri embeds the first ICO entry as the live window icon. Put the largest
  // raster first to avoid enlarging 32 px pixels on high-DPI taskbars. Windows
  // still selects the appropriate size from the full executable icon directory.
  const ico = readFileSync(join(application, 'icon.ico'));
  const count = ico.readUInt16LE(4);
  const entries = Array.from({ length: count }, (_, index) => ico.subarray(6 + index * 16, 22 + index * 16));
  entries.sort((left, right) => (right[0] || 256) - (left[0] || 256));
  const orderedIco = Buffer.concat([ico.subarray(0, 6), ...entries, ico.subarray(6 + count * 16)]);
  writeFileSync(join(output, 'icon.ico'), orderedIco);
  writeFileSync(join(output, 'windows/icon.ico'), orderedIco);
  console.log('Generated Ovren desktop application and tray icons.');
} finally {
  rmSync(temporary, { recursive: true, force: true });
}
