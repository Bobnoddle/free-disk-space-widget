# Free-disk-space-widget

Simple macOS menubar app showing free disk space on `/`.

## Build

```bash
./build-app.sh
```

Produces `FreeDiskSpaceWidget.app` (LSUIElement agent, no Dock icon, ~3.7MB).

## Run on login

```bash
./create-autosta-file.sh
```

Installs and loads `~/Library/LaunchAgents/com.github.jayu.free-disk-space-widget.plist`.

## Uninstall

```bash
./uninstall.sh
```

## Notes

- Polls every 30s via `syscall.Statfs("/")` (no `diskutil` fork).
- Menubar shows `⛁ 11.19 GB`; tooltip shows used %.
- Change-guard skips menubar IPC when nothing changed.
- For permanent use, move `.app` to `/Applications` and update the plist path.