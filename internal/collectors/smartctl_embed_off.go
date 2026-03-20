//go:build !embed_smartctl

package collectors

// smartctlEmbedded is nil when built without the embed_smartctl tag.
// The binary will then search for smartctl.exe in PATH and common locations.
var smartctlEmbedded []byte
