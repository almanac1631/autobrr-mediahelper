package main

import (
	"context"
	"flag"
	"github.com/almanac1631/autobrr-mediahelper/internal/app/autobrrmediahelper"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	dbName              = flag.String("db", "./media.db", "database file name")
	mediaScrapeInterval = flag.Duration("scrape-interval", 24*time.Hour, "interval to scrape popular media")
	webserverPort       = flag.Int("port", 8053, "port to run webserver on")
	authorizationValue  = flag.String("auth", "", "authorization value for \"Authorization\" header")
)

func main() {
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	if *authorizationValue == "" {
		slog.Error("authorization value must be provided")
		flag.Usage()
		os.Exit(1)
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	config := &autobrrmediahelper.Config{
		DBName:              *dbName,
		WebserverPort:       *webserverPort,
		AuthorizationValue:  []byte(*authorizationValue),
		MediaScrapeInterval: *mediaScrapeInterval,
	}
	autobrrmediahelper.StartApp(config, ctx)
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
	cancelFunc()
}
