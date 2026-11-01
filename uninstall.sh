#!/bin/bash
set -euo pipefail
PLIST="$HOME/Library/LaunchAgents/com.github.jayu.free-disk-space-widget.plist"
/bin/launchctl unload "$PLIST" 2>/dev/null || true
rm -f "$PLIST"
pkill -x free-disk-space-widget 2>/dev/null || true
echo "Uninstalled free-disk-space-widget (LaunchAgent removed, process stopped)."
