package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

type anuncioPublico struct {
	Nombre string `json:"nombre"`
	Texto  string `json:"texto,omitempty"`
	Logo   string `json:"logo,omitempty"`
	Enlace string `json:"enlace,omitempty"`
}

// apiAnuncios devuelve los anuncios en vigor de un pueblo, en orden aleatorio, y cuenta una impresión
// para cada uno. Con n > 0 solo devuelve n anuncios, así rotan entre visitas.
func (a *App) apiAnuncios(w http.ResponseWriter, r *http.Request) {
	a.cors(w, r)
	w.Header().Set("Cache-Control", "no-store")

	pueblo := r.URL.Query().Get("pueblo")
	if !slugValido(pueblo) {
		responderJSON(w, http.StatusBadRequest, map[string]string{"error": "pueblo no válido"})
		return
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	n = max(0, min(n, 20))

	hoy := a.hoy()
	lista, err := a.db.ActivosPorPueblo(r.Context(), pueblo, hoy)
	if err != nil {
		log.Printf("anuncios de %s: %v", pueblo, err)
		responderJSON(w, http.StatusInternalServerError, map[string]string{"error": "no disponible"})
		return
	}
	rand.Shuffle(len(lista), func(i, j int) { lista[i], lista[j] = lista[j], lista[i] })
	if n > 0 && len(lista) > n {
		lista = lista[:n]
	}

	if len(lista) > 0 && !esBot(r) {
		ids := make([]int64, len(lista))
		for i, an := range lista {
			ids[i] = an.ID
		}
		if err := a.db.SumarImpresiones(r.Context(), ids, pueblo, hoy); err != nil {
			log.Printf("sumando impresiones en %s: %v", pueblo, err)
		}
	}

	salida := make([]anuncioPublico, 0, len(lista))
	for _, an := range lista {
		p := anuncioPublico{Nombre: an.Nombre, Texto: an.Texto}
		if an.Logo != "" {
			p.Logo = a.cfg.PublicURL + "/logos/" + an.Logo
		}
		if an.URL != "" {
			p.Enlace = fmt.Sprintf("%s/c/%d?p=%s", a.cfg.PublicURL, an.ID, url.QueryEscape(pueblo))
		}
		salida = append(salida, p)
	}
	responderJSON(w, http.StatusOK, map[string]any{"anuncios": salida})
}

// clic cuenta el clic y lleva a la web del anunciante. Si el anuncio ya no existe, vuelve a la ficha del pueblo.
func (a *App) clic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	pueblo := r.URL.Query().Get("p")
	vuelta := a.cfg.SiteURL + "/"
	if slugValido(pueblo) {
		vuelta += pueblo + "/"
	} else {
		pueblo = ""
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Redirect(w, r, vuelta, http.StatusFound)
		return
	}
	hoy := a.hoy()
	destino, cuenta, err := a.db.DestinoClic(r.Context(), id, pueblo, hoy)
	if err != nil || destino == "" {
		if err != nil && !errors.Is(err, ErrNoExiste) {
			log.Printf("clic en el anuncio %d: %v", id, err)
		}
		http.Redirect(w, r, vuelta, http.StatusFound)
		return
	}
	if cuenta && !esBot(r) {
		if err := a.db.SumarClic(r.Context(), id, pueblo, hoy); err != nil {
			log.Printf("sumando clic del anuncio %d: %v", id, err)
		}
	}
	http.Redirect(w, r, destino, http.StatusFound)
}

func (a *App) logo(w http.ResponseWriter, r *http.Request) {
	nombre := r.PathValue("archivo")
	if !archivoLogoValido.MatchString(nombre) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	http.ServeFile(w, r, filepath.Join(a.logosDir, nombre))
}

func (a *App) cors(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Origin")
	origen := r.Header.Get("Origin")
	if origen == "" {
		return
	}
	for _, permitido := range a.cfg.AllowedOrigins {
		if permitido == "*" || permitido == origen {
			w.Header().Set("Access-Control-Allow-Origin", origen)
			return
		}
	}
}

// esBot evita contar las visitas de buscadores y previsualizaciones de enlaces.
func esBot(r *http.Request) bool {
	agente := strings.ToLower(r.UserAgent())
	if agente == "" {
		return true
	}
	for _, marca := range []string{"bot", "crawl", "spider", "slurp", "facebookexternalhit", "whatsapp", "preview", "headless"} {
		if strings.Contains(agente, marca) {
			return true
		}
	}
	return false
}

func responderJSON(w http.ResponseWriter, estado int, datos any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(estado)
	if err := json.NewEncoder(w).Encode(datos); err != nil {
		log.Printf("escribiendo JSON: %v", err)
	}
}
