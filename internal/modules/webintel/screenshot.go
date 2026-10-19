package webintel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type Screenshotter struct {
	BrowserPath string
	OutputDir   string
}

func (s Screenshotter) Capture(ctx context.Context, site, taskID string) (string, error) {
	browser := s.BrowserPath
	if browser == "" {
		for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
			if p, err := exec.LookPath(name); err == nil {
				browser = p
				break
			}
		}
	}
	if browser == "" {
		return "", fmt.Errorf("chromium not found; configure tools.chromiumPath")
	}
	dir := filepath.Join(s.OutputDir, sanitizeID(taskID))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%x.png", fnvHash(site))
	outPath := filepath.Join(dir, name)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browser),
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("ignore-certificate-errors", true),
	)...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	if _, err := chromedp.Run(browserCtx, chromedp.Navigate(site)); err != nil {
		return "", err
	}
	if _, err := chromedp.Run(browserCtx, chromedp.WaitReady(chromedp.CSS("body"))); err != nil {
		return "", err
	}
	buf, err := chromedp.Run(browserCtx, chromedp.CaptureScreenshot())
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, buf, 0o640); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join("/api/v1/screenshots", sanitizeID(taskID), name)), nil
}

func sanitizeID(s string) string {
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
	if s == "" {
		return "unknown"
	}
	return s
}

func fnvHash(s string) uint64 {
	var h uint64 = 1469598103934665603
	for _, b := range []byte(s) {
		h ^= uint64(b)
		h *= 1099511628211
	}
	return h
}
