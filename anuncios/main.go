// Servicio de anuncios de tengounpueblo.com: panel para crear anuncios, asociarlos
// a pueblos y API pública que la web consulta en cada visita.
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
	_ "time/tzdata" // zona horaria de Madrid también en contenedores sin tzdata
)

func main() {
	cfg, err := cargarConfig()
	if err != nil {
		log.Fatal(err)
	}
	app, err := nuevaApp(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer app.db.Close()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.rutas(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		log.Printf("Servicio de anuncios escuchando en %s · panel: %s/admin/", cfg.Addr, cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	parar := make(chan os.Signal, 1)
	signal.Notify(parar, os.Interrupt, syscall.SIGTERM)
	<-parar
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("cerrando el servidor: %v", err)
	}
}
