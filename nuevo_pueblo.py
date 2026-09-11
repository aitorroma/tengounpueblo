#!/usr/bin/env python3
"""Crea la ficha de un pueblo en _pueblos/ buscando sus coordenadas en OpenStreetMap.

Uso:
  python3 nuevo_pueblo.py "Alfarràs" "Lleida"
  python3 nuevo_pueblo.py "Alfarràs" "Lleida" --spotify "https://open.spotify.com/track/..."
  python3 nuevo_pueblo.py "Mi Pueblo" "Teruel" --lat 40.34 --lng -1.10   # sin buscar en internet
"""
import argparse
import json
import re
import sys
import unicodedata
import urllib.parse
import urllib.request
from datetime import date
from pathlib import Path

CARPETA = Path(__file__).resolve().parent / "_pueblos"
TIPOS_POBLACION = {"village", "town", "city", "municipality", "hamlet", "suburb"}


def slug(texto):
    texto = unicodedata.normalize("NFKD", texto).encode("ascii", "ignore").decode()
    return re.sub(r"[^a-z0-9]+", "-", texto.lower()).strip("-")


def buscar(nombre, provincia, pais):
    consulta = ", ".join(x for x in (nombre, provincia, pais) if x)
    url = "https://nominatim.openstreetmap.org/search?" + urllib.parse.urlencode(
        {"q": consulta, "format": "jsonv2", "addressdetails": 1, "limit": 5}
    )
    peticion = urllib.request.Request(url, headers={"User-Agent": "tengounpueblo.com (nuevo_pueblo.py)"})
    with urllib.request.urlopen(peticion, timeout=15) as r:
        resultados = json.load(r)
    if not resultados:
        return None
    return next((r for r in resultados if r.get("addresstype") in TIPOS_POBLACION), resultados[0])


def texto_yaml(valor):
    return json.dumps(valor or "", ensure_ascii=False)


def main():
    p = argparse.ArgumentParser(description="Crea la ficha de un pueblo.")
    p.add_argument("nombre", help='Nombre del pueblo, p. ej. "Alfarràs"')
    p.add_argument("provincia", nargs="?", default="", help='Provincia, p. ej. "Lleida" (ayuda a encontrarlo)')
    p.add_argument("--spotify", default="", help="Enlace de Spotify de la canción")
    p.add_argument("--youtube", default="", help="Enlace de YouTube")
    p.add_argument("--lat", type=float, help="Latitud (si no quieres buscar en OpenStreetMap)")
    p.add_argument("--lng", type=float, help="Longitud")
    p.add_argument("--pais", default="España")
    p.add_argument("--forzar", action="store_true", help="Sobrescribir si ya existe")
    args = p.parse_args()

    archivo = CARPETA / f"{slug(args.nombre)}.md"
    if archivo.exists() and not args.forzar:
        sys.exit(f"Ya existe {archivo.relative_to(CARPETA.parent)} (usa --forzar para sobrescribir)")

    lat, lng, provincia, comarca, comunidad = args.lat, args.lng, args.provincia, "", ""
    if lat is None or lng is None:
        try:
            lugar = buscar(args.nombre, args.provincia, args.pais)
        except OSError as e:
            sys.exit(f"No se pudo consultar OpenStreetMap ({e}). Usa --lat y --lng.")
        if not lugar:
            sys.exit("No lo encuentro en OpenStreetMap. Revisa el nombre o usa --lat y --lng.")
        lat, lng = round(float(lugar["lat"]), 5), round(float(lugar["lon"]), 5)
        dir_ = lugar.get("address", {})
        provincia = dir_.get("province") or dir_.get("state_district") or args.provincia or dir_.get("county", "")
        if dir_.get("county") and dir_.get("county") != provincia:
            comarca = dir_["county"]
        comunidad = dir_.get("state", "")
        print(f"Encontrado: {lugar.get('display_name')}")

    CARPETA.mkdir(exist_ok=True)
    archivo.write_text(f"""---
title: {texto_yaml(args.nombre)}
cancion: ""       # título de la canción, p. ej. "Som fills de l'Aigua"
provincia: {texto_yaml(provincia)}
comarca: {texto_yaml(comarca)}
comunidad: {texto_yaml(comunidad)}
lat: {lat}
lng: {lng}

# Enlaces (pega la URL tal cual la copias de la app; deja "" si no hay)
spotify: {texto_yaml(args.spotify)}
youtube: {texto_yaml(args.youtube)}
apple_music: ""
amazon_music: ""

# Anunciantes (opcional, puede haber varios): quita los # para activarlo
# patrocinadores:
#   - nombre: "Bar de la Plaza"
#     url: "https://..."                                     # web, Instagram o Google Maps
#     logo: "/assets/img/patrocinadores/bar-de-la-plaza.png"    # cuadrado, fondo claro
#     texto: "Tapas y vermut desde 1985"
#   - nombre: "Forn de pa Cal Josep"
#     url: "https://..."

date: {date.today().isoformat()}
description: ""   # opcional: frase para Google y al compartir en redes
portada: ""       # opcional: /assets/img/pueblos/{slug(args.nombre)}.jpg (1200x630)

# Letra: cada verso con 2 espacios delante y una línea en blanco entre estrofas.
# Se puede pegar tal cual sale de Suno, con [Verse], [Chorus], [Bridge]…
letra: |
  Primer verso
  Segundo verso

  Estribillo...
---

Escribe aquí (opcional) la historia de la canción o curiosidades de {args.nombre}.
""", encoding="utf-8")

    print(f"Creado {archivo.relative_to(CARPETA.parent)}  →  https://tengounpueblo.com/{slug(args.nombre)}/")


if __name__ == "__main__":
    main()
