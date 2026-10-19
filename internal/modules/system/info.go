package system

import (
	"runtime"
	"time"

	"github.com/red-copilot/red-x/internal/platform/pdtools"
)

var startedAt = time.Now()

type Info struct {
	Version   string            `json:"version"`
	GoVersion string            `json:"goVersion"`
	OS        string            `json:"os"`
	Arch      string            `json:"arch"`
	UptimeSec int64             `json:"uptimeSec"`
	StartedAt string            `json:"startedAt"`
	Scanners  map[string]string `json:"scanners"`
}

func InfoNow(version string) Info {
	return Info{
		Version: version, GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH,
		UptimeSec: int64(time.Since(startedAt).Seconds()), StartedAt: startedAt.UTC().Format(time.RFC3339),
		Scanners: pdtools.Versions(),
	}
}
