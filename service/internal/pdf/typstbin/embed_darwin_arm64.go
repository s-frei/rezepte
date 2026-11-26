//go:build typst && darwin && arm64

package typstbin

import _ "embed"

//go:embed typst_darwin_arm64
var binary []byte
