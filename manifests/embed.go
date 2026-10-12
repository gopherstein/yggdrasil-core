package manifests

import _ "embed"

//go:embed models/catalog.json
var CatalogJSON []byte

//go:embed models/presets.json
var PresetsJSON []byte

// MediaJSON lists the picture and clip models setup offers (#551).
//
//go:embed models/media.json
var MediaJSON []byte
