package main

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const tamMaxLogo = 2 << 20

var (
	archivoLogoValido = regexp.MustCompile(`^[a-z2-7]{26}\.(png|jpg|webp|gif)$`)
	extensionesLogo   = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif"}
	mensajes          = map[string]string{"guardado": "Anuncio guardado.", "borrado": "Anuncio borrado."}
)

type filaListado struct {
	Anuncio
	Estado         string
	NombresPueblos []string
	CaducaPronto   bool
}

type resumenListado struct {
	Activos       int
	CaducanPronto int
	Impresiones   int64
	Clics         int64
}

func (a *App) listado(w http.ResponseWriter, r *http.Request) {
	anuncios, err := a.db.Listar(r.Context())
	if err != nil {
		a.errorInterno(w, "listando anuncios", err)
		return
	}
	pueblos, errPueblos := a.pueblos.Lista(r.Context())
	nombres := nombresPorSlug(pueblos)
	hoy := a.hoy()
	en30Dias := a.ahora().In(madrid).AddDate(0, 0, 30).Format(time.DateOnly)

	var resumen resumenListado
	filas := make([]filaListado, 0, len(anuncios))
	for _, an := range anuncios {
		f := filaListado{Anuncio: an, Estado: an.EstadoEn(hoy)}
		for _, slug := range an.Pueblos {
			f.NombresPueblos = append(f.NombresPueblos, cmp.Or(nombres[slug], slug))
		}
		if f.Estado == "Activo" {
			resumen.Activos++
			if an.Fin <= en30Dias {
				f.CaducaPronto = true
				resumen.CaducanPronto++
			}
		}
		resumen.Impresiones += an.Impresiones
		resumen.Clics += an.Clics
		filas = append(filas, f)
	}
	a.render(w, http.StatusOK, "listado.html", map[string]any{
		"Filas":        filas,
		"Resumen":      resumen,
		"AvisoPueblos": errPueblos != nil,
		"Mensaje":      mensajes[r.URL.Query().Get("ok")],
	})
}

func (a *App) nuevo(w http.ResponseWriter, r *http.Request) {
	hoy := a.ahora().In(madrid)
	an := Anuncio{Activo: true, Inicio: hoy.Format(time.DateOnly), Fin: hoy.AddDate(1, 0, 0).Format(time.DateOnly)}
	if pueblo := r.URL.Query().Get("pueblo"); slugValido(pueblo) {
		an.Pueblos = []string{pueblo}
	}
	a.formulario(w, r, http.StatusOK, an, nil)
}

func (a *App) editar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	an, err := a.db.Obtener(r.Context(), id)
	if errors.Is(err, ErrNoExiste) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.errorInterno(w, "leyendo anuncio", err)
		return
	}
	a.formulario(w, r, http.StatusOK, an, nil)
}

func (a *App) formulario(w http.ResponseWriter, r *http.Request, estado int, an Anuncio, errores []string) {
	pueblos, errPueblos := a.pueblos.Lista(r.Context())
	nombres := nombresPorSlug(pueblos)
	// Los pueblos ya asociados se muestran aunque la web no los liste (o no responda).
	pueblos = slices.Clone(pueblos)
	for _, slug := range an.Pueblos {
		if _, existe := nombres[slug]; !existe {
			pueblos = append(pueblos, Pueblo{Slug: slug, Nombre: slug})
		}
	}
	datos := map[string]any{
		"Anuncio":      an,
		"Pueblos":      pueblos,
		"Nombres":      nombres,
		"Errores":      errores,
		"AvisoPueblos": errPueblos != nil,
	}
	if an.ID != 0 {
		desde := a.ahora().In(madrid).AddDate(0, 0, -29).Format(time.DateOnly)
		porPueblo, porDia, err := a.db.Estadisticas(r.Context(), an.ID, desde)
		if err != nil {
			log.Printf("estadísticas del anuncio %d: %v", an.ID, err)
		}
		datos["PorPueblo"], datos["PorDia"] = porPueblo, porDia
	}
	a.render(w, estado, "formulario.html", datos)
}

func (a *App) crear(w http.ResponseWriter, r *http.Request) {
	a.guardar(w, r, Anuncio{})
}

func (a *App) actualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	actual, err := a.db.Obtener(r.Context(), id)
	if errors.Is(err, ErrNoExiste) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.errorInterno(w, "leyendo anuncio", err)
		return
	}
	a.guardar(w, r, actual)
}

func (a *App) guardar(w http.ResponseWriter, r *http.Request, actual Anuncio) {
	pueblos, _ := a.pueblos.Lista(r.Context())
	validos := make(map[string]bool)
	for _, p := range pueblos {
		validos[p.Slug] = true
	}
	for _, slug := range actual.Pueblos {
		validos[slug] = true
	}

	an, errores := leerFormulario(r, validos)
	an.ID, an.Logo, an.Impresiones, an.Clics = actual.ID, actual.Logo, actual.Impresiones, actual.Clics
	if len(errores) > 0 {
		a.formulario(w, r, http.StatusUnprocessableEntity, an, errores)
		return
	}

	nuevoLogo, err := a.guardarLogo(r)
	if err != nil {
		a.formulario(w, r, http.StatusUnprocessableEntity, an, []string{err.Error()})
		return
	}
	switch {
	case nuevoLogo != "":
		an.Logo = nuevoLogo
	case r.PostFormValue("quitar_logo") == "1":
		an.Logo = ""
	}

	if err := a.db.Guardar(r.Context(), &an); err != nil {
		if nuevoLogo != "" {
			a.borrarLogo(nuevoLogo)
		}
		if errors.Is(err, ErrNoExiste) {
			http.NotFound(w, r)
			return
		}
		a.errorInterno(w, "guardando anuncio", err)
		return
	}
	if actual.Logo != "" && actual.Logo != an.Logo {
		a.borrarLogo(actual.Logo)
	}
	http.Redirect(w, r, "/admin/?ok=guardado", http.StatusSeeOther)
}

func (a *App) borrar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDeRuta(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	logo, err := a.db.Borrar(r.Context(), id)
	if errors.Is(err, ErrNoExiste) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.errorInterno(w, "borrando anuncio", err)
		return
	}
	if logo != "" {
		a.borrarLogo(logo)
	}
	http.Redirect(w, r, "/admin/?ok=borrado", http.StatusSeeOther)
}

func leerFormulario(r *http.Request, pueblosValidos map[string]bool) (Anuncio, []string) {
	campo := func(nombre string) string { return strings.TrimSpace(r.PostFormValue(nombre)) }
	an := Anuncio{
		Nombre: campo("nombre"),
		Texto:  campo("texto"),
		URL:    campo("url"),
		Inicio: campo("inicio"),
		Fin:    campo("fin"),
		Notas:  campo("notas"),
		Activo: r.PostFormValue("activo") == "1",
	}
	var errores []string

	switch n := utf8.RuneCountInString(an.Nombre); {
	case n == 0:
		errores = append(errores, "El nombre del anunciante es obligatorio.")
	case n > 80:
		errores = append(errores, "El nombre no puede pasar de 80 caracteres.")
	}
	if utf8.RuneCountInString(an.Texto) > 120 {
		errores = append(errores, "La frase no puede pasar de 120 caracteres.")
	}
	if utf8.RuneCountInString(an.Notas) > 2000 {
		errores = append(errores, "Las notas no pueden pasar de 2000 caracteres.")
	}

	if an.URL != "" {
		if !strings.Contains(an.URL, "://") {
			an.URL = "https://" + an.URL
		}
		u, err := url.Parse(an.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(an.URL) > 500 {
			errores = append(errores, "El enlace debe ser una dirección web, por ejemplo https://www.instagram.com/tunegocio")
		}
	}

	inicio, errInicio := time.Parse(time.DateOnly, an.Inicio)
	fin, errFin := time.Parse(time.DateOnly, an.Fin)
	if errInicio != nil || errFin != nil {
		errores = append(errores, "Las fechas de inicio y fin son obligatorias.")
	} else if fin.Before(inicio) {
		errores = append(errores, "La fecha de fin no puede ser anterior a la de inicio.")
	}

	vistos := make(map[string]bool)
	for _, slug := range r.PostForm["pueblos"] {
		if pueblosValidos[slug] && !vistos[slug] {
			vistos[slug] = true
			an.Pueblos = append(an.Pueblos, slug)
		}
	}
	sort.Strings(an.Pueblos)
	if len(an.Pueblos) == 0 {
		errores = append(errores, "Elige al menos un pueblo.")
	}
	return an, errores
}

// guardarLogo guarda la imagen subida (si la hay) con un nombre aleatorio y devuelve ese nombre.
// Solo acepta PNG, JPG, WebP y GIF según su contenido real, nunca SVG.
func (a *App) guardarLogo(r *http.Request) (string, error) {
	archivo, cabecera, err := r.FormFile("logo")
	if errors.Is(err, http.ErrMissingFile) || errors.Is(err, http.ErrNotMultipart) {
		return "", nil
	}
	if err != nil {
		return "", errors.New("No se pudo leer el logo.")
	}
	defer archivo.Close()
	if cabecera.Size == 0 {
		return "", nil
	}
	if cabecera.Size > tamMaxLogo {
		return "", errors.New("El logo no puede pasar de 2 MB.")
	}

	inicio := make([]byte, 512)
	n, _ := io.ReadFull(archivo, inicio)
	extension, permitida := extensionesLogo[http.DetectContentType(inicio[:n])]
	if !permitida {
		return "", errors.New("El logo tiene que ser una imagen PNG, JPG, WebP o GIF.")
	}

	nombre := strings.ToLower(rand.Text()) + extension
	destino, err := os.OpenFile(filepath.Join(a.logosDir, nombre), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		log.Printf("creando logo: %v", err)
		return "", errors.New("No se pudo guardar el logo.")
	}
	_, errCopia := io.Copy(destino, io.MultiReader(bytes.NewReader(inicio[:n]), io.LimitReader(archivo, tamMaxLogo)))
	errCierre := destino.Close()
	if errCopia != nil || errCierre != nil {
		os.Remove(destino.Name())
		log.Printf("guardando logo: %v %v", errCopia, errCierre)
		return "", errors.New("No se pudo guardar el logo.")
	}
	return nombre, nil
}

func (a *App) borrarLogo(nombre string) {
	if !archivoLogoValido.MatchString(nombre) {
		return
	}
	if err := os.Remove(filepath.Join(a.logosDir, nombre)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("borrando logo %s: %v", nombre, err)
	}
}

func idDeRuta(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}
