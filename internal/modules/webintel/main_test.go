package webintel

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/red-copilot/red-x/internal/platform/pdtools"
)

func TestMain(m *testing.M) {
	pdtools.ConfigureLogging(slog.New(slog.NewTextHandler(io.Discard, nil)), false)
	os.Exit(m.Run())
}
