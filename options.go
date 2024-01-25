package bunker

import (
	"os"
	"strings"
)

type DataDir string

func (d DataDir) String() string {
	return string(d)
}

var _debug map[string]bool

func init() {
	_debug = make(map[string]bool)
	for _, v := range strings.Split(os.Getenv("DEBUG"), ",") {
		_debug[strings.TrimSpace(v)] = true
	}
}

func Debug(name string) bool {
	return _debug[name]
}
