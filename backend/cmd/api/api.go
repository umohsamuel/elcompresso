package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/umohsamuel/elcompresso/internal/port/http/handler"
	"github.com/umohsamuel/elcompresso/internal/port/http/middleware"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
)

type Server struct {
	Service     *service.Services
	Engine      *gin.Engine
	Environment *env.EnvironmentVariables
}

func API(services *service.Services, environment *env.EnvironmentVariables) *Server {
	if environment.ProductionEnvironment {
		gin.SetMode(gin.ReleaseMode)
	}

	r := &Server{
		Service:     services,
		Engine:      gin.New(),
		Environment: environment,
	}

	r.Engine.Use(gin.Recovery(), requestLogger())
	if err := r.Engine.SetTrustedProxies(environment.TrustedProxies); err != nil {
		slog.Error("invalid TRUSTED_PROXIES", "err", err)
	}
	r.Engine.Use(cors.New(corsConfig(environment.AllowedOrigins)))

	meta := handler.NewMetaHandler(environment, services)
	r.Engine.GET("/health", meta.Health)
	r.Engine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	api := r.Engine.Group("/api/v1")
	api.GET("/capabilities", meta.Capabilities)
	r.jobRoutes(api)

	return r
}

func (server *Server) jobRoutes(rg *gin.RouterGroup) {
	h := handler.NewJobHandler(server.Environment, server.Service)
	limiter := middleware.NewRateLimiter(server.Environment.RateLimitPerMinute)

	jobs := rg.Group("/jobs")
	jobs.POST("/:type", limiter.Middleware(), h.Create)
	jobs.GET("", h.List)
	jobs.GET("/:id", h.Get)
	jobs.DELETE("/:id", h.Cancel)
	jobs.GET("/:id/download", h.Download)
}

func corsConfig(origins []string) cors.Config {
	allowAll := false
	for _, o := range origins {
		allowAll = allowAll || o == "*"
	}
	return cors.Config{
		AllowOriginFunc: func(origin string) bool {
			if allowAll {
				return true
			}
			for _, pattern := range origins {
				if matchOrigin(pattern, origin) {
					return true
				}
			}
			return false
		},
		AllowMethods:  []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders: []string{"Content-Length", "Content-Disposition", "Retry-After"},
		MaxAge:        12 * time.Hour,
	}
}

// matchOrigin supports exact origins and a single "*" wildcard such as
// "https://*.vercel.app" (handy for Vercel preview deployments).
func matchOrigin(pattern, origin string) bool {
	prefix, suffix, wildcard := strings.Cut(pattern, "*")
	if !wildcard {
		return pattern == origin
	}
	return len(origin) > len(prefix)+len(suffix) &&
		strings.HasPrefix(origin, prefix) && strings.HasSuffix(origin, suffix) &&
		!strings.ContainsAny(origin[len(prefix):len(origin)-len(suffix)], "/:")
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.Request.URL.Path
		if path == "/health" || path == "/ping" || (c.Request.Method == http.MethodGet && c.Writer.Status() < 400 && strings.HasPrefix(path, "/api/v1/jobs")) {
			return // status polling is too chatty to log
		}
		slog.Info("request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).Round(time.Millisecond),
			"ip", c.ClientIP(),
		)
	}
}
