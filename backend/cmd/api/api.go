package api

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/umohsamuel/elcompresso/internal/adapter"
	"github.com/umohsamuel/elcompresso/internal/port/http/handler"
	"github.com/umohsamuel/elcompresso/internal/service"
	"github.com/umohsamuel/elcompresso/pkg/env"
	"github.com/umohsamuel/elcompresso/pkg/response"
)

type Server struct {
	Service     *service.Services
	Engine      *gin.Engine
	Environment *env.EnvironmentVariables
}

func API(services *service.Services, environment *env.EnvironmentVariables) *Server {

	r := &Server{
		Service:     services,
		Engine:      gin.Default(),
		Environment: environment,
	}

	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"POST", "GET", "PUT", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "Accept", "User-Agent", "Cache-Control", "Pragma"}
	config.ExposeHeaders = []string{"Content-Length"}
	config.AllowCredentials = true
	config.MaxAge = 12 * time.Hour

	r.Engine.Use(cors.New(config))

	r.Engine.Static("/downloads", "tmp")

	r.health()

	r.Engine.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	api := r.Engine.Group("/api/v1")

	{
		r.fileCompressRoutes(api)
	}

	return r
}

func (server *Server) health() {
	server.Engine.GET("/health", func(c *gin.Context) {
		response.NewSuccessResponse("server up!!!", nil, nil).Send(c)
	})
}

func (server *Server) fileCompressRoutes(rg *gin.RouterGroup) {
	server.Engine.MaxMultipartMemory = 500 << 20

	h := handler.NewCompressHandler(struct {
		Env     env.EnvironmentVariables
		Adapter adapter.Adapters
	}{
		Env:     *server.Environment,
		Adapter: *server.Service.Adapter,
	},
	)

	fcRoute := rg.Group("/file-compress")

	{
		fcRoute.POST("/video", h.CompressVideo)
		fcRoute.POST("/audio", h.CompressAudio)
		fcRoute.POST("/image", h.CompressImage)
	}
}
