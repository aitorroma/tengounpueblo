(function () {
  'use strict';

  const normalizar = (texto) => texto.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase();

  // Filtro de la lista de pueblos del formulario.
  document.querySelectorAll('[data-filtro]').forEach((input) => {
    const caja = document.querySelector(input.dataset.filtro);
    if (!caja) return;
    input.addEventListener('input', () => {
      const busqueda = normalizar(input.value.trim());
      caja.querySelectorAll('label').forEach((label) => {
        label.hidden = busqueda !== '' && !normalizar(label.textContent).includes(busqueda);
      });
    });
  });

  // Confirmación antes de borrar.
  document.querySelectorAll('form[data-confirmar]').forEach((form) => {
    form.addEventListener('submit', (e) => {
      if (!window.confirm(form.dataset.confirmar)) e.preventDefault();
    });
  });
})();
