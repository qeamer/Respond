// Package frontend holds the Wails WebView UI assets.
package frontend

import "embed"

// Assets is the embedded UI (index.html and any future JS/CSS).
//
//go:embed all:src
var Assets embed.FS
