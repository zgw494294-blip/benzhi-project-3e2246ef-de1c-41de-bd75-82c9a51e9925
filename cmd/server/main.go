package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	configuration, err := parseConfig(args, os.Getenv)
	if err != nil {
		return err
	}
	if configuration.selfcheck {
		if !isLoopbackAddress(configuration.address) {
			return fmt.Errorf("-selfcheck 只允许使用回环监听地址")
		}
		temporary, err := os.MkdirTemp("", "oral-release-selfcheck-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temporary)
		configuration.dataDir = temporary
	}
	runtime, err := newRuntime(configuration)
	if err != nil {
		return fmt.Errorf("启动服务: %w", err)
	}
	serveResult := runtime.serve()
	runtime.logger.Info("service_started", "address", runtime.listener.Addr().String(), "selfcheck", configuration.selfcheck)
	if configuration.selfcheck {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		checkErr := runSelfcheck(ctx, runtime.listener.Addr().String())
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		shutdownErr := runtime.shutdown(shutdownCtx)
		shutdownCancel()
		serveErr := <-serveResult
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if serveErr != nil {
			return serveErr
		}
		fmt.Println("自检通过：案卷已完成退回修正、复核放行、冻结保护和凭据校验")
		return nil
	}
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serveResult:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-signalContext.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return runtime.shutdown(shutdownCtx)
	}
}
