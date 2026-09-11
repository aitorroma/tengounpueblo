package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const tamMaxFormulario = 3 << 20 // logo de 2 MB más los campos

// admin protege el panel con usuario y contraseña, bloquea tras varios intentos fallidos
// y exige el token CSRF en los envíos de formularios.
func (a *App) admin(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := a.ipCliente(r)
		if a.limitador.Bloqueado(ip) {
			http.Error(w, "Demasiados intentos fallidos. Vuelve a probar dentro de unos minutos.", http.StatusTooManyRequests)
			return
		}
		usuario, clave, enviada := r.BasicAuth()
		if !enviada || !iguales(usuario, a.cfg.AdminUser) || !iguales(clave, a.cfg.AdminPassword) {
			if enviada {
				a.limitador.Fallo(ip)
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="Anuncios de tengounpueblo", charset="UTF-8"`)
			http.Error(w, "Acceso restringido", http.StatusUnauthorized)
			return
		}
		a.limitador.Exito(ip)

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self'; style-src 'self'; script-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")

		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, tamMaxFormulario)
			if err := r.ParseMultipartForm(tamMaxFormulario); err != nil && !errors.Is(err, http.ErrNotMultipart) {
				http.Error(w, "El formulario no es válido o es demasiado grande (el logo puede ocupar como máximo 2 MB).", http.StatusBadRequest)
				return
			}
			defer func() {
				if r.MultipartForm != nil {
					r.MultipartForm.RemoveAll()
				}
			}()
			if !iguales(r.PostFormValue("csrf"), a.csrf) || !a.mismoOrigen(r) {
				http.Error(w, "El formulario ha caducado. Recarga la página e inténtalo de nuevo.", http.StatusForbidden)
				return
			}
		}
		h(w, r)
	})
}

// iguales compara en tiempo constante, sin revelar la longitud.
func iguales(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

func (a *App) mismoOrigen(r *http.Request) bool {
	origen := r.Header.Get("Origin")
	if origen == "" {
		return true // algunos navegadores no lo envían; el token CSRF sigue siendo obligatorio
	}
	u, err := url.Parse(origen)
	if err != nil {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	publica, err := url.Parse(a.cfg.PublicURL)
	return err == nil && u.Host == publica.Host
}

func (a *App) ipCliente(r *http.Request) string {
	if a.cfg.TrustProxy {
		if cadena := r.Header.Get("X-Forwarded-For"); cadena != "" {
			saltos := strings.Split(cadena, ",")
			return strings.TrimSpace(saltos[len(saltos)-1]) // la añade el proxy más cercano
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

const (
	maxFallos       = 10
	tiempoBloqueo   = 15 * time.Minute
	maxIPsVigiladas = 10000
)

// Limitador bloquea durante un rato una IP que falla la contraseña muchas veces seguidas.
type Limitador struct {
	mu     sync.Mutex
	fallos map[string]*intentos
}

type intentos struct {
	n      int
	ultimo time.Time
	hasta  time.Time
}

func nuevoLimitador() *Limitador {
	return &Limitador{fallos: make(map[string]*intentos)}
}

func (l *Limitador) Bloqueado(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	i := l.fallos[ip]
	return i != nil && time.Now().Before(i.hasta)
}

func (l *Limitador) Fallo(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ahora := time.Now()
	if len(l.fallos) >= maxIPsVigiladas {
		for clave, i := range l.fallos {
			if ahora.Sub(i.ultimo) > tiempoBloqueo {
				delete(l.fallos, clave)
			}
		}
	}
	i := l.fallos[ip]
	if i == nil {
		i = &intentos{}
		l.fallos[ip] = i
	}
	if ahora.Sub(i.ultimo) > tiempoBloqueo {
		i.n = 0
	}
	i.n++
	i.ultimo = ahora
	if i.n >= maxFallos {
		i.n = 0
		i.hasta = ahora.Add(tiempoBloqueo)
	}
}

func (l *Limitador) Exito(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fallos, ip)
}
