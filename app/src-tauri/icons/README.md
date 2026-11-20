# Official Ovren desktop app icons

The square artwork at [`app/public/ovren-icon.png`](../../public/ovren-icon.png)
is the canonical app icon source for Windows, macOS, and Linux.
[`ovren-mark.png`](../../public/ovren-mark.png) and
[`ovren-mark-mono.png`](../../public/ovren-mark-mono.png) are the color and
monochrome marks used by system-tray icons.

To regenerate the checked-in desktop icons, from `app/` run:

```sh
pnpm install --frozen-lockfile
node scripts/generate-resident-icons.mjs
```

Tauri also runs this generator before packaging. The app icon supplies Linux
PNG launcher sizes, macOS `icon.icns`, Windows `icon.ico` (including NSIS),
and Windows square logos. The color and monochrome tray assets come from their
matching standalone marks.

The previous purple/red linked-loop artwork is retired. Android and iOS
application icons are intentionally outside the scope of this desktop change.
