const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');

test('accepts a release tag when root, app, Tauri, Cargo, and changelog versions agree', async () => {
  const { validateReleaseVersion } = await import('./release-preflight.mjs');

  assert.equal(
    validateReleaseVersion({
      tag: 'v1.0.0',
      rootVersion: '1.0.0',
      packageVersion: '1.0.0',
      tauriVersion: '1.0.0',
      cargoVersion: '1.0.0',
      changelog: '# Changelog\n\n## 1.0.0 — 2026-10-20\n\nStable release notes.\n',
    }),
    '1.0.0'
  );
});

test('rejects a release tag that does not match package versions', async () => {
  const { validateReleaseVersion } = await import('./release-preflight.mjs');

  assert.throws(
    () => validateReleaseVersion({
      tag: 'v1.1.0',
      rootVersion: '1.1.0',
      packageVersion: '1.0.0',
      tauriVersion: '1.0.0',
      cargoVersion: '1.0.0',
      changelog: '# Changelog\n\n## 1.1.0 — 2026-10-21\n\nRelease notes.\n',
    }),
    /must match root, app, Tauri, and Cargo workspace versions/
  );
});

test('rejects a release tag that does not match the Cargo workspace version', async () => {
  const { validateReleaseVersion } = await import('./release-preflight.mjs');

  assert.throws(
    () => validateReleaseVersion({
      tag: 'v1.0.0',
      rootVersion: '1.0.0',
      packageVersion: '1.0.0',
      tauriVersion: '1.0.0',
      cargoVersion: '1.1.9',
      changelog: '# Changelog\n\n## 1.0.0 — 2026-10-21\n\nRelease notes.\n',
    }),
    /Cargo workspace versions/
  );
});

test('rejects a release tag without a matching changelog section', async () => {
  const { validateReleaseVersion } = await import('./release-preflight.mjs');

  assert.throws(
    () => validateReleaseVersion({
      tag: 'v1.0.0',
      rootVersion: '1.0.0',
      packageVersion: '1.0.0',
      tauriVersion: '1.0.0',
      cargoVersion: '1.0.0',
      changelog: '# Changelog\n\n## [Unreleased]\n\nDraft notes.\n',
    }),
    /matching changelog section/
  );
});

test('rejects malformed Ovren tags', async () => {
  const { validateReleaseVersion } = await import('./release-preflight.mjs');

  assert.throws(
    () => validateReleaseVersion({
      tag: 'wyre-v1.0.0',
      rootVersion: '1.0.0',
      packageVersion: '1.0.0',
      tauriVersion: '1.0.0',
      cargoVersion: '1.0.0',
      changelog: '# Changelog\n\n## 1.0.0 — 2026-10-20\n\nStable release notes.\n',
    }),
    /Expected a v semantic-version tag/
  );
});

test('requires the lazy resizable file browser window in every platform Tauri overlay', async () => {
  const { validateFileBrowserWindowConfigs } = await import('./release-preflight.mjs');
  const configs = Object.fromEntries(
    ['windows', 'macos', 'linux'].map((platform) => [
      platform,
      JSON.parse(readFileSync(path.join(__dirname, `../app/src-tauri/tauri.${platform}.conf.json`), 'utf8')),
    ])
  );

  assert.doesNotThrow(() => validateFileBrowserWindowConfigs(configs));
});
