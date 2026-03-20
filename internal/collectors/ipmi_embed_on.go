//go:build embed_ipmitool && windows

package collectors

import _ "embed"

//go:embed drivers/ipmitool.exe
var ipmitoolEmbedded []byte
