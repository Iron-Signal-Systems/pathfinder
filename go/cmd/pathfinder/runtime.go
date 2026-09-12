package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func runServe(args []string) error {
	path, err := configPath("serve", args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/livez",
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("live\n"))
		},
	)
	mux.HandleFunc(
		"/readyz",
		func(writer http.ResponseWriter, _ *http.Request) {
			if err := checkDatabase(cfg); err != nil {
				http.Error(
					writer,
					"not ready",
					http.StatusServiceUnavailable,
				)
				return
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("ready\n"))
		},
	)
	server := &http.Server{
		Addr:              cfg.ReadinessListen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverResult := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serverResult <- err
	}()
	log.Printf(
		"Pathfinder %s starting; readiness=%s",
		version,
		cfg.ReadinessListen,
	)
	signals := make(chan os.Signal, 1)
	signal.Notify(
		signals,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	select {
	case received := <-signals:
		log.Printf("signal received: %s", received)
	case err := <-serverResult:
		if err != nil {
			return fmt.Errorf(
				"health server: %w",
				err,
			)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf(
			"health shutdown: %w",
			err,
		)
	}
	log.Printf("shutdown complete")
	return nil
}

func runValidate(args []string) error {
	path, err := configPath("validate", args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	fmt.Println("Pathfinder validation: PASS")
	fmt.Printf(
		"  database=%s:%d/%s\n",
		cfg.DatabaseHost,
		cfg.DatabasePort,
		cfg.DatabaseName,
	)
	fmt.Printf(
		"  database_user=%s\n",
		cfg.DatabaseUser,
	)
	fmt.Printf(
		"  readiness=%s\n",
		cfg.ReadinessListen,
	)
	return nil
}
