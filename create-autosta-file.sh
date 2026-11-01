#!/bin/bash
set -euo pipefail

# Points launchd at the installed .app bundle in /Applications.
APP_BIN="/Applications/FreeDiskSpaceWidget.app/Contents/MacOS/free-disk-space-widget"
PLIST="$HOME/Library/LaunchAgents/com.github.jayu.free-disk-space-widget.plist"

if [[ ! -x "$APP_BIN" ]]; then
  echo "Missing $APP_BIN — install the app to /Applications first" >&2
  exit 1
fi

# Remove legacy label from the old script if present.
/bin/launchctl unload "$HOME/Library/LaunchAgents/free-disk-space-widget.autostart.plist" 2>/dev/null || true
rm -f "$HOME/Library/LaunchAgents/free-disk-space-widget.autostart.plist"

cat > "$PLIST" << EOL
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>com.github.jayu.free-disk-space-widget</string>
    <key>ProgramArguments</key>
    <array>
      <string>$APP_BIN</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
      <key>SuccessfulExit</key>
      <false/>
    </dict>
    <key>ThrottleInterval</key>
    <integer>30</integer>
  </dict>
</plist>
EOL

chmod 644 "$PLIST"
/bin/launchctl unload "$PLIST" 2>/dev/null || true
/bin/launchctl load "$PLIST"

echo "Loaded $PLIST -> $APP_BIN"
