// Lógica del frontend: carga los mensajes desde la API y envía nuevos sin recargar la página.
"use strict";

const API = "/api/messages";
const MAX_CONTENT = 500;

const form = document.getElementById("message-form");
const authorInput = document.getElementById("author");
const contentInput = document.getElementById("content");
const counter = document.getElementById("counter");
const statusEl = document.getElementById("status");
const list = document.getElementById("messages");
const total = document.getElementById("total");
const template = document.getElementById("message-template");

const dateFormat = new Intl.DateTimeFormat("es-ES", {
  dateStyle: "short",
  timeStyle: "short",
});

function setStatus(text, type = "") {
  statusEl.textContent = text;
  statusEl.className = "status " + type;
}

// Pinta la lista de mensajes. Se usa textContent (nunca innerHTML) para evitar XSS.
function render(messages) {
  list.replaceChildren();
  total.textContent = messages.length ? `(${messages.length})` : "";

  if (messages.length === 0) {
    const p = document.createElement("p");
    p.className = "empty";
    p.textContent = "Aún no hay mensajes. ¡Sé el primero!";
    list.append(p);
    return;
  }

  for (const m of messages) {
    const node = template.content.cloneNode(true);
    const date = new Date(m.created_at);
    node.querySelector(".msg-author").textContent = m.author;
    node.querySelector(".msg-content").textContent = m.content;
    const time = node.querySelector(".msg-date");
    time.dateTime = m.created_at;
    time.textContent = dateFormat.format(date);
    list.append(node);
  }
}

async function loadMessages() {
  try {
    const res = await fetch(API);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    render(await res.json());
  } catch (err) {
    list.innerHTML = "";
    setStatus("No se pudieron cargar los mensajes.", "error");
    console.error(err);
  }
}

async function sendMessage(event) {
  event.preventDefault();
  const payload = {
    author: authorInput.value.trim(),
    content: contentInput.value.trim(),
  };
  if (!payload.author || !payload.content) {
    setStatus("Rellena el nombre y el mensaje.", "error");
    return;
  }

  const button = form.querySelector("button");
  button.disabled = true;
  try {
    const res = await fetch(API, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    contentInput.value = "";
    updateCounter();
    setStatus("Mensaje enviado.", "ok");
    await loadMessages();
  } catch (err) {
    setStatus(`Error: ${err.message}`, "error");
  } finally {
    button.disabled = false;
  }
}

function updateCounter() {
  counter.textContent = `${contentInput.value.length} / ${MAX_CONTENT}`;
}

form.addEventListener("submit", sendMessage);
contentInput.addEventListener("input", updateCounter);
loadMessages();
