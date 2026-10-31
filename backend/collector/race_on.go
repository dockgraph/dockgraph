//go:build race

package collector

// raceBuild reports whether the binary is built with the race detector, whose
// runtime maps more memory at start than a compose parse is allowed.
const raceBuild = true
