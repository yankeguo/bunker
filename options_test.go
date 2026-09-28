package bunker

import "testing"

func TestDataDirString(t *testing.T) {
	if got := DataDir("/var/lib/bunker").String(); got != "/var/lib/bunker" {
		t.Fatalf("DataDir = %q", got)
	}
}

func TestDebugFlag(t *testing.T) {
	prev := _debug
	t.Cleanup(func() { _debug = prev })
	_debug = map[string]bool{"ui": true}
	if !Debug("ui") || Debug("db") {
		t.Fatalf("debug = %#v", _debug)
	}
}
