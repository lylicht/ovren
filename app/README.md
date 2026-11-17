# Ovren desktop application

Ovren by Licht combines a Vue interface with a Rust core for storage analysis, reviewed cleanup, privacy, startup management, applications, and system maintenance. It also includes user-started network measurements and Windows/GNOME pointer controls.

## Development

Requirements: Node.js 22+, pnpm 11.13.1, Rust 1.99.0, and the native Tauri dependencies for your operating system.

```sh
corepack pnpm install --frozen-lockfile
corepack pnpm tauri:dev
```

Run `corepack pnpm check` for source gates, frontend coverage, asset compilation, Rust formatting, Clippy, compilation, and native tests. Build the Windows installer with `corepack pnpm tauri:build` or the Linux installers with `corepack pnpm tauri:build:linux`. Packaging builds frontend assets and the Windows privileged helper; use the complete check before preparing a release.

## GitHub releases

Pushing a `v*` tag runs the root release workflow for Windows, macOS and Linux installers, checksums, signatures, and updater metadata. [Ovren releases](https://github.com/lylicht/ovren/releases) contain published artifacts. Portable builds open the matching release page for manual downloads. The inherited Windows certificate is self-signed as `CN=bxane`.

## AI providers

Configure your own provider endpoint and credentials. Hosted AI is not provided by Ovren.

## Source and license

Ovren is made and maintained by Licht, building on the earlier Wyre work by bxane and MangoDisk by harry0703 and contributors. It is distributed under [GPL-3.0-only](../LICENSE); original author credit and third-party notices are retained. See [UPSTREAMS.md](../UPSTREAMS.md) for source provenance.

The network check transfers about 16 MB to Cloudflare when started. Mouse changes require review and can be undone once.
