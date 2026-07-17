package main

import (
	"log"
	"runtime"

	"github.com/mewisme/vutils/internal/app"
	"github.com/mewisme/vutils/internal/config"
	"github.com/mewisme/vutils/internal/ui"
)

func main() {
	runtime.LockOSThread()

	path := config.ResolvePath()
	svc, err := app.NewService(path)
	if err != nil {
		log.Fatal(err)
	}
	defer svc.Shutdown()

	if err := ui.Run(svc); err != nil {
		log.Fatal(err)
	}
}
