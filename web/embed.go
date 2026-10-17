// Package web embeds the static frontend so the server is a single binary.
package web

import "embed"

// Files holds index.html, app.js and style.css.
//
//go:embed index.html app.js style.css
var Files embed.FS
