package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/application"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/httpapi"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/redaction"
	"benzhi-project-3e2246ef-de1c-41de-bd75-82c9a51e9925/internal/store"
)

type runtime struct {
	repository *store.Repository
	server     *http.Server
	listener   net.Listener
	logger     *slog.Logger
}

func newRuntime(configuration config) (*runtime, error) {
	repository, err := store.Open(configuration.dataDir)
	if err != nil {
		return nil, err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	service := application.NewService(repository, redaction.New())
	handler := httpapi.New(service, logger)
	listener, err := net.Listen("tcp", configuration.address)
	if err != nil {
		_ = repository.Close()
		return nil, err
	}
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32 << 10,
	}
	return &runtime{repository: repository, server: server, listener: listener, logger: logger}, nil
}

func (r *runtime) serve() <-chan error {
	result := make(chan error, 1)
	go func() {
		err := r.server.Serve(r.listener)
		if err == http.ErrServerClosed {
			err = nil
		}
		result <- err
	}()
	return result
}

func (r *runtime) shutdown(ctx context.Context) error {
	serverErr := r.server.Shutdown(ctx)
	storeErr := r.repository.Close()
	if serverErr != nil {
		return serverErr
	}
	return storeErr
}
