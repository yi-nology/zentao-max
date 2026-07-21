package main

import (
	"context"
	"log"

	"github.com/yi-nology/zentao-max/backend/core/app"
	"github.com/yi-nology/zentao-max/backend/core/event"
	"github.com/yi-nology/zentao-max/backend/core/logger"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v3/pkg/application"
	"go.uber.org/zap"
)

// App 是 Wails v3 的 Service（被注册到 application.Options.Services）
// 同时也是业务层的桥接器：订阅 EventBus，转发到前端
type App struct {
	ctx        context.Context
	wailsApp   *app.WailsApp
	v3App      *application.App
	notifUnsub func()
}

// NewApp 创建 App 实例
func NewApp() *App {
	return &App{}
}

// ServiceName 实现 v3 Service 接口（必须）
// 前端 bindings 会生成到 bindings/<serviceName>/ 目录
func (a *App) ServiceName() string {
	return "desktop"
}

// ServiceStartup 实现 v3 Service 接口（启动时调用）
// 这里启动内部业务（Hertz 后端、EventBus 订阅）
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx
	return a.startBackend(ctx)
}

// setV3App 让 main.go 在 Run 前注入 v3App 引用（用于事件、窗口、托盘调用）
func (a *App) setV3App(v3App *application.App) {
	a.v3App = v3App
}

// startBackend 启动内部 Hertz + EventBus 订阅
func (a *App) startBackend(ctx context.Context) error {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	config := &app.AppConfig{
		Type:          "wails",
		Port:          "",
		AuthDBPath:    "",
		EncryptionKey: "",
	}

	application, err := app.InitializeWailsApp(config)
	if err != nil {
		return err
	}

	wailsApp, ok := application.(*app.WailsApp)
	if !ok {
		log.Println("Expected WailsApp instance")
		return nil
	}
	a.wailsApp = wailsApp

	// 启动内部 Hertz（在 goroutine 里跑，不阻塞 Wails 主循环）
	if err := wailsApp.Start(ctx); err != nil {
		return err
	}

	// 订阅事件总线
	a.subscribeNotifications()

	return nil
}

// ServiceShutdown 实现 v3 Service 接口（关闭时调用）
func (a *App) ServiceShutdown(ctx context.Context) {
	if a.notifUnsub != nil {
		a.notifUnsub()
		a.notifUnsub = nil
	}
}

// ============== 业务初始化（在 main.go events.Startup 中调用） ==============

// shutdown 在应用关闭时清理（被 main.go 的 OnShutdown 调用）
func (a *App) shutdown(ctx context.Context) {
	if a.wailsApp != nil {
		if err := a.wailsApp.Stop(ctx); err != nil {
			log.Printf("Failed to stop application: %v", err)
		}
	}
}

// ============== EventBus → 前端事件桥接 ==============

// subscribeNotifications 订阅定时任务事件，通过 v3 app.Event.Emit 推到前端
// 前端通过 @wailsio/runtime 的 EventsOn 接收
func (a *App) subscribeNotifications() {
	bus := event.GetGlobalBus()
	a.notifUnsub = bus.Subscribe(event.TaskCompleted, func(e event.Event) {
		payload, ok := e.Payload.(map[string]interface{})
		if !ok || a.v3App == nil {
			return
		}
		taskName, _ := payload["taskName"].(string)
		status, _ := payload["status"].(string)

		notifPayload := map[string]interface{}{
			"type":     "task",
			"title":    "定时任务执行完成",
			"body":     "任务「" + taskName + "」状态: " + status,
			"taskId":   payload["taskID"],
			"taskName": taskName,
			"status":   status,
			"bugTotal": payload["bugTotal"],
		}
		a.v3App.Event.Emit("notification", notifPayload)
		logger.Info("notification emitted",
			zap.String("taskName", taskName),
			zap.String("status", status))
	})

	bus.Subscribe(event.TaskFailed, func(e event.Event) {
		payload, _ := e.Payload.(map[string]interface{})
		taskName, _ := payload["taskName"].(string)
		if a.v3App != nil {
			a.v3App.Event.Emit("notification", map[string]interface{}{
				"type":     "task",
				"level":    "error",
				"title":    "定时任务执行失败",
				"body":     "任务「" + taskName + "」执行失败",
				"taskName": taskName,
			})
		}
	})
}

// ============== 桌面原生能力 ==============

// createSystemTray 创建系统托盘（v3 原生支持）
func (a *App) createSystemTray(v3App *application.App) {
	a.v3App = v3App

	trayMenu := v3App.NewMenu()
	trayMenu.Add("显示主窗口").OnClick(func(ctx *application.Context) {
		a.showMainWindow()
	})
	trayMenu.Add("刷新数据").OnClick(func(ctx *application.Context) {
		// 通过事件通知前端重新拉取
		v3App.Event.Emit("app:refresh")
	})
	trayMenu.AddSeparator()
	trayMenu.Add("退出 zentao-max").OnClick(func(ctx *application.Context) {
		v3App.Quit()
	})

	tray := v3App.SystemTray.New()
	tray.SetMenu(trayMenu)
	tray.SetIcon(appIcon)
	tray.SetTooltip("zentao-max - 禅道项目管理助手")
	tray.SetLabel("zentao-max")
	tray.OnClick(func() {
		a.showMainWindow()
	})
}

// showMainWindow 显示主窗口（不存在则忽略）
func (a *App) showMainWindow() {
	if a.v3App == nil {
		return
	}
	w := a.v3App.Window.Current()
	if w == nil {
		return
	}
	w.Show()
	w.Focus()
}

// registerGlobalShortcuts 注册全局快捷键（v3 真正的全局快捷键，窗口无焦点也响应）
// CmdOrCtrl+Shift+Z 切换主窗口可见性
func (a *App) registerGlobalShortcuts(v3App *application.App) {
	if err := v3App.GlobalShortcut.Register("CmdOrCtrl+Shift+Z", func() {
		w := v3App.Window.Current()
		if w == nil {
			return
		}
		if w.IsVisible() {
			w.Hide()
		} else {
			w.Show()
		}
	}); err != nil {
		log.Printf("Failed to register global shortcut: %v", err)
	}
}

// registerAppMenu 注册应用菜单（macOS 顶部菜单）
func (a *App) registerAppMenu(v3App *application.App) {
	menu := v3App.NewMenu()
	menu.Add("显示主窗口").OnClick(func(ctx *application.Context) {
		a.showMainWindow()
	})
	menu.Add("隐藏到托盘").OnClick(func(ctx *application.Context) {
		if w := v3App.Window.Current(); w != nil {
			w.Hide()
		}
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(ctx *application.Context) {
		v3App.Quit()
	})
	v3App.Menu.Set(menu)
}

// ============== 暴露给前端调用的方法（通过 v3 Service 绑定） ==============

// ShowWindow 显示主窗口（前端可调用）
func (a *App) ShowWindow() {
	a.showMainWindow()
}

// HideWindow 隐藏主窗口到托盘
func (a *App) HideWindow() {
	if a.v3App != nil {
		if w := a.v3App.Window.Current(); w != nil {
			w.Hide()
		}
	}
}

// MinimizeToTray 等同于 HideWindow
func (a *App) MinimizeToTray() {
	a.HideWindow()
}

// QuitApp 通过菜单/托盘退出
func (a *App) QuitApp() {
	if a.v3App != nil {
		a.v3App.Quit()
	}
}
