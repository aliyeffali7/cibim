// Command backend is Cibim's own API for the mobile app (App API Contract v0.1).
// It reads bank data from CBAR's Open Banking gateway (the mock in the demo),
// categorises it and computes everything the screens show.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// loadState uses PostgreSQL when DATABASE_URL is set, otherwise the JSON file.
// On the first start with an empty database, it imports the JSON file if one exists.
func loadState(dbURL, file string) (*State, error) {
	if dbURL == "" {
		log.Printf("storage: JSON file %s", file)
		return LoadState(file)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := OpenPostgres(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	s, err := LoadStateDB(ctx, db)
	if err != nil {
		return nil, err
	}
	if len(s.Users) == 0 {
		if old, err := LoadState(file); err == nil && len(old.Users) > 0 {
			s.Users, s.FX = old.Users, old.FX
			if err := s.saveDB(); err != nil {
				return nil, fmt.Errorf("importing %s: %w", file, err)
			}
			log.Printf("storage: imported %d users from %s", len(s.Users), file)
		}
	}
	log.Printf("storage: PostgreSQL (%d users loaded)", len(s.Users))
	return s, nil
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /health on the local server and exit (used by Docker)")
	flag.Parse()
	port := env("PORT", "8080")

	if *healthcheck {
		resp, err := http.Get("http://127.0.0.1:" + port + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	state, err := loadState(env("DATABASE_URL", ""), env("DATA_FILE", "data/state.json"))
	if err != nil {
		log.Fatal(err)
	}
	stale, err := time.ParseDuration(env("SYNC_STALE_AFTER", "10m"))
	if err != nil {
		log.Fatal("SYNC_STALE_AFTER: ", err)
	}
	app := &App{
		state:       state,
		cbar:        NewCBARClient(env("MOCK_CBAR_URL", "http://localhost:4000")),
		now:         time.Now,
		redirectURL: env("APP_REDIRECT_URL", "cibim://connections/{id}"),
		staleAfter:  stale,
	}
	srv := &http.Server{Addr: ":" + port, Handler: app.routes(), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		log.Printf("cibim backend on :%s, bank gateway %s", port, app.cbar.baseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
