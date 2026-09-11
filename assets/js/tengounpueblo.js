(function () {
  'use strict';

  // Mapas de OpenStreetMap (gratis, sin clave). Si el tráfico crece, cambia a MapTiler o Stadia con su clave gratuita.
  const TILES = 'https://tile.openstreetmap.org/{z}/{x}/{y}.png';
  const ATRIBUCION = '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>';
  const ESPANA = [[36.0, -9.4], [43.8, 3.4]];

  let pueblos = null;
  function cargarPueblos() {
    pueblos = pueblos || fetch(document.body.dataset.pueblos).then((r) => r.json());
    return pueblos;
  }

  function normalizar(texto) {
    return (texto || '').normalize('NFD').replace(/[\u0300-\u036f]/g, '')
      .toLowerCase().replace(/[^a-z0-9]+/g, ' ').trim();
  }

  function escapar(texto) {
    return String(texto == null ? '' : texto).replace(/[&<>"']/g, (c) => (
      { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
    ));
  }

  function distanciaKm(lat1, lng1, lat2, lng2) {
    const rad = Math.PI / 180;
    const a = Math.sin((lat2 - lat1) * rad / 2) ** 2 +
      Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin((lng2 - lng1) * rad / 2) ** 2;
    return 12742 * Math.asin(Math.sqrt(a));
  }

  function capaBase(mapa) {
    L.tileLayer(TILES, { maxZoom: 19, attribution: ATRIBUCION }).addTo(mapa);
  }

  function icono(conCancion) {
    const color = conCancion ? '#c6693b' : '#8a9a7b';
    return L.divIcon({
      className: 'pin',
      iconSize: [30, 40],
      iconAnchor: [15, 40],
      popupAnchor: [0, -36],
      html: '<svg viewBox="0 0 30 40" width="30" height="40" aria-hidden="true">' +
        '<path d="M15 1C7.3 1 1 7.2 1 14.9 1 25.3 15 39 15 39s14-13.7 14-24.1C29 7.2 22.7 1 15 1z" fill="' + color + '" stroke="#fff" stroke-width="2"/>' +
        '<path d="M13 8.5v9.6a3.4 3.4 0 1 0 2 3.1v-9.2h4.5V8.5z" fill="#fff"/></svg>'
    });
  }

  function tarjeta(p, extra) {
    return '<li><a class="tarjeta" href="' + escapar(p.url) + '"><strong>' + escapar(p.nombre) + '</strong>' +
      '<span>' + escapar(p.provincia) + (extra || '') + '</span>' +
      (p.cancion ? '' : '<em class="etiqueta">Próximamente</em>') + '</a></li>';
  }

  // ─── Mapa de la portada ───────────────────────────────────────────
  function mapaPrincipal(el) {
    const mapa = L.map(el, { scrollWheelZoom: false });
    mapa.fitBounds(ESPANA);
    capaBase(mapa);
    // La rueda del ratón solo hace zoom después de hacer clic, para no atrapar el scroll de la página.
    mapa.on('click', () => mapa.scrollWheelZoom.enable());
    mapa.on('mouseout', () => mapa.scrollWheelZoom.disable());

    const grupo = L.markerClusterGroup
      ? L.markerClusterGroup({
        showCoverageOnHover: false,
        maxClusterRadius: 45,
        iconCreateFunction: (c) => L.divIcon({ className: 'cluster', html: '<span>' + c.getChildCount() + '</span>', iconSize: [42, 42] })
      })
      : L.featureGroup();

    cargarPueblos().then((lista) => {
      lista.forEach((p) => {
        if (p.lat == null || p.lng == null) return;
        L.marker([p.lat, p.lng], { icon: icono(p.cancion), title: p.nombre })
          .bindPopup('<div class="popup"><strong>' + escapar(p.nombre) + '</strong><span>' + escapar(p.provincia) + '</span>' +
            ((p.patrocinadores || []).length ? '<small class="popup-patrocinio">Con el apoyo de ' + escapar(p.patrocinadores.join(', ')) + '</small>' : '') +
            '<a class="btn btn-peq" href="' + escapar(p.url) + '">' + (p.cancion ? 'Escuchar la canción' : 'Ver ficha') + '</a></div>')
          .addTo(grupo);
      });
      grupo.addTo(mapa);
    });
  }

  // ─── Buscador ─────────────────────────────────────────────────────
  function buscador(input) {
    const caja = document.getElementById(input.getAttribute('aria-controls'));
    const encarga = document.querySelector('.nav .btn');
    let resultados = [];
    let activo = -1;

    function pintar() {
      const q = normalizar(input.value);
      if (!q) { caja.hidden = true; return; }
      cargarPueblos().then((lista) => {
        resultados = lista
          .map((p) => ({ p, n: normalizar(p.nombre) }))
          .filter((r) => r.n.includes(q))
          .sort((a, b) => (b.n.startsWith(q) - a.n.startsWith(q)) || a.n.localeCompare(b.n))
          .slice(0, 8)
          .map((r) => r.p);
        activo = resultados.length ? 0 : -1;
        caja.innerHTML = resultados.length
          ? resultados.map((p, i) => '<li><a href="' + escapar(p.url) + '"' + (i === 0 ? ' class="activa"' : '') + '>' +
            escapar(p.nombre) + '<span>' + escapar(p.provincia) + '</span></a></li>').join('')
          : '<li class="vacio">Aún no tenemos «' + escapar(input.value) + '». <a href="' + (encarga ? encarga.href : '#') + '">¡Encarga su canción!</a></li>';
        caja.hidden = false;
      });
    }

    function marcar(i) {
      const enlaces = caja.querySelectorAll('li > a');
      if (!enlaces.length) return;
      activo = (i + enlaces.length) % enlaces.length;
      enlaces.forEach((a, j) => a.classList.toggle('activa', j === activo));
    }

    input.addEventListener('input', pintar);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowDown') { e.preventDefault(); marcar(activo + 1); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); marcar(activo - 1); }
      else if (e.key === 'Enter' && resultados[activo]) { e.preventDefault(); location.href = resultados[activo].url; }
      else if (e.key === 'Escape') { caja.hidden = true; }
    });
    document.addEventListener('click', (e) => {
      if (!input.parentNode.contains(e.target)) caja.hidden = true;
    });
  }

  // ─── Ficha del pueblo ─────────────────────────────────────────────
  function miniMapa(el) {
    const punto = [parseFloat(el.dataset.lat), parseFloat(el.dataset.lng)];
    const mapa = L.map(el, { scrollWheelZoom: false, dragging: !L.Browser.mobile }).setView(punto, 11);
    capaBase(mapa);
    L.marker(punto, { icon: icono(true) }).addTo(mapa);
  }

  function cercanos(ul) {
    const lat = parseFloat(ul.dataset.lat);
    const lng = parseFloat(ul.dataset.lng);
    cargarPueblos().then((lista) => {
      const cerca = lista
        .filter((p) => p.url !== ul.dataset.url && p.lat != null && p.lng != null)
        .map((p) => Object.assign({ km: distanciaKm(lat, lng, p.lat, p.lng) }, p))
        .sort((a, b) => a.km - b.km)
        .slice(0, 6);
      if (!cerca.length) return;
      ul.innerHTML = cerca.map((p) => tarjeta(p, ' · ' + Math.round(p.km) + ' km')).join('');
      ul.closest('section').hidden = false;
    });
  }

  function compartir(caja) {
    const canonica = document.querySelector('link[rel="canonical"]');
    const url = canonica ? canonica.href : location.href;
    const texto = caja.dataset.texto || document.title;
    const enlaces = {
      whatsapp: 'https://wa.me/?text=' + encodeURIComponent(texto + ' ' + url),
      facebook: 'https://www.facebook.com/sharer/sharer.php?u=' + encodeURIComponent(url),
      x: 'https://twitter.com/intent/tweet?text=' + encodeURIComponent(texto) + '&url=' + encodeURIComponent(url)
    };
    const menu = caja.querySelector('details');
    if (menu) {
      // En móvil, "Compartir" abre directamente el menú nativo del teléfono.
      if (navigator.share) {
        menu.querySelector('summary').addEventListener('click', (e) => {
          e.preventDefault();
          navigator.share({ title: document.title, text: texto, url }).catch(() => {});
        });
      }
      document.addEventListener('click', (e) => { if (!menu.contains(e.target)) menu.open = false; });
    }
    caja.querySelectorAll('[data-compartir]').forEach((a) => {
      const red = a.dataset.compartir;
      if (enlaces[red]) { a.href = enlaces[red]; return; }
      a.addEventListener('click', (e) => {
        e.preventDefault();
        navigator.clipboard.writeText(url).then(() => {
          a.textContent = '¡Enlace copiado!';
          setTimeout(() => { a.textContent = 'Copiar enlace'; }, 2000);
        });
      });
    });
  }

  // Las letras largas se muestran plegadas con un botón para verlas enteras.
  function plegarLetra(letra) {
    if (letra.scrollHeight < 900) return;
    letra.classList.add('plegada');
    const boton = document.createElement('button');
    boton.type = 'button';
    boton.className = 'btn ver-letra';
    boton.textContent = 'Ver la letra completa';
    boton.addEventListener('click', () => {
      letra.classList.remove('plegada');
      boton.remove();
    });
    letra.appendChild(boton);
  }

  // ─── Arranque ─────────────────────────────────────────────────────
  const $ = (sel) => document.querySelector(sel);
  if (window.L && $('#mapa-principal')) mapaPrincipal($('#mapa-principal'));
  if (window.L && $('#minimapa')) miniMapa($('#minimapa'));
  if ($('#buscar')) buscador($('#buscar'));
  if ($('#cercanos')) cercanos($('#cercanos'));
  if ($('.compartir')) compartir($('.compartir'));
  if ($('.letra')) plegarLetra($('.letra'));

  window.TengoUnPueblo = { cargarPueblos, normalizar };
})();
