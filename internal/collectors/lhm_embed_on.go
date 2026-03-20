//go:build embed_lhm && windows

package collectors

import _ "embed"

//go:embed drivers/lhm_bridge.exe
var lhmEmbedded []byte
