package tars

import (
	"fmt"
	"os"

	"github.com/MacgradyHuang/TarsGo/tars/util/debug"
)

// CheckPanic used to dump stack info to file when catch panic
func CheckPanic() {
	if r := recover(); r != nil {
		var msg string
		if err, ok := r.(error); ok {
			msg = err.Error()
		} else {
			msg = fmt.Sprintf("%#v", r)
		}
		debug.DumpStack(true, "panic", msg)
		os.Exit(-1)
	}
}
