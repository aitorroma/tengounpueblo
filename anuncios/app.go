package main

import (
	"bytes"
	"crypto/rand"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

//go:embed plantillas/*.html estaticos/*
var archivos embed.FS

// madrid es la zona horaria con la que se decide qué anuncios están en vigor.
var madrid = func() *time.Location {
	zona, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		return time.UTC
	}
	return zona
}()

type App struct {
	cfg       Config
	db        *Store
	pueblos   *Pueblos
	paginas   map[string]*template.Template
	csrf      string
	logosDir  string
	ahora     func() time.Time
	limitador *Limitador
}

func nuevaApp(cfg Config) (*App, error) {
	logosDir := filepath.Join(cfg.DataDir, "logos")
	if err := os.MkdirAll(logosDir, 0o750); err != nil {
		return nil, fmt.Errorf("no se pudo crear %s: %w", logosDir, err)
	}
	db, err := abrirStore(filepath.Join(cfg.DataDir, "anuncios.db"))
	if err != nil {
		return nil, err
	}
	paginas, err := cargarPlantillas()
	if err != nil {
		db.Close()
		return nil, err
	}
	return &App{
		cfg:       cfg,
		db:        db,
		pueblos:   nuevosPueblos(cfg.SiteURL + "/pueblos.json"),
		paginas:   paginas,
		csrf:      rand.Text(),
		logosDir:  logosDir,
		ahora:     time.Now,
		limitador: nuevoLimitador(),
	}, nil
}

func (a *App) hoy() string {
	return a.ahora().In(madrid).Format(time.DateOnly)
}

func (a *App) rutas() http.Handler {
	mux := http.NewServeMux()

	// Público: lo usa la web desde el navegador de cada visitante.
	mux.HandleFunc("GET /api/anuncios", a.apiAnuncios)
	mux.HandleFunc("GET /c/{id}", a.clic)
	mux.HandleFunc("GET /logos/{archivo}", a.logo)
	mux.HandleFunc("GET /salud", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// Panel de gestión, con usuario y contraseña.
	estaticos, _ := fs.Sub(archivos, "estaticos")
	mux.Handle("GET /admin/estaticos/", a.admin(http.StripPrefix("/admin/estaticos/", http.FileServerFS(estaticos)).ServeHTTP))
	mux.Handle("GET /admin/{$}", a.admin(a.listado))
	mux.Handle("GET /admin/anuncios/nuevo", a.admin(a.nuevo))
	mux.Handle("POST /admin/anuncios", a.admin(a.crear))
	mux.Handle("GET /admin/anuncios/{id}", a.admin(a.editar))
	mux.Handle("POST /admin/anuncios/{id}", a.admin(a.actualizar))
	mux.Handle("POST /admin/anuncios/{id}/borrar", a.admin(a.borrar))
	irAlPanel := func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/admin/", http.StatusFound) }
	mux.HandleFunc("GET /admin", irAlPanel)
	mux.HandleFunc("GET /{$}", irAlPanel)

	return cabecerasComunes(mux)
}

func cabecerasComunes(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.ServeHTTP(w, r)
	})
}

func cargarPlantillas() (map[string]*template.Template, error) {
	funciones := template.FuncMap{
		"contiene":   func(lista []string, valor string) bool { return slices.Contains(lista, valor) },
		"ctr":        ctr,
		"miles":      miles,
		"fecha":      fecha,
		"inicial":    inicial,
		"minusculas": strings.ToLower,
	}
	base, err := template.New("base.html").Funcs(funciones).ParseFS(archivos, "plantillas/base.html")
	if err != nil {
		return nil, err
	}
	paginas := make(map[string]*template.Template)
	for _, nombre := range []string{"listado.html", "formulario.html"} {
		copia, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if paginas[nombre], err = copia.ParseFS(archivos, "plantillas/"+nombre); err != nil {
			return nil, err
		}
	}
	return paginas, nil
}

func (a *App) render(w http.ResponseWriter, estado int, pagina string, datos map[string]any) {
	datos["CSRF"] = a.csrf
	var buf bytes.Buffer
	if err := a.paginas[pagina].ExecuteTemplate(&buf, "base.html", datos); err != nil {
		a.errorInterno(w, "pintando "+pagina, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(estado)
	buf.WriteTo(w)
}

func (a *App) errorInterno(w http.ResponseWriter, contexto string, err error) {
	log.Printf("%s: %v", contexto, err)
	http.Error(w, "Error interno. Revisa los registros del servicio.", http.StatusInternalServerError)
}

// ctr es el porcentaje de clics sobre impresiones, con coma decimal.
func ctr(impresiones, clics int64) string {
	if impresiones == 0 {
		return "—"
	}
	porcentaje := strconv.FormatFloat(float64(clics)*100/float64(impresiones), 'f', 1, 64)
	return strings.Replace(porcentaje, ".", ",", 1) + " %"
}

// miles formatea 12345 como "12.345".
func miles(n int64) string {
	s := strconv.FormatInt(n, 10)
	signo := ""
	if n < 0 {
		signo, s = "-", s[1:]
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return signo + s
}

// fecha pasa "2026-09-11" a "11/09/2026".
func fecha(dia string) string {
	t, err := time.Parse(time.DateOnly, dia)
	if err != nil {
		return dia
	}
	return t.Format("02/01/2006")
}

func inicial(texto string) string {
	r, _ := utf8.DecodeRuneInString(texto)
	if r == utf8.RuneError {
		return "?"
	}
	return string(unicode.ToUpper(r))
}
