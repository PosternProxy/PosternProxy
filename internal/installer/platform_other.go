//go:build !windows

package installer

import (
	"context"
	"fmt"
	"github.com/posternproxy/posternproxy/internal/agentconfig"
)

func installWindows(context.Context, Options, *agentconfig.Config, string) error {
	return fmt.Errorf("Windows service installation requires Windows")
}
