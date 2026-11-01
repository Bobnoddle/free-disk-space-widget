package main

import (
	"fmt"
	"log"
	"syscall"
	"time"

	"github.com/getlantern/systray"
)

func main() {
	systray.Run(onReady, onExit)
}

func formatBytesToGb(bytes int64) string {
	return fmt.Sprintf("%.2f", float64(bytes)/1024/1024/1024)
}

var diskEmoji = "⛁"

const (
	pollEvery = 30 * time.Second // disk usage moves slowly
)

// getUsage returns free/total bytes and used % for / via one Statfs syscall.
func getUsage() (freeBytes, totalBytes int64, usedPct float64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/", &st); err != nil {
		return 0, 0, 0, err
	}
	bsize := int64(st.Bsize)
	totalBytes = int64(st.Blocks) * bsize
	freeBytes = int64(st.Bavail) * bsize
	if totalBytes <= 0 {
		return 0, 0, 0, fmt.Errorf("statfs returned non-positive total: %d", totalBytes)
	}
	usedPct = 100 * (1 - float64(freeBytes)/float64(totalBytes))
	usedPct = clamp01(usedPct/100) * 100
	return freeBytes, totalBytes, usedPct, nil
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

var (
	lastTitle string
	lastTip   string
)

func updateFreeSpace() {
	freeBytes, totalBytes, usedPct, err := getUsage()
	if err != nil {
		log.Printf("statfs: %v", err)
		return
	}

	freeGB := formatBytesToGb(freeBytes)
	title := diskEmoji + " " + freeGB + " GB"
	tip := fmt.Sprintf("Free %s GB of %s GB (%.1f%% used)", freeGB, formatBytesToGb(totalBytes), usedPct)

	// Skip IPC to the menubar process when nothing visible changed.
	if title == lastTitle && tip == lastTip {
		return
	}
	if title != lastTitle {
		systray.SetTitle(title)
		lastTitle = title
	}
	if tip != lastTip {
		systray.SetTooltip(tip)
		lastTip = tip
	}
}

func onReady() {
	systray.SetTitle(diskEmoji + " Init")

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")

	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	go func() {
		ticker := time.NewTicker(pollEvery)
		defer ticker.Stop()

		for range ticker.C {
			updateFreeSpace()
		}
	}()

	updateFreeSpace()
}

func onExit() {
	// clean up here
}
