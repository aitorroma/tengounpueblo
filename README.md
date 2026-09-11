# tengounpueblo.com

Web estática de **Tengo un pueblo · canciones de pueblos**: un mapa con un punto por pueblo y una ficha para cada uno (`tengounpueblo.com/alfarras/`) con el reproductor de Spotify.

Está hecha con Jekyll, así que **GitHub Pages la compila sola**. No hace falta instalar nada para publicar.

## Añadir un pueblo

**Opción A: con el script** (busca las coordenadas, la provincia y la comarca en OpenStreetMap):

```bash
python3 nuevo_pueblo.py "Alfarràs" "Lleida" --spotify "https://open.spotify.com/track/..."
```

**Opción B: a mano** (también desde la web de GitHub, con *Add file → Create new file*): crea `_pueblos/nombre-del-pueblo.md` copiando `_pueblos/alfarras.md`.

El nombre del archivo es la URL: `_pueblos/villanueva-del-rio.md` → `tengounpueblo.com/villanueva-del-rio/`.

| Campo | Qué es |
|---|---|
| `title` | Nombre del pueblo tal cual (con tildes) |
| `provincia`, `comarca` | Se muestran bajo el nombre |
| `lat`, `lng` | Coordenadas del punto en el mapa |
| `spotify` | Enlace copiado de Spotify (canción, álbum o playlist). Si está vacío, la ficha dice «Muy pronto» y el punto sale en gris |
| `youtube`, `apple_music`, `amazon_music` | Opcionales. El vídeo de YouTube se incrusta en la ficha |
| `cancion` | Opcional. Título de la canción; sale en la cabecera y en Google |
| `letra` | Opcional. Cada verso con 2 espacios delante; una línea en blanco entre estrofas. Se puede pegar tal cual sale de Suno: `[Verse]`, `[Chorus]`, `[Bridge]`… se muestran como «Estrofa», «Estribillo», «Puente», y los estribillos van resaltados |
| `description` | Opcional. Frase para Google y para cuando se comparte |
| `portada` | Opcional. Imagen 1200×630 para compartir en redes (en `assets/img/pueblos/`) |

Lo que escribas debajo del segundo `---` aparece en la ficha (historia, curiosidades…).

Haz `git push` y en uno o dos minutos está publicado.

## Publicar en GitHub Pages

1. Crea un repositorio en GitHub (por ejemplo `tengounpueblo`) y sube esto:
   ```bash
   git init && git add . && git commit -m "Primera versión"
   git branch -M main
   git remote add origin git@github.com:TU_USUARIO/tengounpueblo.git
   git push -u origin main
   ```
2. En el repositorio, ve a **Settings → Pages → Source: Deploy from a branch → `main` / `(root)`**.
3. En el mismo sitio, pon `tengounpueblo.com` en **Custom domain** (el archivo `CNAME` ya lo tiene).
4. En el DNS de tu dominio, crea estos registros:
   | Tipo | Nombre | Valor |
   |---|---|---|
   | A | @ | 185.199.108.153 |
   | A | @ | 185.199.109.153 |
   | A | @ | 185.199.110.153 |
   | A | @ | 185.199.111.153 |
   | AAAA | @ | 2606:50c0:8000::153 (y `8001`, `8002`, `8003`) |
   | CNAME | www | TU_USUARIO.github.io |
5. Cuando el certificado esté listo, marca **Enforce HTTPS**.
6. Recomendado: verifica el dominio en **GitHub → Settings (de tu cuenta) → Pages**, para que nadie más pueda usarlo.

## Cuestionario para crear canciones

`tengounpueblo.com/cuestionario/` hace preguntas una a una para recoger el contexto de la canción: para qué es, lugares, fiestas, comida, personajes, expresiones, idioma, estilo, voz… Algunas preguntas solo aparecen según lo que se responda (por ejemplo, el nombre de la peña). Las respuestas se guardan en el navegador, así que se puede dejar a medias y seguir más tarde.

Al terminar, se ve un resumen que se envía por WhatsApp (si configuras `whatsapp`), por email o con Formspree (si configuras `formspree`).

- **Cambiar las preguntas:** edita `_data/cuestionario.yml`. No hay que tocar código.
- **Modo creador:** abre `tengounpueblo.com/cuestionario/?creador`. Al final aparecen dos prompts listos para copiar:
  - el de **estilo musical**, para tu herramienta de música con IA;
  - el de **la letra**, con todos los detalles y la estructura `[Verse]`, `[Chorus]`…

  En la pantalla de inicio puedes **pegar el mensaje que te envió un cliente** y te genera los prompts directamente. Las plantillas de los prompts están al final del mismo `.yml`.
- Las fichas «Muy pronto» enlazan al cuestionario con el pueblo ya rellenado, para que la gente del pueblo te dé ideas.

## Patrocinio de canciones

Un negocio paga `precio_patrocinio` (50 €) **por anuncio**: su anuncio se ve en la página de la canción del pueblo durante `patrocinio_duracion` (1 año). Los dos valores se cambian en `_config.yml`, y las peticiones llegan a `email_patrocinios`.

- **Fichas sin anunciante:** muestran «¿Tienes un negocio en…? 50 € por anuncio», que lleva a `/patrocina/?pueblo=…`. Esa página explica lo que incluye y abre el email o el WhatsApp con el pueblo ya escrito.
- **Para activar un patrocinio:**
  1. Guarda su logo en `assets/img/patrocinadores/`.
  2. En la ficha del pueblo, quita los `#` del bloque `patrocinador` y rellénalo:
     ```yaml
     patrocinador:
       nombre: "Bar de la Plaza"
       url: "https://www.instagram.com/bardelaplaza"
       logo: "/assets/img/patrocinadores/bar-de-la-plaza.png"
       texto: "Tapas y vermut desde 1985"
     ```
  3. Aparece bajo el reproductor («Canción patrocinada por»), en el globo del mapa y en la tarjeta de la portada. El enlace lleva `rel="sponsored"`, como pide Google para los enlaces pagados.
- **Cuando termine el año:** borra el bloque o renueva.

## Monetización

Todo se configura en `_config.yml`:

- **`spotify_artista`, `youtube_canal`, `instagram`, `tiktok`**: enlaces en el pie y en las fichas «Muy pronto».
- **`email`, `whatsapp`, `precio_desde`**: la página `/encarga/` para vender canciones por encargo (fiestas, peñas, ayuntamientos…). La portada, las fichas y la página de error 404 llevan allí.
- **`adsense_client` / `adsense_slot`**: si los rellenas, se añaden anuncios en las fichas y se genera `ads.txt` automáticamente.
  - En España/UE, Google exige un aviso de consentimiento de cookies. Actívalo en AdSense, en **Privacidad y mensajes**.
  - Completa también `privacidad.html`.
- **`goatcounter`**: analítica gratis y sin cookies ([goatcounter.com](https://www.goatcounter.com)), para ver qué pueblos se escuchan más.

El mapa usa las teselas públicas de OpenStreetMap, que son gratis y no necesitan clave pero están pensadas para un tráfico moderado. Si la web despega, crea una clave gratuita en [MapTiler](https://www.maptiler.com) o [Stadia Maps](https://stadiamaps.com) y cambia `TILES` en `assets/js/tengounpueblo.js`.

Extra: si alguien escribe `tengounpueblo.com/Alfarràs` o `/ALFARRAS`, la página 404 lo redirige a la ficha buena. Si el pueblo no existe, le invita a encargar la canción.

## Verlo en local (opcional)

Con Docker, sin instalar Ruby:

```bash
docker run --rm -it -p 4000:4000 -v "$PWD":/srv/jekyll -w /srv/jekyll ruby:3.3 \
  bash -c "bundle install && bundle exec jekyll serve --host 0.0.0.0"
```

Abre http://localhost:4000

## Estructura

```
_pueblos/          ← un .md por pueblo (lo único que tocarás a menudo)
_config.yml        ← nombre, redes, anuncios
_layouts/          ← plantilla general y ficha del pueblo
_includes/         ← cabecera, pie, reproductor Spotify/YouTube, anuncio, llamada a encargar
assets/            ← CSS, JS (mapa, buscador, compartir) e imágenes del logo
index.html         ← portada con mapa y buscador
encarga.html       ← página de encargos
pueblos.json       ← se genera solo; lo usan el mapa y el buscador
nuevo_pueblo.py    ← script para crear fichas
```
