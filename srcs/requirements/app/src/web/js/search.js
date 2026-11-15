import { searchSources, saveResult } from "./api.js";
import { EXTRA_LABELS, isSaved, sourceLabel } from "./state.js";
import { $, cloneTemplate, fillCard, formatDate, plural, setStatus } from "./dom.js";
import { refreshSavedAll } from "./saved.js";

export function initSearch() {
  const form = $("#search-form");
  form.addEventListener("submit", onSubmit);

  for (const chip of document.querySelectorAll("[data-example]")) {
    chip.addEventListener("click", () => {
      $("#q").value = chip.dataset.example;
      form.requestSubmit();
    });
  }
}

async function onSubmit(event) {
  event.preventDefault();
  const q = $("#q").value.trim();
  if (!q) return;

  const status = $("#search-status");
  const button = $("#search-form button[type=submit]");
  button.disabled = true;
  setStatus(status, "Buscando…");
  $("#results").replaceChildren();
  $("#source-errors").replaceChildren();

  const { ok, status: code, data } = await searchSources({ q, source: $("#source").value });
  button.disabled = false;

  if (!data || (!ok && code !== 502)) {
    setStatus(status, data?.error || "Error al buscar", "error");
    return;
  }

  const results = data.results || [];
  const allFailed = code === 502;
  renderSourceErrors(data.errors || [], !allFailed);

  if (allFailed) {
    setStatus(status, "No se pudo consultar ninguna fuente. Inténtalo de nuevo en unos minutos.", "error");
  } else if (results.length) {
    setStatus(status, `${plural(results.length, "resultado")} para «${data.query}»`);
  } else {
    setStatus(status, `Sin resultados para «${data.query}». Prueba con menos palabras o mira la pestaña Ayuda.`);
  }
  renderResults(results);
}

function renderSourceErrors(errors, othersWorked) {
  const box = $("#source-errors");
  for (const e of errors) {
    const p = document.createElement("p");
    p.className = "notice";
    p.textContent = `${sourceLabel(e.source)}: ${e.message}.` + (othersWorked ? " Se muestran el resto de fuentes." : "");
    box.append(p);
  }
}

function renderResults(results) {
  $("#results").replaceChildren(...results.map(resultCard));
}

function resultCard(result) {
  const node = cloneTemplate("result-template");
  fillCard(node, result);
  setupDetail(node, result);
  setupSave(node, result);
  return node;
}

function setupDetail(node, result) {
  const detail = $(".detail", node);
  const extra = result.extra || {};
  const keys = [
    ...Object.keys(EXTRA_LABELS).filter((k) => k in extra),
    ...Object.keys(extra).filter((k) => !(k in EXTRA_LABELS)),
  ];

  const addRow = (label, value) => {
    const dt = document.createElement("dt");
    dt.textContent = label;
    const dd = document.createElement("dd");
    dd.textContent = value;
    detail.append(dt, dd);
  };
  for (const k of keys) {
    addRow(EXTRA_LABELS[k] || k, formatDate(extra[k]) || extra[k]);
  }
  addRow("Identificador", result.external_id);

  const button = $(".btn-detail", node);
  button.addEventListener("click", () => {
    detail.hidden = !detail.hidden;
    button.setAttribute("aria-expanded", String(!detail.hidden));
    button.textContent = detail.hidden ? "Ver detalle" : "Ocultar detalle";
  });
}

function setupSave(node, result) {
  const button = $(".btn-save", node);
  const form = $(".save-form", node);

  const markSaved = () => {
    button.textContent = "Guardado ✓";
    button.disabled = true;
    form.hidden = true;
  };
  if (isSaved(result)) markSaved();

  button.addEventListener("click", () => {
    form.hidden = false;
    form.elements.tags.focus();
  });
  $(".btn-cancel", node).addEventListener("click", () => {
    form.hidden = true;
  });

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const tags = form.elements.tags.value.split(",").map((t) => t.trim()).filter(Boolean);
    const note = form.elements.note.value.trim();

    const submit = $("button[type=submit]", form);
    submit.disabled = true;
    const { ok, status, data } = await saveResult(result, { tags, note });
    submit.disabled = false;

    const message = $("#search-status");
    if (ok || status === 409) {
      markSaved();
      await refreshSavedAll();
      setStatus(message, status === 409 ? "Ya estaba guardado." : "Guardado en tu lista.", "ok");
    } else {
      setStatus(message, `No se pudo guardar: ${data?.error || "error " + status}`, "error");
    }
  });
}
