# Ovren desktop application icons

The official rounded-square orbital icon for **Windows, macOS, and Linux** is
[`app/public/ovren-desktop-icon.png`](../../public/ovren-desktop-icon.png).

All desktop app, Windows Store, installer, Linux PNG, and macOS icons in this
directory are generated from this single image by:

```sh
cd app
node scripts/generate-resident-icons.mjs
```

The build runs this generator automatically, so native icons cannot revert to the
previous design when a new installer is produced. The NSIS installer and
uninstaller use `icon.ico`, and Windows also ships `windows/icon.ico`.

The color and monochrome **system-tray** icons remain separate because the tray
uses platform-specific sizing and masking, not the Dock/taskbar launcher icon.
Android and iOS resources are intentionally outside this desktop-only change.
