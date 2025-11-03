package web

import "embed"

//go:embed templates/* templates/components/* static/css/* static/js/*
var Assets embed.FS
