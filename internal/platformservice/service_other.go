//go:build !windows

package platformservice

import (
	"context"
	"fmt"
)

func Dispatch(args []string, name string, run func(context.Context) error) (bool, error) {
	if len(args) > 0 && (args[0] == "service" || args[0] == "uninstall") {
		return true, fmt.Errorf("this service operation is supported on Windows only")
	}
	return false, nil
}
