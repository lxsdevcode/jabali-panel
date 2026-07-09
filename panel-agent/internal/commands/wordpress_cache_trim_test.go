package commands

import "testing"

// JAB-59: the panel calls wordpress.cache_trim to enforce the Redis budget;
// that command must actually be registered (it was a no-op before).
func TestCacheTrimRegistered(t *testing.T) {
	found := false
	for _, c := range Default.Commands() {
		if c == "wordpress.cache_trim" {
			found = true
			break
		}
	}
	if !found {
		t.Error("wordpress.cache_trim not registered in Default")
	}
}
