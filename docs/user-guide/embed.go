// Package userguide ships the user guide inside the daemon, so the
// assistant can answer questions about Yggdrasil from it.
package userguide

import _ "embed"

// GuideJSON is guide.json, the user guide also published as docs.
//
//go:embed guide.json
var GuideJSON []byte
