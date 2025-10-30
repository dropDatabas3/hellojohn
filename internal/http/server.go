package http

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// durFromEnv lee una duración (Go time.ParseDuration) de la variable de entorno key.
// Si no existe o falla el parseo, devuelve def.
func durFromEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// intFromEnv lee un entero de la variable de entorno key.
// Si no existe o falla el parseo, devuelve def.
func intFromEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// Start levanta el servidor HTTP con timeouts razonables y graceful shutdown.
// Timeouts por defecto (overridable por env):
//
//	HTTP_READ_HEADER_TIMEOUT = 5s
//	HTTP_READ_TIMEOUT        = 15s
//	HTTP_WRITE_TIMEOUT       = 15s
//	HTTP_IDLE_TIMEOUT        = 60s
//	HTTP_SHUTDOWN_TIMEOUT    = 10s
//	HTTP_MAX_HEADER_BYTES    = 1048576  (1 MiB)
func Start(addr string, handler http.Handler) error {
	readHeaderTimeout := durFromEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	readTimeout := durFromEnv("HTTP_READ_TIMEOUT", 15*time.Second)
	writeTimeout := durFromEnv("HTTP_WRITE_TIMEOUT", 15*time.Second)
	idleTimeout := durFromEnv("HTTP_IDLE_TIMEOUT", 60*time.Second)
	shutdownTimeout := durFromEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second)
	maxHeaderBytes := intFromEnv("HTTP_MAX_HEADER_BYTES", 1<<20) // 1 MiB

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ErrorLog:          log.New(os.Stderr, "http: ", 0),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	errCh := make(chan error, 1)
	go func() {
		// Nota: ListenAndServe bloquea; lo disparamos en goroutine para poder
		// escuchar SIGINT/SIGTERM y hacer shutdown ordenado.
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	// Esperamos señales de terminación para bajar limpio.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		// Si falla al arrancar o se cerró sin señal externa.
		return err

	case sig := <-sigCh:
		log.Printf("http: shutdown solicitado por señal: %v", sig)
		// Damos un deadline para cerrar conexiones activas.
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			// Si el shutdown con grace falla, forzamos el cierre.
			log.Printf("http: shutdown con errores: %v; forzando Close()", err)
			_ = srv.Close()
			return err
		}
		log.Printf("http: shutdown completo")
		return nil
	}
}

// StartBackground inicia un servidor HTTP en background sin manejar señales.
// Devuelve el *http.Server para permitir Shutdown/Close desde el llamador.
// Útil para levantar puertos auxiliares (p.ej., UI estática) mientras Start()
// maneja el ciclo de vida principal.
func StartBackground(addr string, handler http.Handler) (*http.Server, error) {
	readHeaderTimeout := durFromEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	readTimeout := durFromEnv("HTTP_READ_TIMEOUT", 15*time.Second)
	writeTimeout := durFromEnv("HTTP_WRITE_TIMEOUT", 15*time.Second)
	idleTimeout := durFromEnv("HTTP_IDLE_TIMEOUT", 60*time.Second)
	maxHeaderBytes := intFromEnv("HTTP_MAX_HEADER_BYTES", 1<<20)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ErrorLog:          log.New(os.Stderr, "http(ui): ", 0),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http(ui): listen error: %v", err)
		}
	}()
	return srv, nil
}
