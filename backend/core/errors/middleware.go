package errors

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
)

func CORSMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")

		origin := string(c.GetHeader("Origin"))

		if allowedOrigins == "" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			origins := strings.Split(allowedOrigins, ",")
			allowed := false
			for _, allowedOrigin := range origins {
				allowedOrigin = strings.TrimSpace(allowedOrigin)
				if origin == allowedOrigin {
					allowed = true
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}

			if !allowed && origin != "" {
				log.Printf("[CORS] Blocked request from unauthorized origin: %s", origin)
			}
		}

		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Token")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if string(c.Request.Method()) == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next(ctx)
	}
}
