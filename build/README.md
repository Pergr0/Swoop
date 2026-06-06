# Build Directory

The build directory is used to house all the build files and assets for your application. 

The structure is:

* bin - Output directory
* darwin - macOS specific files
* windows - Windows specific files
* appicon.png - Source icon (1024x1024 PNG) for all platforms

## Mac

The `darwin` directory holds files specific to Mac builds.
These may be customised and used as part of the build. To return these files to the default state, simply delete them
and
build with `wails build`.

The directory contains the following files:

- `Info.plist` - the main plist file used for Mac builds. It is used when building using `wails build`.
- `Info.dev.plist` - same as the main plist file but used when building using `wails dev`.
- `iconfile.icns` - Optional cache (not checked in). Wails writes a fresh `iconfile.icns`
  into `build/bin/swoop.app/Contents/Resources/` from `appicon.png` on each pack.
  `scripts/build.sh` deletes any `build/darwin/iconfile.icns` and the previous
  `swoop.app` before building so Finder always shows the S mark from `appicon.png`.

## Windows

The `windows` directory contains the manifest and rc files used when building with `wails build`.
These may be customised for your application. To return these files to the default state, simply delete them and
build with `wails build`.

- `icon.ico` - Generated at build time from `build/appicon.png` (not checked in).
  Wails only creates it when missing; the build scripts regenerate it with
  `scripts/genicon`, remove `build/bin/`, and drop any `*-res.syso`
  in the repo root before each Windows build (`wails build -s -f`) so Explorer
  shows the S mark from `appicon.png`, not Wails' default W. Edit `appicon.png`
  (1024x1024), then rebuild.
- `installer/*` - The files used to create the Windows installer. These are used when building using `wails build`.
- `info.json` - Application details used for Windows builds. The data here will be used by the Windows installer,
  as well as the application itself (right click the exe -> properties -> details)
- `wails.exe.manifest` - The main application manifest file.

## Linux

Wails does not package Linux icons or `.desktop` files. Swoop handles this in
two layers (same `appicon.png` source as Windows/macOS; refreshed every build):

- **Runtime** - `build/appicon.png` is embedded in the binary and passed to
  Wails `linux.Options.Icon` (GTK window and taskbar icon while the app runs).
  Rebuilding after editing `appicon.png` picks up the new bytes automatically.
- **Build output** - `scripts/build.sh` removes any stale `build/bin/swoop.png`,
  then copies a fresh `swoop.png` into `build/bin/` and writes `swoop.desktop`
  plus `install-desktop-entry.sh`. Run the installer once to register Swoop in
  the application menu with the correct icon:

  ```bash
  ./build/bin/install-desktop-entry.sh
  ```