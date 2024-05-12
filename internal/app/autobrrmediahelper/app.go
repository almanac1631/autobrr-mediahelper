package autobrrmediahelper

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func StartApp(config *Config, ctx context.Context) {
	mediaManager := NewMediaManager(config)
	if err := mediaManager.Init(); err != nil {
		slog.Error("failed to initialize media manager", "error", err)
		os.Exit(1)
	}
	go runMediaScrapes(mediaManager, ctx)
	go runWebhookWebserver(mediaManager, ctx)
}

func runWebhookWebserver(manager *MediaManager, ctx context.Context) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(gin.Recovery(), authHandler(manager.config))
	router.POST("/media-check", mediaCheck(manager))
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", manager.config.WebserverPort),
		Handler: router,
	}
	go func() {
		slog.Info("starting webserver", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("failed to run webserver", "error", err)
		}
	}()
	<-ctx.Done()
	if err := srv.Shutdown(context.Background()); err != nil {
		slog.Error("failed to shutdown webserver", "error", err)
	}
}

func authHandler(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") != string(config.AuthorizationValue) {
			c.Status(401)
			c.Abort()
		}
	}
}

func runMediaScrapes(manager *MediaManager, ctx context.Context) {
	ticker := time.NewTicker(manager.config.MediaScrapeInterval)
	refreshMedia := func() {
		if err := manager.RefreshPopularMedia(); err != nil {
			slog.Error("failed to refresh popular media", "error", err)
			os.Exit(1)
		}
	}
	refreshMedia()
	slog.Info("starting media scrape loop", "interval", manager.config.MediaScrapeInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshMedia()
		}
	}
}
