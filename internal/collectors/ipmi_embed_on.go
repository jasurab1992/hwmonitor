//go:build embed_ipmiutil && windows

package collectors

import "embed"

//go:embed drivers/ipmiutil/ipmiutil.exe drivers/ipmiutil/ipmiutillib.dll drivers/ipmiutil/libeay32.dll drivers/ipmiutil/ssleay32.dll drivers/ipmiutil/showselmsg.dll
var ipmiutilFS embed.FS
