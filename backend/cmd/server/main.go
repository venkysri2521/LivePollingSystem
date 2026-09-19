// Command server wires the whole backend together: config, Mongo, Redis,
// middleware and routes. Everything below is composition - the behaviour lives
// in internal/.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/venkat/livepolls/backend/internal/config"
	"github.com/venkat/livepolls/backend/internal/db"
	"github.com/venkat/livepolls/backend/internal/handlers"
	"github.com/venkat/livepolls/backend/internal/middleware"
	"github.com/venkat/livepolls/backend/internal/realtime"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	mongoDB, err := db.ConnectMongo(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer mongoDB.Close(ctx)
	log.Println("startup: mongo connected")

	rdb, err := db.ConnectRedis(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer rdb.Close()
	log.Println("startup: redis connected")

	rt := realtime.New(rdb)
	auth := middleware.NewAuth(cfg.JWTSecret)

	router := buildRouter(cfg, mongoDB, rt, auth)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: SSE connections are long-lived by design and a
		// write deadline would cut every viewer off mid-poll.
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		log.Printf("startup: listening on :%s (%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown so in-flight votes finish instead of being dropped
	// when the platform redeploys.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown: draining connections")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("shutdown: done")
}

func buildRouter(cfg *config.Config, mongoDB *db.Mongo, rt *realtime.Service, auth *middleware.Auth) *gin.Engine {
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery(), middleware.SecurityHeaders(), middleware.CORS(cfg.CORSOrigins), middleware.BodyLimit(16*1024))
	if !cfg.IsProd() {
		r.Use(gin.Logger())
	}

	authHandler := &handlers.AuthHandler{Mongo: mongoDB, Auth: auth}
	pollHandler := &handlers.PollHandler{Mongo: mongoDB, RT: rt, Cfg: cfg}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

	api := r.Group("/api")
	{
		a := api.Group("/auth")
		a.POST("/signup", middleware.RateLimit(rt, "signup", 10, time.Hour), authHandler.Signup)
		a.POST("/login", middleware.RateLimit(rt, "login", 20, 15*time.Minute), authHandler.Login)
		a.GET("/me", auth.Required(), authHandler.Me)

		p := api.Group("/polls")
		// Reads are public; only the poll's slug is needed to watch it.
		p.GET("/:slug", auth.Optional(), pollHandler.Get)
		p.GET("/:slug/results", pollHandler.Results)
		p.GET("/:slug/stream", pollHandler.Stream)
		p.POST("/:slug/vote", auth.Optional(),
			middleware.RateLimit(rt, "vote", 30, time.Minute), pollHandler.Vote)

		// Everything that creates or changes a poll needs a real account.
		p.POST("", auth.Required(), middleware.RateLimit(rt, "create", 30, time.Hour), pollHandler.Create)
		p.GET("", auth.Required(), pollHandler.Mine)
		p.PATCH("/:slug/status", auth.Required(), pollHandler.SetStatus)
		p.DELETE("/:slug", auth.Required(), pollHandler.Delete)
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such endpoint"})
	})
	return r
}
