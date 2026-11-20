# Changelog

## 1.3.3 — 2026-11-20

- Refined the desktop and tray icon with the approved flat Ovren artwork across Windows, macOS, and Linux.
- Added Desktop and standard user folders to the in-app explorer, with persistent favorites and roomier spacing.
- Reduced repeated directory reads with a bounded short-lived cache, and removed a duplicate native listing sort.
- Localized new explorer locations and favorite actions in every supported language.

## 1.3.2 — 2026-11-20

- Adopted the official orbital desktop icon across Windows, macOS, and Linux, including installers and native tray artwork.
- Expanded the file browser into an Ovren-styled native file explorer with navigation history, virtualized details/grid views, selection, search, and keyboard shortcuts.
- Added native Rust create, rename, copy, move, and Trash operations with no-overwrite safety and staged copies.
- Connected the explorer to storage analysis, large-file discovery, and duplicate-file tools.
- Added localization and filesystem regression tests.
- Keep folder navigation and directory links inside Ovren, and remove the system file-manager handoff from Explorer.
- Use Ovren's address bar, recent folders, volumes, and drag-and-drop instead of native file and folder pickers.
- Add a platform-aware latest-installer download from About.

## 1.3.0 — 2026-11-18

- Added an Ovren file browser with folder navigation, recursive name search, and opening items in their system apps.

## 1.2.1 — 2026-11-18

- Added OvrenNET connection selection, weighting, speed limits, IPv6 and MTU controls, automatic pause, route recovery, a loopback SOCKS5 proxy, and tray quick actions.
- Added per-adapter total and routed traffic rates, and show the speed test server and Cloudflare edge location.
- Regenerate Windows, macOS, and Linux package icons from the approved Ovren mark during builds; hide the Network page scrollbar while keeping scrolling available.
- Added localized OvrenNET controls across all supported languages.

## 1.2.0 — 2026-11-18

- Added Persian (fa-IR) localization and right-to-left layout support.
- Refined application spacing and updated the Ovren icon.
- Improved reliability and diagnostics for network speed checks.
## 1.1.0 — 2026-11-18

- Added OvrenNET network merging controls and live adapter traffic to the Network page.
- Added the OvrenNET helper for Windows, macOS and Linux builds, including its bundled platform dependencies and license notices.

## 1.0.0 — 2026-11-17

Ovren is the new desktop identity for the project, made and maintained by Licht.

- Added the original Ovren logo and regenerated the platform icons from its vector mark.
- Updated the application identity, metadata, interface names, updater links, and release workflow to Ovren.
- Switched the interface symbol source to SFStudio while keeping SF Symbols out of the brand mark and app icon.
- Replaced the warm-gray palette with independent purple and blood-red themes for light and dark mode.
- Kept existing Wyre installations and data separate; Ovren does not import them automatically.

## 2.0.0 — 2026-11-10

Wyre 2.0 gets an Apple-style look and drops every service it inherited from MangoDisk. Wyre now talks only to servers you choose.

- Restyled the interface after Apple's design language, with SF Symbols icons from GlyphStudio.
- Removed the hosted MangoDisk AI service, its quota display, reply ratings, installation ID and request headers. AI explanations now use only the provider you configure. Saved free-service settings load as unconfigured.
- Portable builds now open Wyre's GitHub release page for updates instead of a MangoDisk download link.
- Keep the desktop window's native minimize animation and enlarge custom window-control glyphs.
- Generate resident tray icons from the vector Wyre mark and document the correct vector source for packaged icons.
- Validate release tags against all app versions and the matching changelog before starting platform builds.

## Wyre 1.0.0 — 2026-10-20

- Replaced the standalone Electron shell with the full Wyre Vue/Tauri application as Wyre's base.
- Added on-demand network latency, jitter, download, and upload measurement.
- Added live network interface status, local IPv4 details, and current traffic rates to the Network page.
- Added native Windows and GNOME pointer speed and acceleration controls with review and one-step undo.
- Rebranded the desktop app, package metadata, icons, and release workflow as Wyre.
- Added Windows NSIS, Linux AppImage and Debian installers, SHA-256 checksums, Tauri/GPG signatures, and this changelog to GitHub Releases. Windows uses a self-signed `CN=bxane` code-signing certificate, so Windows reports an untrusted publisher until users opt into trusting the included certificate.
- Switched the combined source to GPL-3.0-only and preserved original author credit and license notices.

## 0.3.0 — 2026-10-19

- Integrated Wyre's native scan engine as an on-demand cleanup preview; Wyre does not apply cleanup or expose matched local paths.
- Added native Wyre CLI builds to Windows/Linux installer and signed release workflows.
- Added the original GPL-3.0-only license to packaged resources and documented its GPL-3.0 source.

## 0.2.0 — 2026-10-19

- Added an opt-in local storage scan that lists the largest files in a chosen folder without deleting anything.
- Added active IPv4 interface and address details to the network page.
- Added review-before-apply dialogs and one-step undo for power profile and pointer changes.
- Added failed HTTP probe percentage to the on-demand network readout.
- Documented the scope and decisions for the requested reference projects.
- Improved mobile layout, focus visibility, and dialog styling.

## 0.1.0 — 2026-10-19

- Introduced the Wyre desktop control panel for Windows and Linux.
- Added a local CPU and memory overview with user-selected performance and balanced power profiles.
- Added an on-demand Cloudflare network check for latency, jitter, download, and upload speed.
- Added pointer speed and acceleration controls for Windows and GNOME Linux.
- Added the Wyre wordmark, SVG app mark, app icon, and Windows/Linux installer configuration.
- Added a signed multi-platform GitHub release workflow.
