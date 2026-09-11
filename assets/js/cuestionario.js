(function () {
  'use strict';

  const raiz = document.getElementById('cuestionario');
  const datosEl = document.getElementById('cuestionario-datos');
  if (!raiz || !datosEl) return;

  const datos = JSON.parse(datosEl.textContent);
  const preguntas = datos.preguntas.map((q) => Object.assign({}, q, {
    opciones: (q.opciones || []).map((o) => (typeof o === 'string' ? { valor: o } : o))
  }));
  const porId = Object.fromEntries(preguntas.map((q) => [q.id, q]));
  const cfg = raiz.dataset;
  const pantalla = raiz.querySelector('.pantalla');
  const barra = raiz.querySelector('.progreso');
  const params = new URLSearchParams(location.search);
  const creador = params.has('creador');
  const CLAVE = 'tengounpueblo:cuestionario';

  let respuestas = leer();
  const habiaProgreso = Object.values(respuestas).some((v) => !vacia(v));
  let actual = null;
  let copias = {};
  let conPuntero = false;
  let temporizador = null;

  // /cuestionario/?pueblo=Alfarràs&provincia=Lleida rellena esas respuestas.
  ['pueblo', 'provincia'].forEach((id) => {
    if (porId[id] && params.get(id) && vacia(respuestas[id])) respuestas[id] = params.get(id);
  });

  // ─── Utilidades ───────────────────────────────────────────────────
  function leer() {
    try { return JSON.parse(localStorage.getItem(CLAVE)) || {}; } catch (e) { return {}; }
  }
  function guardar() {
    try { localStorage.setItem(CLAVE, JSON.stringify(respuestas)); } catch (e) { /* sin almacenamiento */ }
  }
  function borrar() {
    respuestas = {};
    try { localStorage.removeItem(CLAVE); localStorage.removeItem(CLAVE + ':paso'); } catch (e) { /* sin almacenamiento */ }
  }
  // Recuerda la última pantalla vista para "Continuar donde lo dejaste".
  function recordarPaso(id) {
    try { localStorage.setItem(CLAVE + ':paso', id); } catch (e) { /* sin almacenamiento */ }
  }
  function ultimoPaso() {
    try { return localStorage.getItem(CLAVE + ':paso'); } catch (e) { return null; }
  }
  function escapar(texto) {
    return String(texto == null ? '' : texto).replace(/[&<>"']/g, (c) => (
      { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]
    ));
  }
  function vacia(v) {
    return Array.isArray(v) ? v.length === 0 : !String(v == null ? '' : v).trim();
  }
  function texto(q) {
    const v = respuestas[q.id];
    if (vacia(v)) return '';
    return Array.isArray(v) ? v.join(', ') : String(v).trim();
  }
  function elegida(q, valor) {
    const v = respuestas[q.id];
    return Array.isArray(v) ? v.includes(valor) : v === valor;
  }
  function visible(q) {
    return !q.si || (porId[q.si.id] && elegida(porId[q.si.id], q.si.es));
  }
  function visibles() {
    return preguntas.filter(visible);
  }
  function subirArriba() {
    const y = raiz.getBoundingClientRect().top + window.scrollY - 90;
    if (window.scrollY > y) window.scrollTo({ top: y });
  }
  function progreso(parte) {
    barra.hidden = false;
    barra.firstElementChild.style.width = Math.round(parte * 100) + '%';
  }

  // ─── Pantalla de inicio ───────────────────────────────────────────
  function mostrarInicio() {
    barra.hidden = true;
    pantalla.innerHTML =
      '<div class="paso">' +
        '<p class="paso-seccion">' + escapar(datos.antetitulo) + '</p>' +
        '<h1 tabindex="-1">' + escapar(datos.titulo) + '</h1>' +
        '<p class="entradilla">' + escapar(datos.intro) + '</p>' +
        '<div class="paso-botones">' +
          (habiaProgreso
            ? '<button type="button" class="btn" data-accion="continuar">Continuar donde lo dejaste</button>' +
              '<button type="button" class="btn btn-borde" data-accion="reiniciar">Empezar de nuevo</button>'
            : '<button type="button" class="btn" data-accion="empezar">Empezar</button>') +
        '</div>' +
        (creador
          ? '<details class="importar"><summary>Modo creador: pegar respuestas recibidas</summary>' +
            '<p class="ayuda">Pega el mensaje que te llegó por WhatsApp o email y se generarán los prompts.</p>' +
            '<textarea class="campo" rows="7" id="importar-texto"></textarea>' +
            '<p class="error" hidden>No he reconocido ninguna respuesta en ese texto.</p>' +
            '<button type="button" class="btn btn-peq" data-accion="importar">Cargar respuestas</button></details>'
          : '') +
      '</div>';
  }

  // ─── Una pregunta ─────────────────────────────────────────────────
  function mostrarPregunta(i) {
    const lista = visibles();
    const indice = Math.max(0, Math.min(i, lista.length - 1));
    const q = lista[indice];
    const v = respuestas[q.id];
    actual = q;
    recordarPaso(q.id);
    progreso(indice / lista.length);

    let campo;
    let titulo = escapar(q.pregunta);
    if (q.tipo === 'opciones' || q.tipo === 'multiple') {
      const tipo = q.tipo === 'opciones' ? 'radio' : 'checkbox';
      campo = '<fieldset class="opciones"><legend class="sr">' + escapar(q.pregunta) + '</legend>' +
        q.opciones.map((o) => '<label class="opcion"><input type="' + tipo + '" name="r" value="' + escapar(o.valor) + '"' +
          (elegida(q, o.valor) ? ' checked' : '') + '><span>' + escapar(o.valor) + '</span></label>').join('') +
        '</fieldset>';
    } else {
      titulo = '<label for="r">' + titulo + '</label>';
      const comunes = ' class="campo" id="r" name="r" placeholder="' + escapar(q.placeholder || '') + '"';
      campo = q.tipo === 'parrafo'
        ? '<textarea' + comunes + ' rows="4">' + escapar(v || '') + '</textarea><p class="atajo">Ctrl + Enter para continuar</p>'
        : '<input' + comunes + ' type="text" autocomplete="' + escapar(q.autocompletar || 'off') + '" value="' + escapar(v || '') + '">';
    }

    pantalla.innerHTML =
      '<form class="paso" novalidate>' +
        '<p class="paso-seccion">' + escapar(q.seccion || '') + ' <span>· ' + (indice + 1) + ' de ' + lista.length + '</span></p>' +
        '<h1>' + titulo + (q.obligatoria ? '' : ' <small>(opcional)</small>') + '</h1>' +
        (q.ayuda ? '<p class="ayuda">' + escapar(q.ayuda) + '</p>' : '') +
        campo +
        '<p class="error" role="alert" hidden>Esta pregunta necesita una respuesta.</p>' +
        '<div class="paso-botones">' +
          '<button type="button" class="btn btn-borde" data-accion="' + (indice > 0 ? 'atras' : 'inicio') + '">← ' + (indice > 0 ? 'Anterior' : 'Inicio') + '</button>' +
          '<button type="submit" class="btn" data-siguiente></button>' +
        '</div>' +
      '</form>';

    const form = pantalla.querySelector('form');
    const error = form.querySelector('.error');
    const botonSiguiente = form.querySelector('[data-siguiente]');
    const etiquetaBoton = () => {
      botonSiguiente.textContent = (!q.obligatoria && vacia(leerCampo(form, q))) ? 'Saltar →' : 'Siguiente →';
    };
    etiquetaBoton();

    form.addEventListener('input', () => {
      respuestas[q.id] = leerCampo(form, q);
      guardar();
      error.hidden = true;
      etiquetaBoton();
    });
    form.addEventListener('change', (e) => {
      // Al tocar una opción única con el ratón o el dedo, avanza solo.
      if (e.target.type === 'radio' && conPuntero) {
        clearTimeout(temporizador);
        temporizador = setTimeout(() => form.requestSubmit(), 250);
      }
    });
    form.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); form.requestSubmit(); }
    });
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      clearTimeout(temporizador);
      const valor = leerCampo(form, q);
      if (q.obligatoria && vacia(valor)) {
        error.hidden = false;
        (form.querySelector('.campo, input') || form).focus();
        return;
      }
      respuestas[q.id] = valor;
      guardar();
      const nueva = visibles();
      const pos = nueva.indexOf(q);
      if (pos + 1 < nueva.length) mostrarPregunta(pos + 1);
      else mostrarFinal();
    });

    subirArriba();
    const foco = form.querySelector('.campo') || form.querySelector('input:checked') || form.querySelector('input');
    if (foco) foco.focus({ preventScroll: true });
  }

  function leerCampo(form, q) {
    if (q.tipo === 'multiple') return Array.from(form.querySelectorAll('input:checked'), (x) => x.value);
    if (q.tipo === 'opciones') {
      const x = form.querySelector('input:checked');
      return x ? x.value : '';
    }
    return form.querySelector('.campo').value;
  }

  // ─── Resumen y prompts ────────────────────────────────────────────
  function resumen() {
    const pueblo = porId.pueblo ? texto(porId.pueblo) : '';
    const lineas = ['🎵 Canción para ' + (pueblo || 'mi pueblo') + ' · tengounpueblo.com'];
    visibles().forEach((q) => {
      const t = texto(q);
      if (t) lineas.push('', '*' + q.etiqueta + '*', t);
    });
    return lineas.join('\n');
  }

  function prompts() {
    const valores = {};
    visibles().forEach((q) => { valores[q.id] = texto(q).replace(/\s*\n+\s*/g, '; '); });

    const letra = String(datos.prompt_letra || '').split('\n')
      .filter((linea) => (linea.match(/\{(\w+)\}/g) || []).every((c) => valores[c.slice(1, -1)]))
      .map((linea) => linea.replace(/\{(\w+)\}/g, (_, id) => valores[id]))
      .join('\n').replace(/\n{3,}/g, '\n\n').trim();

    const etiquetas = [];
    const añadir = (lista) => String(lista || '').split(',').forEach((t) => {
      t = t.trim();
      if (t && !etiquetas.includes(t)) etiquetas.push(t);
    });
    visibles().forEach((q) => q.opciones.forEach((o) => { if (elegida(q, o.valor)) añadir(o.estilo); }));
    añadir(datos.prompt_estilo_extra);

    return { estilo: etiquetas.join(', '), letra };
  }

  function importar(textoPegado) {
    const porEtiqueta = Object.fromEntries(preguntas.map((q) => [q.etiqueta.toLowerCase(), q]));
    const nuevas = {};
    let pregunta = null;
    let buffer = [];
    const cerrar = () => {
      const v = buffer.join('\n').trim();
      if (pregunta && v) nuevas[pregunta.id] = pregunta.tipo === 'multiple' ? v.split(/\s*,\s*/) : v;
      buffer = [];
    };
    textoPegado.split(/\r?\n/).forEach((linea) => {
      const m = linea.trim().match(/^\*(.+)\*$/);
      if (m && porEtiqueta[m[1].trim().toLowerCase()]) {
        cerrar();
        pregunta = porEtiqueta[m[1].trim().toLowerCase()];
      } else if (pregunta) {
        buffer.push(linea);
      }
    });
    cerrar();
    return nuevas;
  }

  // ─── Pantalla final ───────────────────────────────────────────────
  function mostrarFinal() {
    actual = null;
    recordarPaso('final');
    progreso(1);
    const pueblo = porId.pueblo ? texto(porId.pueblo) : '';
    copias = { resumen: resumen() };

    let envio = '';
    if (cfg.formspree) {
      envio += '<button type="button" class="btn" data-accion="enviar">Enviar respuestas</button>';
    }
    if (cfg.whatsapp) {
      envio += '<a class="btn btn-whatsapp" target="_blank" rel="noopener" href="https://wa.me/' + escapar(cfg.whatsapp) +
        '?text=' + encodeURIComponent(copias.resumen) + '">Enviar por WhatsApp</a>';
    }
    envio += '<a class="btn' + (cfg.formspree || cfg.whatsapp ? ' btn-borde' : '') + '" href="mailto:' + escapar(cfg.email) +
      '?subject=' + encodeURIComponent('Canción para ' + (pueblo || 'mi pueblo')) +
      '&body=' + encodeURIComponent(copias.resumen) + '">Enviar por email</a>';
    envio += '<button type="button" class="btn btn-borde" data-accion="copiar" data-copiar="resumen">Copiar respuestas</button>';

    let bloqueCreador = '';
    if (creador) {
      const p = prompts();
      copias.estilo = p.estilo;
      copias.letra = p.letra;
      bloqueCreador =
        '<section class="bloque-prompt"><h2>Prompt de estilo musical</h2>' +
          '<p class="ayuda">Para el campo de estilo de tu herramienta de música con IA.</p>' +
          '<textarea class="campo" readonly rows="3">' + escapar(p.estilo) + '</textarea>' +
          '<button type="button" class="btn btn-peq btn-borde" data-accion="copiar" data-copiar="estilo">Copiar estilo</button></section>' +
        '<section class="bloque-prompt"><h2>Prompt para escribir la letra</h2>' +
          '<p class="ayuda">Pégalo en tu asistente de IA para obtener la letra con su estructura.</p>' +
          '<textarea class="campo" readonly rows="14">' + escapar(p.letra) + '</textarea>' +
          '<button type="button" class="btn btn-peq btn-borde" data-accion="copiar" data-copiar="letra">Copiar prompt de letra</button></section>';
    }

    pantalla.innerHTML =
      '<div class="paso">' +
        '<p class="paso-seccion">' + escapar(pueblo || datos.antetitulo) + '</p>' +
        '<h1 tabindex="-1">' + escapar(datos.final_titulo) + '</h1>' +
        '<p class="ayuda">' + escapar(datos.final_texto) + '</p>' +
        '<div class="resumen">' + escapar(copias.resumen) + '</div>' +
        '<div class="acciones envio">' + envio + '</div>' +
        bloqueCreador +
        '<div class="paso-botones">' +
          '<button type="button" class="btn btn-borde" data-accion="editar">← Revisar respuestas</button>' +
          '<button type="button" class="btn btn-borde" data-accion="reiniciar">Empezar de nuevo</button>' +
        '</div>' +
      '</div>';
    subirArriba();
    pantalla.querySelector('h1').focus({ preventScroll: true });
  }

  function enviar(boton) {
    const pueblo = porId.pueblo ? texto(porId.pueblo) : '';
    const cuerpo = { _subject: 'Canción para ' + (pueblo || 'un pueblo'), resumen: copias.resumen };
    visibles().forEach((q) => { const t = texto(q); if (t) cuerpo[q.etiqueta] = t; });
    const contacto = porId.contacto ? texto(porId.contacto) : '';
    if (contacto.includes('@')) cuerpo.email = contacto;

    boton.disabled = true;
    boton.textContent = 'Enviando…';
    fetch('https://formspree.io/f/' + encodeURIComponent(cfg.formspree), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(cuerpo)
    }).then((r) => {
      if (!r.ok) throw new Error(r.status);
      boton.outerHTML = '<p class="aviso-ok" role="status">¡Recibido! Te contestaremos muy pronto.</p>';
    }).catch(() => {
      boton.disabled = false;
      boton.textContent = 'No se pudo enviar. Reintentar';
    });
  }

  // ─── Botones ──────────────────────────────────────────────────────
  pantalla.addEventListener('pointerdown', () => { conPuntero = true; });
  pantalla.addEventListener('keydown', () => { conPuntero = false; });

  pantalla.addEventListener('click', (e) => {
    const boton = e.target.closest('[data-accion]');
    if (!boton) return;
    const accion = boton.dataset.accion;

    if (accion === 'empezar') mostrarPregunta(0);
    else if (accion === 'inicio') mostrarInicio();
    else if (accion === 'continuar') {
      const lista = visibles();
      const pos = lista.findIndex((q) => q.id === ultimoPaso());
      const pendiente = lista.findIndex((q) => q.obligatoria && vacia(respuestas[q.id]));
      if (pos !== -1) mostrarPregunta(pos);
      else if (pendiente !== -1) mostrarPregunta(pendiente);
      else mostrarFinal();
    } else if (accion === 'reiniciar') {
      borrar();
      mostrarPregunta(0);
    } else if (accion === 'atras' && actual) {
      mostrarPregunta(visibles().indexOf(actual) - 1);
    } else if (accion === 'editar') {
      mostrarPregunta(0);
    } else if (accion === 'enviar') {
      enviar(boton);
    } else if (accion === 'copiar') {
      const original = boton.textContent;
      navigator.clipboard.writeText(copias[boton.dataset.copiar] || '').then(() => {
        boton.textContent = '¡Copiado!';
        setTimeout(() => { boton.textContent = original; }, 1800);
      });
    } else if (accion === 'importar') {
      const nuevas = importar(document.getElementById('importar-texto').value);
      if (!Object.keys(nuevas).length) {
        boton.parentNode.querySelector('.error').hidden = false;
        return;
      }
      respuestas = nuevas;
      guardar();
      mostrarFinal();
    }
  });

  mostrarInicio();
})();
