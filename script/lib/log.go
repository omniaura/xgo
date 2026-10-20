package lib

import (
	"fmt"
	"os"
)

func Logf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[cleanup] "+format+"\n", args...)
}
