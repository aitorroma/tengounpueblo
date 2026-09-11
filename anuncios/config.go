package main

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"
)

type Config struct {
	Addr           string
	DataDir        string
	SiteURL        string   // web estática, de donde se leen los pueblos
	PublicURL      string   // dirección pública de este servicio (logos y enlaces de clic)
	AllowedOrigins []string // webs que pueden pedir anuncios desde el navegador
	AdminUser      string
	AdminPassword  string
	TrustProxy     bool // detrás de un proxy: la IP del cliente sale de X-Forwarded-For
}

func cargarConfig() (Config, error) {
	c := Config{
		Addr:          entorno("ADDR", ":8080"),
		DataDir:       entorno("DATA_DIR", "./data"),
		SiteURL:       strings.TrimRight(entorno("SITE_URL", "https://tengounpueblo.com"), "/"),
		PublicURL:     strings.TrimRight(entorno("PUBLIC_URL", "http://localhost:8080"), "/"),
		AdminUser:     entorno("ADMIN_USER", "admin"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		TrustProxy:    os.Getenv("TRUST_PROXY") == "1",
	}
	for _, origen := range strings.Split(entorno("ALLOWED_ORIGINS", c.SiteURL), ",") {
		if origen = strings.TrimRight(strings.TrimSpace(origen), "/"); origen != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, origen)
		}
	}
	if utf8.RuneCountInString(c.AdminPassword) < 12 {
		return c, errors.New("ADMIN_PASSWORD es obligatoria y debe tener al menos 12 caracteres")
	}
	for nombre, valor := range map[string]string{"SITE_URL": c.SiteURL, "PUBLIC_URL": c.PublicURL} {
		if u, err := url.Parse(valor); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return c, errors.New(nombre + " debe ser una URL completa, por ejemplo https://tengounpueblo.com")
		}
	}
	return c, nil
}

func entorno(clave, defecto string) string {
	if valor := strings.TrimSpace(os.Getenv(clave)); valor != "" {
		return valor
	}
	return defecto
}
