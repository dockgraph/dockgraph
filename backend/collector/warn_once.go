package collector

import (
	"log"
	"sync"
)

// warned holds the keys of the warnings already logged.
var warned sync.Map

// warnOnce logs a warning the first time key is seen in this process. For
// conditions detected on every poll that would otherwise flood the log.
func warnOnce(key, format string, args ...any) {
	if _, seen := warned.LoadOrStore(key, struct{}{}); !seen {
		log.Printf(format, args...)
	}
}
