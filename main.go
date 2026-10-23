package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"infradesktop/internal/adapters/presentation"
	"infradesktop/internal/composition"
	"os"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed packaging/linux/assets/infradesktop-512.png
var icon []byte

func main() {
	app := composition.Desktop()
	var cancel context.CancelFunc
	err := wails.Run(&options.App{Title: "InfraDesktop", Width: 1440, Height: 860, MinWidth: 720, MinHeight: 600,
		Linux:       &linux.Options{Icon: icon},
		AssetServer: &assetserver.Options{Assets: assets}, Bind: []interface{}{app},
		OnStartup: func(ctx context.Context) {
			var child context.Context
			child, cancel = context.WithCancel(ctx)
			presentation.AttachContext(app, child)
		},
		OnDomReady: func(ctx context.Context) {
			screens, e := runtime.ScreenGetAll(ctx)
			if e != nil {
				return
			}
			for _, screen := range screens {
				if screen.IsCurrent {
					runtime.WindowSetSize(ctx, min(1440, max(720, screen.Size.Width-32)), min(860, max(600, screen.Size.Height-100)))
					runtime.WindowCenter(ctx)
					break
				}
			}
		},
		OnBeforeClose: func(context.Context) bool { return presentation.PreventClose(app) },
		OnShutdown: func(context.Context) {
			if cancel != nil {
				cancel()
			}
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "desktop initialization failed")
		os.Exit(1)
	}
}
