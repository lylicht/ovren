# Official Ovren desktop app icons

The square orbital artwork at [`app/public/ovren-icon.png`](../../public/ovren-icon.png)
is the **single canonical app icon source** for Windows, macOS, and Linux.
[`ovren-mark.png`](../../public/ovren-mark.png) is the transparent orbital
mark used by the color system-tray icon.

To regenerate the checked-in desktop icons, from `app/` run:

```sh
pnpm install --frozen-lockfile
node scripts/generate-resident-icons.mjs
```

Tauri also runs this generator before packaging. The app icon supplies Linux
PNG launcher sizes, macOS `icon.icns`, Windows `icon.ico` (including NSIS),
and Windows square logos. The full-color tray asset uses the standalone mark.
The checked-in `tray-template.png` is a monochrome version of that mark for
system trays; it must not be replaced by the colored app icon.

The previous purple/red linked-loop artwork is retired. Android and iOS
application icons are intentionally outside the scope of this desktop change.
