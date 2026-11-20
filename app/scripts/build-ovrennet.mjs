import { spawnSync } from 'node:child_process';
import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const appDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const sourceDir = resolve(appDir, 'ovrennet');
const binaryDir = resolve(appDir, 'src-tauri', 'binaries', 'ovrennet');
const target =
  process.env.TAURI_ENV_TARGET_TRIPLE ??
  spawnSync('rustc', ['--print', 'host-tuple'], { encoding: 'utf8' }).stdout?.trim();
const platforms = {
  'x86_64-pc-windows-msvc': ['windows', 'amd64'],
  'x86_64-unknown-linux-gnu': ['linux', 'amd64'],
  'aarch64-unknown-linux-gnu': ['linux', 'arm64'],
  'x86_64-apple-darwin': ['darwin', 'amd64'],
  'aarch64-apple-darwin': ['darwin', 'arm64'],
};
const platform = platforms[target];

if (!platform) throw new Error(`OvrenNET does not support target ${target ?? 'unknown'}`);

mkdirSync(binaryDir, { recursive: true });
copyFileSync(resolve(sourceDir, 'licenses', 'wintun.txt'), resolve(binaryDir, 'wintun-LICENSE.txt'));
const extension = platform[0] === 'windows' ? '.exe' : '';
const output = resolve(binaryDir, `ovrenNET-${target}${extension}`);
const build = spawnSync('go', ['build', '-trimpath', '-buildvcs=false', '-o', output, './cmd/ovrennet'], {
  cwd: sourceDir,
  encoding: 'utf8',
  env: { ...process.env, CGO_ENABLED: '0', GOOS: platform[0], GOARCH: platform[1] },
  stdio: 'inherit',
});

if (build.error) throw build.error;
if (build.status !== 0) process.exit(build.status ?? 1);

if (platform[0] === 'windows') {
  const fetchEnvironment = { ...process.env };
  delete fetchEnvironment.PSModulePath;
  const fetch = spawnSync(
    'powershell.exe',
    [
      '-NoProfile',
      '-NonInteractive',
      '-ExecutionPolicy',
      'Bypass',
      '-File',
      resolve(sourceDir, 'scripts', 'fetch-wintun.ps1'),
      '-OutDir',
      binaryDir,
    ],
    { stdio: 'inherit', env: fetchEnvironment }
  );
  if (fetch.error) throw fetch.error;
  if (fetch.status !== 0) process.exit(fetch.status ?? 1);
}
