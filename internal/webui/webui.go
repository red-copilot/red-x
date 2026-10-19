package webui

import "embed"

// Dist is replaced by the Vite build during the release image build.
//
//go:embed all:dist
var Dist embed.FS
