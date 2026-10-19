// Command mock-cbar imitates CBAR's Open Banking gateway for the Cibim demo.
// See README.md and the Mock CBAR API Contract.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /health on the local server and exit (used by Docker)")
	flag.Parse()

	port := os.Getenv("PORT")
	if port == "" {
		port = "4000"
	}

	if *healthcheck {
		resp, err := http.Get("http://127.0.0.1:" + port + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	srv := &Server{
		store:         NewStore(time.Now),
		enforceFreq:   os.Getenv("MOCK_ENFORCE_FREQUENCY") == "true",
		publicBaseURL: os.Getenv("PUBLIC_BASE_URL"),
	}
	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("mock CBAR listening on :%s (frequency limit enforced: %v)", port, srv.enforceFreq)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}
