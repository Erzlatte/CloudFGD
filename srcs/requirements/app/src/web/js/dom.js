import { sourceLabel } from "./state.js";

export const $ = (selector, root = document) => root.querySelector(selector);

export function setStatus(el, text, type = "") {
  el.textContent = text;
  el.className = "status " + type;
}

export function formatDate(iso) {
  if (!/^\d{4}-\d{2}-\d{2}/.test(iso || "")) return "";
  const [y, m, d] = iso.slice(0, 10).split("-");
  return `${d}/${m}/${y}`;
}

export function safeURL(url) {
  return /^https?:\/\//i.test(url || "") ? url : null;
}

export function cloneTemplate(id) {
  return document.getElementById(id).content.firstElementChild.cloneNode(true);
}

export function fillCard(node, item) {
  const badge = $(".badge", node);
  badge.textContent = sourceLabel(item.source);
  badge.dataset.source = item.source;

  const time = $(".date", node);
  time.textContent = formatDate(item.date);
  if (item.date) time.dateTime = item.date;

  $(".title", node).textContent = item.title;
  $(".summary", node).textContent = item.summary || "";

  const link = $(".official", node);
  const url = safeURL(item.url);
  if (url) link.href = url;
  else link.remove();
}

export function plural(n, singular, pluralForm = singular + "s") {
  return `${n} ${n === 1 ? singular : pluralForm}`;
}
