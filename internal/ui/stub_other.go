//go:build !windows

package ui

import (
	"fmt"

	"github.com/mewisme/vutils/internal/app"
)

// Run is only supported on Windows.
func Run(svc *app.Service) error {
	_ = svc
	return fmt.Errorf("native UI requires Windows")
}
