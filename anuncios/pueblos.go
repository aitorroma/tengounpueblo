package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var patronSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// slugValido acepta los nombres de las fichas de la web, como "alfarras" o "villanueva-del-rio".
func slugValido(s string) bool {
	return len(s) <= 100 && patronSlug.MatchString(s)
}

type Pueblo struct {
	Slug      string
	Nombre    string
	Provincia string
}

// Pueblos lee la lista de pueblos que publica la web en pueblos.json, para no darlos de alta dos veces.
type Pueblos struct {
	url     string
	cliente *http.Client
	mu      sync.Mutex
	lista   []Pueblo
	leida   time.Time
}

func nuevosPueblos(direccion string) *Pueblos {
	return &Pueblos{url: direccion, cliente: &http.Client{Timeout: 10 * time.Second}}
}

// Lista devuelve los pueblos con 5 minutos de caché. Si la web no responde,
// devuelve la última lista buena (si la hay) junto con el error.
func (p *Pueblos) Lista(ctx context.Context) ([]Pueblo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lista != nil && time.Since(p.leida) < 5*time.Minute {
		return p.lista, nil
	}
	lista, err := p.descargar(ctx)
	if err != nil {
		return p.lista, err
	}
	p.lista, p.leida = lista, time.Now()
	return lista, nil
}

func (p *Pueblos) descargar(ctx context.Context) ([]Pueblo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.cliente.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer %s: %w", p.url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s respondió %s", p.url, resp.Status)
	}
	var crudos []struct {
		Nombre    string `json:"nombre"`
		Provincia string `json:"provincia"`
		URL       string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 5<<20)).Decode(&crudos); err != nil {
		return nil, fmt.Errorf("pueblos.json no es válido: %w", err)
	}
	lista := make([]Pueblo, 0, len(crudos))
	for _, c := range crudos {
		if slug := path.Base(strings.Trim(c.URL, "/")); slugValido(slug) {
			lista = append(lista, Pueblo{Slug: slug, Nombre: c.Nombre, Provincia: c.Provincia})
		}
	}
	sort.Slice(lista, func(i, j int) bool { return strings.ToLower(lista[i].Nombre) < strings.ToLower(lista[j].Nombre) })
	return lista, nil
}

func nombresPorSlug(lista []Pueblo) map[string]string {
	nombres := make(map[string]string, len(lista))
	for _, p := range lista {
		nombres[p.Slug] = p.Nombre
	}
	return nombres
}
