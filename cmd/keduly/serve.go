package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Lewin671/keduly/internal/core"
	"github.com/Lewin671/keduly/internal/dav"
	"github.com/Lewin671/keduly/internal/httpapi"
	"github.com/Lewin671/keduly/internal/store"
	"github.com/Lewin671/keduly/internal/webui"
)

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func serve(args []string) int {
	fs := flag.NewFlagSet("keduly serve", flag.ContinueOnError)
	addr := fs.String("addr", envOr("KEDULY_ADDR", "127.0.0.1:8080"), "address to listen on (env KEDULY_ADDR)")
	data := fs.String("data", envOr("KEDULY_DATA", "./data"), "directory that holds keduly.db (env KEDULY_DATA)")
	registration := fs.String("registration", envOr("KEDULY_REGISTRATION", "open"), "open or closed (env KEDULY_REGISTRATION)")
	baseURL := fs.String("base-url", os.Getenv("KEDULY_BASE_URL"), "public URL; https marks cookies Secure (env KEDULY_BASE_URL)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *registration != "open" && *registration != "closed" {
		fmt.Fprintln(os.Stderr, "keduly: registration must be open or closed")
		return 2
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db, err := store.Open(*data)
	if err != nil {
		logger.Error("cannot open the database", "error", err.Error())
		return 1
	}
	defer db.Close()

	svc := core.New(db)
	svc.Registration, svc.Version = *registration, version
	server := &http.Server{
		Addr: *addr,
		Handler: httpapi.New(httpapi.Config{Service: svc, BaseURL: *baseURL, DAV: dav.New(svc),
			Static: webui.Handler(), Logger: logger}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()
	logger.Info("listening", "addr", *addr, "version", version, "registration", *registration)

	select {
	case err := <-failed:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err.Error())
			return 1
		}
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			logger.Error("shutdown", "error", err.Error())
			return 1
		}
	}
	return 0
}
