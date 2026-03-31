package frontend

import "embed"

//go:embed all:dist
var Assets embed.FS

//go:embed login.html
var LoginHTML []byte
