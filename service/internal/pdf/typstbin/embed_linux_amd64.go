//go:build typst && linux && amd64

package typstbin

import _ "embed"

//go:embed typst_linux_amd64
var binary []byte
