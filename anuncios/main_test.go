package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	navegador   = "Mozilla/5.0 (X11; Linux x86_64; rv:154.0) Gecko/20100101 Firefox/154.0"
	claveAdmin  = "clave-de-prueba-larga"
	origenWeb   = "https://tengounpueblo.com"
	origenPanel = "https://anuncios.example"
)

func appPrueba(t *testing.T) (*App, http.Handler) {
	t.Helper()
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pueblos.json" {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, `[{"nombre":"Alfarràs","provincia":"Lleida","url":"/alfarras/","lat":41.8},{"nombre":"Almenar","provincia":"Lleida","url":"/almenar/"}]`)
	}))
	t.Cleanup(web.Close)
	app, err := nuevaApp(Config{
		DataDir:        t.TempDir(),
		SiteURL:        web.URL,
		PublicURL:      origenPanel,
		AllowedOrigins: []string{origenWeb},
		AdminUser:      "aitor",
		AdminPassword:  claveAdmin,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.db.Close() })
	app.ahora = func() time.Time { return time.Date(2026, 9, 11, 12, 0, 0, 0, madrid) }
	return app, app.rutas()
}

func crear(t *testing.T, app *App, an Anuncio) Anuncio {
	t.Helper()
	if err := app.db.Guardar(t.Context(), &an); err != nil {
		t.Fatal(err)
	}
	return an
}

func peticion(metodo, ruta string, cuerpo io.Reader) *http.Request {
	req := httptest.NewRequest(metodo, ruta, cuerpo)
	req.Header.Set("User-Agent", navegador)
	return req
}

func conClave(req *http.Request) *http.Request {
	req.SetBasicAuth("aitor", claveAdmin)
	return req
}

func pedir(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func pedirAPI(t *testing.T, h http.Handler, consulta string) []anuncioPublico {
	t.Helper()
	req := peticion("GET", "/api/anuncios?"+consulta, nil)
	req.Header.Set("Origin", origenWeb)
	rec := pedir(h, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("API %s: código %d: %s", consulta, rec.Code, rec.Body.String())
	}
	var cuerpo struct {
		Anuncios []anuncioPublico `json:"anuncios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cuerpo); err != nil {
		t.Fatal(err)
	}
	return cuerpo.Anuncios
}

func totales(t *testing.T, app *App, id int64) (impresiones, clics int64) {
	t.Helper()
	an, err := app.db.Obtener(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return an.Impresiones, an.Clics
}

var patronCSRF = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

func tokenCSRF(t *testing.T, html string) string {
	t.Helper()
	m := patronCSRF.FindStringSubmatch(html)
	if m == nil {
		t.Fatal("no se encontró el token CSRF en la página")
	}
	return m[1]
}

func formularioMultipart(t *testing.T, campos url.Values, logo []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for clave, valores := range campos {
		for _, v := range valores {
			mw.WriteField(clave, v)
		}
	}
	if logo != nil {
		parte, err := mw.CreateFormFile("logo", "logo.png")
		if err != nil {
			t.Fatal(err)
		}
		parte.Write(logo)
	}
	mw.Close()
	return &buf, mw.FormDataContentType()
}

func pngPrueba(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		for y := range 8 {
			img.Set(x, y, color.RGBA{198, 105, 59, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func camposValidos(token string) url.Values {
	return url.Values{
		"csrf": {token}, "nombre": {"Forn Cal Josep"}, "inicio": {"2026-09-11"}, "fin": {"2027-09-10"},
		"activo": {"1"}, "pueblos": {"alfarras"},
	}
}

func TestAPISoloDevuelveAnunciosEnVigorDelPueblo(t *testing.T) {
	app, h := appPrueba(t)
	bar := crear(t, app, Anuncio{Nombre: "Bar de la Plaza", URL: "https://bar.example", Inicio: "2026-09-01", Fin: "2027-08-31", Activo: true, Pueblos: []string{"alfarras"}})
	crear(t, app, Anuncio{Nombre: "Caducado", Inicio: "2025-01-01", Fin: "2026-09-10", Activo: true, Pueblos: []string{"alfarras"}})
	crear(t, app, Anuncio{Nombre: "Programado", Inicio: "2026-09-12", Fin: "2027-09-12", Activo: true, Pueblos: []string{"alfarras"}})
	crear(t, app, Anuncio{Nombre: "Pausado", Inicio: "2026-09-01", Fin: "2027-09-01", Activo: false, Pueblos: []string{"alfarras"}})
	crear(t, app, Anuncio{Nombre: "Otro pueblo", Inicio: "2026-09-01", Fin: "2027-09-01", Activo: true, Pueblos: []string{"almenar"}})
	crear(t, app, Anuncio{Nombre: "Último día", Inicio: "2025-09-11", Fin: "2026-09-11", Activo: true, Pueblos: []string{"alfarras", "almenar"}})

	anuncios := pedirAPI(t, h, "pueblo=alfarras")
	var nombres []string
	for _, an := range anuncios {
		nombres = append(nombres, an.Nombre)
		switch an.Nombre {
		case "Bar de la Plaza":
			if an.Enlace != origenPanel+"/c/"+strconv.FormatInt(bar.ID, 10)+"?p=alfarras" {
				t.Errorf("enlace de clic inesperado: %q", an.Enlace)
			}
		case "Último día":
			if an.Enlace != "" {
				t.Errorf("un anuncio sin web no debería llevar enlace: %q", an.Enlace)
			}
		}
	}
	sort.Strings(nombres)
	if got := strings.Join(nombres, "|"); got != "Bar de la Plaza|Último día" {
		t.Fatalf("anuncios devueltos: %s", got)
	}
	if impresiones, _ := totales(t, app, bar.ID); impresiones != 1 {
		t.Errorf("impresiones = %d, se esperaba 1", impresiones)
	}
}

func TestAPICabecerasYValidacion(t *testing.T) {
	_, h := appPrueba(t)

	req := peticion("GET", "/api/anuncios?pueblo=alfarras", nil)
	req.Header.Set("Origin", origenWeb)
	rec := pedir(h, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != origenWeb || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("cabeceras para la web: %v", rec.Header())
	}

	req = peticion("GET", "/api/anuncios?pueblo=alfarras", nil)
	req.Header.Set("Origin", "https://malo.example")
	if rec := pedir(h, req); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("no debería permitir otros orígenes")
	}

	for _, consulta := range []string{"", "pueblo=", "pueblo=../etc", "pueblo=Alfarras", "pueblo=a%20b"} {
		if rec := pedir(h, peticion("GET", "/api/anuncios?"+consulta, nil)); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: código %d, se esperaba 400", consulta, rec.Code)
		}
	}
}

func TestAPIRotaYLimitaElNumeroDeAnuncios(t *testing.T) {
	app, h := appPrueba(t)
	for _, letra := range []string{"A", "B", "C", "D"} {
		crear(t, app, Anuncio{Nombre: "Anunciante " + letra, Inicio: "2026-01-01", Fin: "2026-12-31", Activo: true, Pueblos: []string{"alfarras"}})
	}
	if todos := pedirAPI(t, h, "pueblo=alfarras"); len(todos) != 4 {
		t.Fatalf("sin límite deberían salir los 4, salen %d", len(todos))
	}
	vistos := make(map[string]int)
	for range 40 {
		uno := pedirAPI(t, h, "pueblo=alfarras&n=1")
		if len(uno) != 1 {
			t.Fatalf("con n=1 salen %d anuncios", len(uno))
		}
		vistos[uno[0].Nombre]++
	}
	if len(vistos) < 3 {
		t.Errorf("con n=1 debería rotar entre anunciantes, solo han salido: %v", vistos)
	}
}

func TestAPINoCuentaBots(t *testing.T) {
	app, h := appPrueba(t)
	an := crear(t, app, Anuncio{Nombre: "Bar", Inicio: "2026-01-01", Fin: "2026-12-31", Activo: true, Pueblos: []string{"alfarras"}})
	req := peticion("GET", "/api/anuncios?pueblo=alfarras", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	pedir(h, req)
	if impresiones, _ := totales(t, app, an.ID); impresiones != 0 {
		t.Errorf("un bot no debería contar impresiones, hay %d", impresiones)
	}
}

func TestClicCuentaYRedirige(t *testing.T) {
	app, h := appPrueba(t)
	an := crear(t, app, Anuncio{Nombre: "Bar", URL: "https://bar.example/carta", Inicio: "2026-01-01", Fin: "2026-12-31", Activo: true, Pueblos: []string{"alfarras"}})
	id := strconv.FormatInt(an.ID, 10)

	rec := pedir(h, peticion("GET", "/c/"+id+"?p=alfarras", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://bar.example/carta" {
		t.Fatalf("clic: %d → %s", rec.Code, rec.Header().Get("Location"))
	}
	if _, clics := totales(t, app, an.ID); clics != 1 {
		t.Errorf("clics = %d, se esperaba 1", clics)
	}

	// En un pueblo al que no está asociado lleva igualmente al anunciante, pero no cuenta.
	pedir(h, peticion("GET", "/c/"+id+"?p=almenar", nil))
	if _, clics := totales(t, app, an.ID); clics != 1 {
		t.Errorf("un clic desde otro pueblo no debería contar (clics = %d)", clics)
	}

	if rec := pedir(h, peticion("GET", "/c/9999?p=alfarras", nil)); rec.Header().Get("Location") != app.cfg.SiteURL+"/alfarras/" {
		t.Errorf("anuncio inexistente debería volver a la ficha, va a %s", rec.Header().Get("Location"))
	}
	if rec := pedir(h, peticion("GET", "/c/abc?p=%3Cscript%3E", nil)); rec.Header().Get("Location") != app.cfg.SiteURL+"/" {
		t.Errorf("id y pueblo no válidos deberían volver a la portada, va a %s", rec.Header().Get("Location"))
	}
}

func TestAdminPideContrasenaYBloqueaIntentos(t *testing.T) {
	_, h := appPrueba(t)
	rec := pedir(h, peticion("GET", "/admin/", nil))
	if rec.Code != http.StatusUnauthorized || !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Basic") {
		t.Fatalf("sin contraseña: %d", rec.Code)
	}
	rec = pedir(h, conClave(peticion("GET", "/admin/", nil)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Todavía no hay anuncios") {
		t.Fatalf("con contraseña: %d", rec.Code)
	}
	for range maxFallos {
		req := peticion("GET", "/admin/", nil)
		req.SetBasicAuth("aitor", "mala")
		pedir(h, req)
	}
	if rec := pedir(h, conClave(peticion("GET", "/admin/", nil))); rec.Code != http.StatusTooManyRequests {
		t.Errorf("tras %d fallos debería bloquear, código %d", maxFallos, rec.Code)
	}
}

func TestAdminCreaAnuncioConLogo(t *testing.T) {
	app, h := appPrueba(t)
	rec := pedir(h, conClave(peticion("GET", "/admin/anuncios/nuevo?pueblo=alfarras", nil)))
	if rec.Code != http.StatusOK {
		t.Fatalf("formulario nuevo: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `value="alfarras" checked`) {
		t.Error("el pueblo de la URL debería venir marcado")
	}
	campos := camposValidos(tokenCSRF(t, rec.Body.String()))
	campos.Set("texto", "Coca de recapte")
	campos.Set("url", "www.forncaljosep.example")
	campos["pueblos"] = []string{"alfarras", "almenar", "alfarras", "inventado"}
	cuerpo, tipo := formularioMultipart(t, campos, pngPrueba(t))

	req := conClave(peticion("POST", "/admin/anuncios", cuerpo))
	req.Header.Set("Content-Type", tipo)
	req.Header.Set("Origin", origenPanel)
	rec = pedir(h, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/?ok=guardado" {
		t.Fatalf("guardar: %d %s", rec.Code, rec.Body.String())
	}

	lista, err := app.db.Listar(t.Context())
	if err != nil || len(lista) != 1 {
		t.Fatalf("anuncios guardados: %d (%v)", len(lista), err)
	}
	an := lista[0]
	if an.URL != "https://www.forncaljosep.example" {
		t.Errorf("la web sin https:// debería completarse, queda %q", an.URL)
	}
	if got := strings.Join(an.Pueblos, ","); got != "alfarras,almenar" {
		t.Errorf("pueblos = %s (sin repetidos ni inventados)", got)
	}
	if !archivoLogoValido.MatchString(an.Logo) {
		t.Fatalf("nombre de logo no válido: %q", an.Logo)
	}

	rec = pedir(h, peticion("GET", "/logos/"+an.Logo, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("logo: %d %v", rec.Code, rec.Header())
	}
	if anuncios := pedirAPI(t, h, "pueblo=almenar"); len(anuncios) != 1 || anuncios[0].Logo != origenPanel+"/logos/"+an.Logo {
		t.Errorf("la API debería devolver el logo: %+v", anuncios)
	}
	listado := pedir(h, conClave(peticion("GET", "/admin/", nil))).Body.String()
	if !strings.Contains(listado, "Forn Cal Josep") || !strings.Contains(listado, "Alfarràs, Almenar") {
		t.Error("el listado debería mostrar el anuncio con los nombres de sus pueblos")
	}
}

func TestAdminValidaElFormulario(t *testing.T) {
	app, h := appPrueba(t)
	token := tokenCSRF(t, pedir(h, conClave(peticion("GET", "/admin/anuncios/nuevo", nil))).Body.String())

	cuerpo, tipo := formularioMultipart(t, url.Values{
		"csrf": {token}, "nombre": {""}, "url": {"javascript:alert(1)"}, "inicio": {"2026-09-11"}, "fin": {"2026-01-01"},
	}, nil)
	req := conClave(peticion("POST", "/admin/anuncios", cuerpo))
	req.Header.Set("Content-Type", tipo)
	rec := pedir(h, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("código %d, se esperaba 422", rec.Code)
	}
	for _, mensaje := range []string{"El nombre del anunciante es obligatorio", "El enlace debe ser una dirección web", "La fecha de fin no puede ser anterior", "Elige al menos un pueblo"} {
		if !strings.Contains(rec.Body.String(), mensaje) {
			t.Errorf("falta el mensaje %q", mensaje)
		}
	}

	cuerpo, tipo = formularioMultipart(t, camposValidos(token), []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	req = conClave(peticion("POST", "/admin/anuncios", cuerpo))
	req.Header.Set("Content-Type", tipo)
	rec = pedir(h, req)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "PNG, JPG, WebP o GIF") {
		t.Errorf("un SVG debería rechazarse: %d", rec.Code)
	}

	if lista, _ := app.db.Listar(t.Context()); len(lista) != 0 {
		t.Errorf("no debería haberse guardado nada, hay %d anuncios", len(lista))
	}
	if archivos, _ := os.ReadDir(app.logosDir); len(archivos) != 0 {
		t.Errorf("no debería haber logos guardados, hay %d", len(archivos))
	}
}

func TestAdminExigeTokenYMismoOrigen(t *testing.T) {
	app, h := appPrueba(t)
	token := tokenCSRF(t, pedir(h, conClave(peticion("GET", "/admin/anuncios/nuevo", nil))).Body.String())

	sinToken := camposValidos(token)
	sinToken.Del("csrf")
	for nombre, prueba := range map[string]struct {
		campos url.Values
		origen string
	}{
		"sin token":     {sinToken, ""},
		"token erróneo": {url.Values{"csrf": {"otro"}, "nombre": {"X"}}, ""},
		"otro origen":   {camposValidos(token), "https://malo.example"},
	} {
		req := conClave(peticion("POST", "/admin/anuncios", strings.NewReader(prueba.campos.Encode())))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if prueba.origen != "" {
			req.Header.Set("Origin", prueba.origen)
		}
		if rec := pedir(h, req); rec.Code != http.StatusForbidden {
			t.Errorf("%s: código %d, se esperaba 403", nombre, rec.Code)
		}
	}
	if lista, _ := app.db.Listar(t.Context()); len(lista) != 0 {
		t.Errorf("no debería haberse guardado nada, hay %d anuncios", len(lista))
	}
}

func TestAdminEditaYBorra(t *testing.T) {
	app, h := appPrueba(t)
	an := crear(t, app, Anuncio{Nombre: "Bar", URL: "https://bar.example", Inicio: "2026-01-01", Fin: "2026-12-31", Activo: true, Pueblos: []string{"alfarras"}})
	id := strconv.FormatInt(an.ID, 10)
	pedirAPI(t, h, "pueblo=alfarras")
	pedir(h, peticion("GET", "/c/"+id+"?p=alfarras", nil))

	rec := pedir(h, conClave(peticion("GET", "/admin/anuncios/"+id, nil)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Por pueblo") || !strings.Contains(rec.Body.String(), "100,0 %") {
		t.Fatalf("página de edición: %d", rec.Code)
	}
	token := tokenCSRF(t, rec.Body.String())

	cambios := url.Values{"csrf": {token}, "nombre": {"Bar de la Plaza"}, "url": {"https://bar.example"},
		"inicio": {"2026-01-01"}, "fin": {"2026-12-31"}, "pueblos": {"almenar"}}
	req := conClave(peticion("POST", "/admin/anuncios/"+id, strings.NewReader(cambios.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if rec := pedir(h, req); rec.Code != http.StatusSeeOther {
		t.Fatalf("editar: %d %s", rec.Code, rec.Body.String())
	}
	editado, err := app.db.Obtener(t.Context(), an.ID)
	if err != nil {
		t.Fatal(err)
	}
	if editado.Nombre != "Bar de la Plaza" || editado.Activo || strings.Join(editado.Pueblos, ",") != "almenar" || editado.Impresiones != 1 {
		t.Errorf("anuncio editado: %+v", editado)
	}
	if anuncios := pedirAPI(t, h, "pueblo=almenar"); len(anuncios) != 0 {
		t.Error("un anuncio pausado no debería salir")
	}

	req = conClave(peticion("POST", "/admin/anuncios/"+id+"/borrar", strings.NewReader(url.Values{"csrf": {token}}.Encode())))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if rec := pedir(h, req); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admin/?ok=borrado" {
		t.Fatalf("borrar: %d", rec.Code)
	}
	if _, err := app.db.Obtener(t.Context(), an.ID); !errors.Is(err, ErrNoExiste) {
		t.Errorf("el anuncio debería estar borrado: %v", err)
	}
	var filas int
	app.db.db.QueryRow(`SELECT COUNT(*) FROM estadisticas`).Scan(&filas)
	if filas != 0 {
		t.Errorf("las estadísticas deberían borrarse con el anuncio, quedan %d", filas)
	}
}
