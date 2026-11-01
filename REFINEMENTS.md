# Refinement Plan: free-disk-space-widget

## 1. Customisable refresh (alternative to "Refresh now")

**Approach:** Add a "Refresh interval" submenu with preset options (10s, 30s, 60s, 5m) plus "Refresh now". Persist the choice in `~/Library/Preferences/com.github.jayu.free-disk-space-widget.plist` via `CFPreferences` (or a simple JSON file in `~/Library/Application Support/`). On startup, read the saved interval; on selection, rewrite the ticker.

**Implementation:**
- Store `pollInterval time.Duration` as a package var (default 30s).
- In `onReady()`, load saved interval from prefs before starting ticker.
- Add submenu: `mInterval := systray.AddMenuItem("Refresh interval", "")` with children `m10s, m30s, m60s, m5m`. Check the active one with `m.Checked = true`.
- "Refresh now" item calls `updateFreeSpace()` directly.
- On selection: save pref, stop old ticker, start new one with selected duration, re-check menu items.
- Ticker restart needs a `context.CancelFunc` or channel-based restart pattern since `time.Ticker` can't change its period.

**Complexity:** ~40 lines. Low risk. Main subtlety: restarting the ticker goroutine cleanly (use a `restartCh chan time.Duration`).

---

## 2. Launch-at-login toggle

**Approach:** Use `SMAppService` (macOS 13+) to register/unregister the app as a login item programmatically, instead of relying solely on the LaunchAgent plist. This gives a native toggle that shows up in System Settings → General → Login Items.

**Implementation:**
- Import `github.com/...` — Go has no first-party `SMAppService` binding. Options:
  - **cgo** with `#import <ServiceManagement/ServiceManagement.h>` and call `SMAppService.mainAppService.register()`.
  - **Shell out** to `osascript` or `sfltool` — fragile.
  - **Keep the plist approach** but add a menu toggle that loads/unloads it via `launchctl bootstrap`/`bootout`. Simpler, no cgo, but doesn't show in System Settings.
- Recommended: cgo + SMAppService. Add a build tag `//go:build darwin` and a small `login_item_darwin.go` file.
- Menu: `mLogin := systray.AddMenuItemCheckbox("Launch at login", "", isLoginItem())`. On click, toggle registration and update checkbox state.

**Complexity:** ~60 lines + cgo. Medium risk (cgo cross-compilation, entitlements). Alternative (plist toggle) is ~20 lines, low risk, but less "native."

---

## 3. Multiple volumes

**Approach:** Enumerate mounted volumes via `syscall.Getfsstat` or `golang.org/x/sys/unix.Getfsstat`, filter to local read-write volumes (skip network, read-only, system snapshots), and show each in a submenu with its free/total/used%.

**Implementation:**
- Replace `getUsage()` (single `/`) with `getVolumes() []VolumeInfo` returning `{path, name, freeBytes, totalBytes, usedPct}`.
- In `onReady()`, build a "Volumes" submenu dynamically: for each volume, `mVol := mVolumes.AddSubMenuItem(name, "")` with title `⛁ 11.19 GB  (95% used)`.
- Poll all volumes each tick; update each submenu item's title.
- Track which volume is "primary" (the one shown in the menubar title). Default `/`; let user pick via submenu radio buttons.
- Skip volumes where `st.Flags & MNT_RDONLY != 0` or where the filesystem type is `nfs`/`smbfs`/`afpfs`.

**Complexity:** ~80 lines. Medium risk (volume enumeration edge cases, APFS snapshot filtering). Main subtlety: `Getfsstat` returns all mounts including system ones; filtering is essential to avoid noise.

---

## 4. Notarization (distribution)

**Approach:** Sign with a Developer ID certificate and submit to Apple's notary service. This is a build/CI concern, not a code change.

**Implementation:**
- Requires Apple Developer account ($99/yr).
- Update `build-app.sh`:
  ```bash
  codesign --force --deep --options runtime --sign "Developer ID Application: Name (TEAM)" FreeDiskSpaceWidget.app
  ditto -c -k --keepParent FreeDiskSpaceWidget.app FreeDiskSpaceWidget.zip
  xcrun notarytool submit FreeDiskSpaceWidget.zip --apple-id ... --team-id ... --password ... --wait
  xcrun stapler staple FreeDiskSpaceWidget.app
  ```
- Add hardened runtime entitlements if needed (none required for this app — no network, no special APIs).
- Optionally add a GitHub Actions workflow for automated builds.

**Complexity:** Build script changes only. No Go code changes. Cost: $99/yr Apple Developer account.

---

## Recommended priority

| # | Change | Effort | Value | Risk |
|---|--------|--------|-------|------|
| 1 | Customisable refresh + "Refresh now" | Low | High | Low |
| 2 | Launch-at-login toggle (plist-based, no cgo) | Low | Medium | Low |
| 2b | Launch-at-login toggle (SMAppService, cgo) | Medium | Medium | Medium |
| 3 | Multiple volumes | Medium | Medium | Medium |
| 4 | Notarization | Low (script only) | Low (personal use) | Low |

**Suggested order:** 1 → 2 (plist-based) → 3 → 4 (only if distributing).

Want me to implement 1 and 2 (plist-based toggle) now? Those are the highest-value, lowest-risk items. Item 3 is a bigger change and item 4 requires an Apple Developer account.
