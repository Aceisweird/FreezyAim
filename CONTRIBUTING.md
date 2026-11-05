# Contributing to Freezy Aim

Thanks for helping out. Bug reports, crosshair ideas and pull requests are all welcome.

## Set up

You need [Go](https://go.dev/dl/) 1.24 or newer. There are no other build dependencies.

```
cd src
go test ./...
```

## How it fits together

The app window, saved data and crosshair drawing are shared by every system.

- `ui/` is the app window, a small web page with no framework or build step.
- `crosshair.go` and `shapes.go` hold the settings and draw the crosshair.
- `library.go` and `presets.go` hold saved data and built-in crosshairs.
- `server.go` is the private local server behind the app window.
- `main_windows.go`, `appwindow_windows.go` and `system_windows.go` are the native Windows
  overlay, tray icon, shortcuts and startup entry.
- `main_unix.go` and `overlay.py` do the same job on macOS and Linux.

## Work on the app window

You can work on it from any macOS or Linux system:

```
cd src
go run . some-folder
```

Open the printed link in a browser. The folder holds test data, and nothing appears on
screen in this mode.

## Build the app

For Windows, from any system:

```
cd src
GOOS=windows GOARCH=amd64 go build -trimpath -tags nethttpomithttp2 -ldflags "-s -w -H=windowsgui" -o ../dist/FreezyAim.exe .
```

For macOS or Linux, on that system:

```
cd src
go build -trimpath -tags nethttpomithttp2 -ldflags "-s -w" -o freezy-aim .
```

The `nethttpomithttp2` tag leaves out web features the app doesn't use, which makes the
file smaller.

## Common changes

- **Add a built-in crosshair:** add it to `presets` in `src/presets.go`.
- **Add a section:** add its name to `categories` in `src/presets.go`.
- **Add an editor base:** add it to `bases` in `src/presets.go`.
- **Add a crosshair setting:** add it to `Config` in `src/crosshair.go`, draw it in
  `src/shapes.go`, and add a control in `src/ui/editor.js`.
- **Add a font:** add it to `fontChoices` in `src/library.go`.
- **Add an app setting:** add it to `Options` in `src/library.go` and to
  `src/ui/settings.js`.

Only add work you have the right to share, and list anything third-party in
`THIRD_PARTY_NOTICES.md`.

## Before opening a pull request

- Run `gofmt -w .` and `go test ./...` in `src/`.
- Add a test for changes to drawing or saved data.
- Add a line to `CHANGELOG.md` for anything people will notice.

## Publish a release

1. Update the version in `src/library.go`, `src/resources/app.rc` and `CHANGELOG.md`.
2. Run `src/resources/build.sh` to rebuild the icon and version details.
3. Rebuild `dist/FreezyAim.exe` with the Windows command above.
4. Commit, then tag and push the release:

   ```
   git tag v1.0
   git push origin v1.0
   ```

The release workflow tests the code, builds the app for every system and publishes it.
