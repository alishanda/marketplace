package web

import "embed"

//go:embed index.html app.js styles.css assets/*
var FS embed.FS
