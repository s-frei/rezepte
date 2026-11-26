//go:build typst && darwin && amd64

package typstbin

import _ "embed"

//go:embed typst_darwin_amd64
var binary []byte
