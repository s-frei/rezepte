//go:build typst && linux && arm64

package typstbin

import _ "embed"

//go:embed typst_linux_arm64
var binary []byte
