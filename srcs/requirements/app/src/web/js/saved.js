import { deleteSaved, listSaved } from "./api.js";
import { state } from "./state.js";
import { $, cloneTemplate, fillCard, formatDate, plural, setStatus } from "./dom.js";

const FILTERS = ["source", "tag", "from", "to"];

export function initSaved() {
  const form = $("#filter-form");
  form.addEventListener("change", loadSaved);
  form.addEventListener("reset", () => setTimeout(loadSaved));
}

export async function refreshSavedAll() {
  const { ok, data } = await listSaved();
  if (!ok || !Array.isArray(data)) return;
  state.savedAll = data;
  $("#saved-count").textContent = data.length;
  fillTagFilter();
}

export async function loadSaved() {
  const form = $("#filter-form");
  const filters = {};
  for (const name of FILTERS) {
    const value = form.elements[name].value;
    if (value) filters[name] = value;
  }

  const status = $("#saved-status");
  const { ok, data } = await listSaved(filters);
  if (!ok || !Array.isArray(data)) {
    setStatus(status, data?.error || "No se pudo cargar la lista", "error");
    return;
  }

  const filtered = Object.keys(filters).length > 0;
  if (data.length) {
    setStatus(status, plural(data.length, "elemento") + (filtered ? " con estos filtros" : ""));
  } else if (filtered) {
    setStatus(status, "Ningún elemento cumple estos filtros.");
  } else {
    setStatus(status, "Todavía no has guardado nada. Busca algo y pulsa «Guardar».");
  }
  renderSaved(data);
}

function fillTagFilter() {
  const select = $("#f-tag");
  const current = select.value;
  const tags = [...new Set(state.savedAll.flatMap((s) => s.tags || []))]
    .sort((a, b) => a.localeCompare(b, "es"));

  select.replaceChildren(new Option("Todas", ""));
  for (const t of tags) select.append(new Option(t, t));
  select.value = tags.includes(current) ? current : "";
}

function renderSaved(items) {
  $("#saved-list").replaceChildren(...items.map(savedCard));
}

function savedCard(item) {
  const node = cloneTemplate("saved-template");
  fillCard(node, item);

  const tags = $(".tags", node);
  for (const t of item.tags || []) {
    const li = document.createElement("li");
    li.textContent = t;
    tags.append(li);
  }
  $(".note", node).textContent = item.note || "";
  $(".saved-at", node).textContent = "Guardado el " + formatDate(item.saved_at);

  $(".btn-delete", node).addEventListener("click", async () => {
    if (!confirm(`¿Borrar «${item.title}» de tu lista?`)) return;
    const { ok, data } = await deleteSaved(item.id);
    if (!ok) {
      setStatus($("#saved-status"), data?.error || "No se pudo borrar", "error");
      return;
    }
    await refreshSavedAll();
    await loadSaved();
  });

  return node;
}
