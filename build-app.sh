#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")"

APP="FreeDiskSpaceWidget.app"
BIN="$APP/Contents/MacOS/free-disk-space-widget"

export PATH=/opt/homebrew/bin:$PATH
go build -trimpath -ldflags="-s -w" -o "$BIN" .

mkdir -p "$APP/Contents"
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleIdentifier</key>
  <string>com.github.jayu.free-disk-space-widget</string>
  <key>CFBundleName</key>
  <string>FreeDiskSpaceWidget</string>
  <key>CFBundleVersion</key>
  <string>1.0</string>
  <key>CFBundleExecutable</key>
  <string>free-disk-space-widget</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
  <key>MinimumSystemVersion</key>
  <string>13.0</string>
</dict>
</plist>
PLIST

# Ad-hoc sign so the bundle launches without Gatekeeper complaints when built locally.
codesign --force --deep --sign - "$APP" 2>&1 | head -3 || true

echo "Built $BIN ($(du -h "$BIN" | cut -f1))"
echo "Bundle: $(pwd)/$APP (LSUIElement hides Dock icon)"
