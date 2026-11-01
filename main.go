package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/getlantern/systray"
)

func main() {
	systray.Run(onReady, onExit)
}

func formatBytesToGb(bytes int64) string {
	return fmt.Sprintf("%.2f", float64(bytes)/1024/1024/1024)
}

var diskEmoji = "⛁"

// ---- config (~/Library/Application Support/.../config.json) ----

type config struct {
	IntervalSec int    `json:"interval_sec"` // poll interval
	PrimaryPath string `json:"primary_path"` // volume shown in title
}

var cfgPath string

func loadConfig() config {
	c := config{IntervalSec: 30, PrimaryPath: "/"}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	if c.IntervalSec < 5 {
		c.IntervalSec = 5
	}
	if c.PrimaryPath == "" {
		c.PrimaryPath = "/"
	}
	return c
}

func saveConfig(c config) {
	b, _ := json.Marshal(c)
	_ = os.WriteFile(cfgPath, b, 0644)
}

// ---- volume enumeration ----

type volInfo struct {
	path    string
	name    string
	free    int64
	total   int64
	usedPct float64
}

// getVolumes returns local, writable volumes (skips network/readonly/system).
func getVolumes() []volInfo {
	var buf [256]unix.Statfs_t
	n, err := unix.Getfsstat(buf[:], unix.MNT_NOWAIT)
	if err != nil || n <= 0 {
		v, err := singleVol("/")
		if err != nil {
			return nil
		}
		return []volInfo{v}
	}
	out := make([]volInfo, 0, n)
	seen := map[string]bool{}
	for i := 0; i < n && i < len(buf); i++ {
		st := &buf[i]
		bsize := int64(st.Bsize)
		total := int64(st.Blocks) * bsize
		if total <= 0 {
			continue
		}
		mp := unix.ByteSliceToString(st.Mntonname[:])
		if mp == "" || seen[mp] {
			continue
		}
		if st.Flags&unix.MNT_RDONLY != 0 && mp != "/" {
			continue
		}
		fsType := unix.ByteSliceToString(st.Fstypename[:])
		switch fsType {
		case "nfs", "smbfs", "afpfs", "webdav", "fuse", "devfs", "autofs", "nullfs":
			continue
		}
		if mp != "/" && (mp == "/System/Volumes/VM" || mp == "/System/Volumes/Preboot" ||
			mp == "/System/Volumes/Update" || mp == "/System/Volumes/xarts" ||
			mp == "/System/Volumes/iSCPreboot" || mp == "/System/Volumes/Hardware") {
			continue
		}
		free := int64(st.Bavail) * bsize
		usedPct := 100 * (1 - float64(free)/float64(total))
		usedPct = clamp01(usedPct/100) * 100
		name := filepath.Base(mp)
		if mp == "/" {
			name = "Macintosh HD"
		} else if mp == "/System/Volumes/Data" {
			name = "Macintosh HD (Data)"
		} else if name == "" || name == "." || name == "/" {
			name = filepath.Base(mp)
		}
		out = append(out, volInfo{path: mp, name: name, free: free, total: total, usedPct: usedPct})
		seen[mp] = true
	}
	if len(out) == 0 {
		v, err := singleVol("/")
		if err == nil {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func singleVol(path string) (volInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return volInfo{}, err
	}
	bsize := int64(st.Bsize)
	total := int64(st.Blocks) * bsize
	free := int64(st.Bavail) * bsize
	if total <= 0 {
		return volInfo{}, fmt.Errorf("bad total")
	}
	usedPct := 100 * (1 - float64(free)/float64(total))
	usedPct = clamp01(usedPct/100) * 100
	return volInfo{path: path, name: "Macintosh HD", free: free, total: total, usedPct: usedPct}, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ---- state ----

var (
	cfg         config
	lastTitle   string
	lastTip     string
	volItems    map[string]*systray.MenuItem // path -> submenu item
	primaryPath string
	intervalCh  chan time.Duration
	loginToggle *systray.MenuItem
)

func updateVolumes() {
	vols := getVolumes()
	// Build primary title from selected volume.
	var primary *volInfo
	for i := range vols {
		if vols[i].path == primaryPath {
			primary = &vols[i]
			break
		}
	}
	if primary == nil && len(vols) > 0 {
		primary = &vols[0]
		primaryPath = primary.path
	}

	if primary != nil {
		freeGB := formatBytesToGb(primary.free)
		title := diskEmoji + " " + freeGB + " GB"
		tip := fmt.Sprintf("Free %s GB of %s GB (%.1f%% used) — %s", freeGB, formatBytesToGb(primary.total), primary.usedPct, primary.path)
		if title != lastTitle {
			systray.SetTitle(title)
			lastTitle = title
		}
		if tip != lastTip {
			systray.SetTooltip(tip)
			lastTip = tip
		}
	}

	// Update volume submenu items.
	for _, v := range vols {
		if it, ok := volItems[v.path]; ok {
			it.SetTitle(fmt.Sprintf("%s  —  %s GB free (%.0f%% used)", v.name, formatBytesToGb(v.free), v.usedPct))
			if v.path == primaryPath {
				it.Check()
			} else {
				it.Uncheck()
			}
		}
	}
}

func isLoginItem() bool {
	out, err := exec.Command("launchctl", "print", "gui/"+fmt.Sprint(os.Getuid())+"/com.github.jayu.free-disk-space-widget").Output()
	if err != nil {
		return false
	}
	return string(out) != ""
}

func toggleLoginItem() {
	label := "com.github.jayu.free-disk-space-widget"
	plist := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", label+".plist")
	if isLoginItem() {
		exec.Command("launchctl", "bootout", "gui/"+fmt.Sprint(os.Getuid()), plist).Run()
	} else {
		exec.Command("launchctl", "bootstrap", "gui/"+fmt.Sprint(os.Getuid()), plist).Run()
	}
	// Update checkbox.
	if isLoginItem() {
		loginToggle.Check()
	} else {
		loginToggle.Uncheck()
	}
}

func onReady() {
	home, _ := os.UserHomeDir()
	cfgPath = filepath.Join(home, "Library", "Application Support", "free-disk-space-widget", "config.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	cfg = loadConfig()
	primaryPath = cfg.PrimaryPath

	systray.SetTitle(diskEmoji + " Init")

	// Menu items in display order.
	mRefresh := systray.AddMenuItem("Refresh now", "Update immediately")
	mInterval := systray.AddMenuItem("Refresh interval", "")
	mVolumes := systray.AddMenuItem("Volumes", "Mounted volumes")
	loginToggle = systray.AddMenuItemCheckbox("Launch at login", "", isLoginItem())
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

	// --- Volumes submenu ---
	volItems = make(map[string]*systray.MenuItem)

	// --- Refresh interval submenu ---
	presets := []struct {
		label string
		sec   int
	}{
		{"10 seconds", 10},
		{"30 seconds (default)", 30},
		{"1 minute", 60},
		{"5 minutes", 300},
	}
	intervalItems := make([]*systray.MenuItem, len(presets))
	for i, p := range presets {
		mi := mInterval.AddSubMenuItem(p.label, "")
		intervalItems[i] = mi
		sec := p.sec
		go func(item *systray.MenuItem, s int) {
			<-item.ClickedCh
			cfg.IntervalSec = s
			saveConfig(cfg)
			intervalCh <- time.Duration(s) * time.Second
			// Update checkmarks.
			for j, it := range intervalItems {
				if presets[j].sec == s {
					it.Check()
				} else {
					it.Uncheck()
				}
			}
		}(mi, sec)
	}
	// Check current.
	for i, p := range presets {
		if p.sec == cfg.IntervalSec {
			intervalItems[i].Check()
		}
	}

	// Refresh now.
	go func() {
		for range mRefresh.ClickedCh {
			updateVolumes()
		}
	}()

	// --- Launch at login toggle ---
	go func() {
		for range loginToggle.ClickedCh {
			toggleLoginItem()
		}
	}()

	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	// Volume selection handler (populated after first enumeration).
	intervalCh = make(chan time.Duration, 1)

	// Ticker goroutine with restart support.
	go func() {
		dur := time.Duration(cfg.IntervalSec) * time.Second
		ticker := time.NewTicker(dur)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				updateVolumes()
			case newDur := <-intervalCh:
				ticker.Stop()
				ticker = time.NewTicker(newDur)
				cfg.IntervalSec = int(newDur.Seconds())
				saveConfig(cfg)
				updateVolumes()
			}
		}
	}()

	// Initial enumeration + populate volume submenu.
	updateVolumes()
	vols := getVolumes()
	for _, v := range vols {
		vi := v
		mi := mVolumes.AddSubMenuItem(vi.name, vi.path)
		volItems[vi.path] = mi
		mi.SetTitle(fmt.Sprintf("%s  —  %s GB free", vi.name, formatBytesToGb(vi.free)))
		// Radio-style: clicking sets primary.
		go func(item *systray.MenuItem, path string) {
			for range item.ClickedCh {
				primaryPath = path
				cfg.PrimaryPath = path
				saveConfig(cfg)
				lastTitle = "" // force title update
				lastTip = ""
				updateVolumes()
			}
		}(mi, vi.path)
	}
	// Mark primary.
	for _, v := range vols {
		if v.path == primaryPath {
			volItems[v.path].Check()
		}
	}
}

func onExit() {}
