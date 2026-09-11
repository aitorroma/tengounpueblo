# Servicio de anuncios de tengounpueblo

Un pequeño servicio en Go para gestionar los anuncios de las canciones:

- **Panel `/admin/`** (con usuario y contraseña) para crear anuncios con nombre, frase, enlace, logo y fechas, y asociarlos a uno o varios pueblos. Los pueblos se leen de `pueblos.json` de la web, así no hay que darlos de alta dos veces.
- **Estadísticas** de cada anuncio: impresiones, clics y CTR, en total, por pueblo y por día. Sirven para enseñárselas al anunciante.
- **Caducidad automática:** cada anuncio solo se muestra entre sus fechas de inicio y fin. Se puede pausar sin borrarlo.
- **API pública** que consulta la ficha de cada pueblo en cada visita. Devuelve los anuncios en orden aleatorio, así rotan al recargar.

Todo va en un único binario con SQLite: no hace falta base de datos externa. Los datos (base de datos y logos) se guardan en `DATA_DIR`.

## Probarlo en local

```bash
cd anuncios
ADMIN_PASSWORD='una-clave-larga-de-verdad' SITE_URL=https://tengounpueblo.com go run .
```

Abre http://localhost:8080/admin/ (usuario `admin`).

Para ejecutar las pruebas: `go test ./...`

## Configuración

| Variable | Por defecto | Qué es |
|---|---|---|
| `ADMIN_PASSWORD` | — | **Obligatoria**, al menos 12 caracteres |
| `ADMIN_USER` | `admin` | Usuario del panel |
| `PUBLIC_URL` | `http://localhost:8080` | Dirección pública del servicio; se usa en los logos y los enlaces de clic |
| `SITE_URL` | `https://tengounpueblo.com` | Web de donde se leen los pueblos |
| `ALLOWED_ORIGINS` | igual que `SITE_URL` | Webs que pueden pedir anuncios desde el navegador, separadas por comas (`*` = cualquiera) |
| `DATA_DIR` | `./data` | Carpeta de la base de datos y los logos |
| `ADDR` | `:8080` | Dirección y puerto en el que escucha |
| `TRUST_PROXY` | — | Pon `1` si va detrás de un proxy, para limitar los intentos de contraseña por IP real |

## Desplegarlo en tu servidor

1. Copia la carpeta `anuncios/` al servidor y crea un archivo `.env` junto a `docker-compose.yml`:
   ```bash
   ADMIN_PASSWORD=pon-aqui-una-clave-larga
   ```
2. Revisa `PUBLIC_URL` y `ADMIN_USER` en `docker-compose.yml` y arranca:
   ```bash
   docker compose up -d --build
   ```
3. Ponlo detrás de un proxy con HTTPS. Con Caddy basta con:
   ```
   anuncios.tengounpueblo.com {
       reverse_proxy 127.0.0.1:8080
   }
   ```
4. En Cloudflare, crea el registro `anuncios` apuntando a la IP de tu servidor.
5. Comprueba que responde: `https://anuncios.tengounpueblo.com/salud` debe devolver `ok`.

**Copias de seguridad:** todo está en el volumen `datos` (`anuncios.db` y la carpeta `logos/`). Para sacar una copia:
`docker compose cp anuncios:/data ./copia-anuncios`

## Conectarlo con la web

En `_config.yml` de la web:

```yaml
anuncios_api: "https://anuncios.tengounpueblo.com"
anuncios_por_vista: 0   # 0 = todos los anuncios del pueblo; 1, 2… = solo esos, rotando
```

Haz `git push`. Desde entonces, las fichas muestran los anuncios del servicio en lugar de los `patrocinadores` del `.md`.

## API

**`GET /api/anuncios?pueblo=alfarras&n=0`**: anuncios en vigor del pueblo, en orden aleatorio. Con `n` mayor que 0 devuelve como máximo `n`. Cuenta una impresión por cada anuncio devuelto.

```json
{
  "anuncios": [
    {
      "nombre": "Bar de la Plaza",
      "texto": "Tapas y vermut desde 1985",
      "logo": "https://anuncios.tengounpueblo.com/logos/abc….png",
      "enlace": "https://anuncios.tengounpueblo.com/c/3?p=alfarras"
    }
  ]
}
```

**`GET /c/{id}?p=alfarras`**: cuenta el clic y redirige a la web del anunciante. Si el anuncio ya no existe, vuelve a la ficha del pueblo.

**`GET /logos/{archivo}`**: logos subidos. **`GET /salud`**: comprobación de que el servicio está vivo.

## Sobre las estadísticas y la seguridad

- **Qué cuenta como impresión:** cada vez que la ficha de un pueblo pide los anuncios, es decir, cada carga de página.
- **Bots:** no se cuentan buscadores ni previsualizaciones de enlaces (Google, WhatsApp, Facebook…).
- **Privacidad:** no se usan cookies ni se guardan datos personales, solo totales por anuncio, pueblo y día.
- **Contraseña:** tras 10 intentos fallidos desde la misma IP, el panel se bloquea 15 minutos para esa IP.
- **Formularios:** llevan protección CSRF.
- **Logos:** solo se aceptan PNG, JPG, WebP o GIF de hasta 2 MB (nunca SVG).
