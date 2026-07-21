package main

import (
	"embed"
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// 应用菜单图标（嵌入到二进制）
//
//go:embed build/appicon.png
var appIcon []byte

func main() {
	app := NewApp()

	// 创建 Wails v3 应用
	wailsApp := application.New(application.Options{
		Name:        "zentao-max",
		Description: "禅道项目管理桌面助手",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.murphyyi.zentao-max",
		},
		Logger:   application.DefaultLogger(slog.LevelInfo),
		LogLevel: slog.LevelInfo,
	})
	// 关闭时的清理（替代 v2 OnShutdown）
	wailsApp.OnShutdown(func() {
		app.shutdown(wailsApp.Context())
	})

	// 创建主窗口（v3 通过 WindowManager 创建）
	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "zentao-max",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 30,
		},
	})

	// 创建系统托盘（v3 原生支持，v2 需第三方库）
	app.createSystemTray(wailsApp)

	// 注册全局快捷键 CmdOrCtrl+Shift+Z 唤起主窗口
	app.registerGlobalShortcuts(wailsApp)

	// 注册应用菜单
	app.registerAppMenu(wailsApp)

	// ServiceStartup 会在 app.Run() 启动时被调用，初始化业务（Hertz、EventBus）
	// 但因为 Hertz 启动较重，放到 wailsApp 启动后的第一个 tick（这里直接在 Run 前 set ctx）
	app.setV3App(wailsApp)

	// 运行（阻塞）
	if err := wailsApp.Run(); err != nil {
		log.Fatalf("Wails run failed: %v", err)
	}
}

