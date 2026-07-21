package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/yi-nology/zentao-max/backend/core/app"
	"github.com/yi-nology/zentao-max/backend/core/config"
	"github.com/yi-nology/zentao-max/backend/core/logger"
	"github.com/yi-nology/zentao-max/backend/core/metrics"
)

var (
	encryptionKey  = ""
	zentaoServer   = ""
	zentaoAccount  = ""
	zentaoPassword = ""
	authDBPath     = "./data/auth.db"
)

//go:embed static/*
var embeddedStaticFS embed.FS

func getFileSystem() http.FileSystem {
	staticFS, err := fs.Sub(embeddedStaticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FS(staticFS)
}

func main() {
	if err := logger.Init(&config.LogConfig{
		Level:            "info",
		Format:           "console",
		OutputPath:       "",
		EnableCaller:     true,
		EnableStacktrace: false,
	}); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	if err := metrics.Init(); err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}

	appConfig := &app.AppConfig{
		Type:           "http",
		Port:           os.Getenv("PORT"),
		AuthDBPath:     authDBPath,
		EncryptionKey:  encryptionKey,
		ZentaoServer:   zentaoServer,
		ZentaoAccount:  zentaoAccount,
		ZentaoPassword: zentaoPassword,
		StaticPath:     "embedded",
		StaticFS:       getFileSystem(),
	}

	application, err := app.InitializeHTTPApp(appConfig)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	httpApp, ok := application.(*app.HTTPApp)
	if !ok {
		log.Fatalf("Expected HTTPApp, got %T", application)
	}

	if err := httpApp.Start(context.Background()); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	log.Printf("Serving frontend from embedded static file system")

	select {}
}
