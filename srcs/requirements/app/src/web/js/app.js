// Frontend de CloudFGD: buscador en fuentes públicas y lista de guardados.
// Todo el texto se pinta con textContent (nunca innerHTML) para evitar XSS.
"use strict";

const $ = (sel) => document.querySelector(sel);

// Estado de la página
const state = {
  sources: {},   // nombre -> etiqueta legible ("boe" -> "BOE · Legislación consolidada")
  savedAll: [],  // todos los guardados, para el contador, las etiquetas y saber qué ya está guardado
};

// Nombres legibles de los campos "extra" que devuelve cada conector
const EXTRA_LABELS = {
  rango: "Rango",
  numero_oficial: "Número oficial",
  departamento: "Departamento",
  consolidacion: "Consolidación",
  vigencia: "Vigencia",
  actualizada: "Última actualización",
  publicador: "Publicador",
  tematica: "Temática",
  palabras_clave: "Palabras clave",
};

// ── Utilidades ──

// Llama a la API y devuelve { ok, status, data } sin lanzar excepciones por errores HTTP.
async function api(path, options = {}) {
  try {
    const res = await fetch(path, options);
    const data = res.status === 204 ? null : await res.json().catch(() => null);
    return { ok: res.ok, status: res.status, data };
  } catch (err) {
    return { ok: false, status: 0, data: { error: "No se pudo conectar con el servidor" } };
  }
}

function setStatus(el, text, type = "") {
  el.textContent = text;
  el.className = "status " + type;
}

// "2007-07-12" -> "12/07/2007" (sin pasar por Date, para no tener problemas de zona horaria)
function formatDate(iso) {
  if (!/^\d{4}-\d{2}-\d{2}/.test(iso || "")) return "";
  const [y, m, d] = iso.slice(0, 10).split("-");
  return `${d}/${m}/${y}`;
}

// Solo se enlazan URLs http(s)
function safeURL(url) {
  return /^https?:\/\//i.test(url || "") ? url : null;
}

function sourceLabel(name) {
  return state.sources[name] || name;
}

function fillCommon(node, item) {
  const badge = node.querySelector(".badge");
  badge.textContent = sourceLabel(item.source);
  badge.dataset.source = item.source;

  const time = node.querySelector(".date");
  time.textContent = formatDate(item.date);
  if (item.date) time.dateTime = item.date;

  node.querySelector(".title").textContent = item.title;
  node.querySelector(".summary").textContent = item.summary || "";

  const link = node.querySelector(".official");
  const url = safeURL(item.url);
  if (url) link.href = url;
  else link.remove();
}

function isSaved(result) {
  return state.savedAll.some((s) => s.source === result.source && s.external_id === result.external_id);
}

// ── Pestañas ──

function showTab(name) {
  for (const tab of ["search", "saved"]) {
    const active = tab === name;
    $(`#tab-${tab}`).setAttribute("aria-selected", String(active));
    $(`#panel-${tab}`).hidden = !active;
  }
  if (name === "saved") loadSaved();
}

// ── Fuentes ──

async function loadSources() {
  const { ok, data } = await api("/api/sources");
  if (!ok || !Array.isArray(data)) return;
  for (const s of data) {
    state.sources[s.name] = s.label;
    for (const select of [$("#source"), $("#f-source")]) {
      const opt = document.createElement("option");
      opt.value = s.name;
      opt.textContent = s.label;
      select.append(opt);
    }
  }
}

// ── Búsqueda ──

async function search(event) {
  event.preventDefault();
  const q = $("#q").value.trim();
  if (!q) return;

  const status = $("#search-status");
  const button = $("#search-form button");
  button.disabled = true;
  setStatus(status, "Buscando…");
  $("#results").replaceChildren();
  $("#source-errors").replaceChildren();

  const params = new URLSearchParams({ q, source: $("#source").value });
  const { ok, status: code, data } = await api(`/api/search?${params}`);
  button.disabled = false;

  if (!data || (!ok && code !== 502)) {
    setStatus(status, data?.error || "Error al buscar", "error");
    return;
  }

  const errors = data.errors || [];
  const results = data.results || [];
  renderSourceErrors(errors, code !== 502);
  if (code === 502) {
    setStatus(status, "No se pudo consultar ninguna fuente. Inténtalo de nuevo en unos minutos.", "error");
  } else {
    setStatus(status, results.length
      ? `${results.length} resultado${results.length === 1 ? "" : "s"} para «${data.query}»`
      : `Sin resultados para «${data.query}»`);
  }
  renderResults(results);
}

// othersWorked indica si alguna otra fuente sí respondió.
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
  const list = $("#results");
  list.replaceChildren();
  for (const r of results) {
    list.append(resultCard(r));
  }
}

function resultCard(result) {
  const node = $("#result-template").content.firstElementChild.cloneNode(true);
  fillCommon(node, result);

  // Detalle: campos extra de la fuente, en el orden de EXTRA_LABELS y después el resto
  const detail = node.querySelector(".detail");
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

  const btnDetail = node.querySelector(".btn-detail");
  btnDetail.addEventListener("click", () => {
    detail.hidden = !detail.hidden;
    btnDetail.setAttribute("aria-expanded", String(!detail.hidden));
    btnDetail.textContent = detail.hidden ? "Ver detalle" : "Ocultar detalle";
  });

  // Guardar: abre un pequeño formulario para etiquetas y nota
  const btnSave = node.querySelector(".btn-save");
  const form = node.querySelector(".save-form");
  const markSaved = () => {
    btnSave.textContent = "Guardado ✓";
    btnSave.disabled = true;
    form.hidden = true;
  };
  if (isSaved(result)) markSaved();

  btnSave.addEventListener("click", () => {
    form.hidden = false;
    form.elements.tags.focus();
  });
  node.querySelector(".btn-cancel").addEventListener("click", () => { form.hidden = true; });

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const payload = {
      ...result,
      tags: form.elements.tags.value.split(",").map((t) => t.trim()).filter(Boolean),
      note: form.elements.note.value.trim(),
    };
    const submit = form.querySelector('button[type="submit"]');
    submit.disabled = true;
    const { ok, status, data } = await api("/api/saved", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    submit.disabled = false;

    if (ok || status === 409) {
      markSaved();
      await refreshSavedAll();
      setStatus($("#search-status"), status === 409 ? "Ya estaba guardado." : "Guardado en tu lista.", "ok");
    } else {
      setStatus($("#search-status"), `No se pudo guardar: ${data?.error || "error " + status}`, "error");
    }
  });

  return node;
}

// ── Guardados ──

// Pide todos los guardados (sin filtros) para el contador y el desplegable de etiquetas.
async function refreshSavedAll() {
  const { ok, data } = await api("/api/saved");
  if (!ok || !Array.isArray(data)) return;
  state.savedAll = data;
  $("#saved-count").textContent = data.length;
  fillTagFilter();
}

function fillTagFilter() {
  const select = $("#f-tag");
  const current = select.value;
  const tags = [...new Set(state.savedAll.flatMap((s) => s.tags || []))].sort((a, b) => a.localeCompare(b, "es"));
  select.replaceChildren(new Option("Todas", ""));
  for (const t of tags) select.append(new Option(t, t));
  select.value = tags.includes(current) ? current : "";
}

// Carga la lista aplicando los filtros del formulario (los filtra el servidor).
async function loadSaved() {
  const form = $("#filter-form");
  const params = new URLSearchParams();
  for (const name of ["source", "tag", "from", "to"]) {
    const v = form.elements[name].value;
    if (v) params.set(name, v);
  }
  const status = $("#saved-status");
  const { ok, data } = await api(`/api/saved?${params}`);
  if (!ok || !Array.isArray(data)) {
    setStatus(status, data?.error || "No se pudo cargar la lista", "error");
    return;
  }
  const filtered = params.toString() !== "";
  setStatus(status, data.length
    ? `${data.length} elemento${data.length === 1 ? "" : "s"}${filtered ? " con estos filtros" : ""}`
    : filtered ? "Ningún elemento cumple estos filtros." : "Todavía no has guardado nada. Busca algo y pulsa «Guardar».");
  renderSaved(data);
}

function renderSaved(items) {
  const list = $("#saved-list");
  list.replaceChildren();
  for (const item of items) {
    const node = $("#saved-template").content.firstElementChild.cloneNode(true);
    fillCommon(node, item);

    const tags = node.querySelector(".tags");
    for (const t of item.tags || []) {
      const li = document.createElement("li");
      li.textContent = t;
      tags.append(li);
    }
    node.querySelector(".note").textContent = item.note || "";
    node.querySelector(".saved-at").textContent = "Guardado el " + formatDate(item.saved_at);

    node.querySelector(".btn-delete").addEventListener("click", async () => {
      if (!confirm(`¿Borrar «${item.title}» de tu lista?`)) return;
      const { ok, data } = await api(`/api/saved/${encodeURIComponent(item.id)}`, { method: "DELETE" });
      if (!ok) {
        setStatus($("#saved-status"), data?.error || "No se pudo borrar", "error");
        return;
      }
      await refreshSavedAll();
      await loadSaved();
    });
    list.append(node);
  }
}

// ── Arranque ──

$("#tab-search").addEventListener("click", () => showTab("search"));
$("#tab-saved").addEventListener("click", () => showTab("saved"));
$("#search-form").addEventListener("submit", search);
$("#filter-form").addEventListener("change", loadSaved);
$("#filter-form").addEventListener("reset", () => setTimeout(loadSaved)); // tras limpiar los campos

loadSources().then(refreshSavedAll);
