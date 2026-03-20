//go:build embed_smartctl && windows

package collectors

import _ "embed"

//go:embed drivers/smartctl.exe
var smartctlEmbedded []byte
