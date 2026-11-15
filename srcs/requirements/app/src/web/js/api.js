const ROUTES = {
  sources: "/api/sources",
  search: "/api/search",
  saved: "/api/saved",
  savedItem: (id) => `/api/saved/${encodeURIComponent(id)}`,
};

const NETWORK_ERROR = { error: "No se pudo conectar con el servidor" };

async function request(method, path, body) {
  const options = { method };
  if (body !== undefined) {
    options.headers = { "Content-Type": "application/json" };
    options.body = JSON.stringify(body);
  }

  try {
    const res = await fetch(path, options);
    const data = res.status === 204 ? null : await res.json().catch(() => null);
    return { ok: res.ok, status: res.status, data };
  } catch {
    return { ok: false, status: 0, data: NETWORK_ERROR };
  }
}

function withQuery(path, params) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== "") {
      query.set(key, value);
    }
  }
  const qs = query.toString();
  return qs ? `${path}?${qs}` : path;
}

export function getSources() {
  return request("GET", ROUTES.sources);
}

export function searchSources({ q, source = "all", limit } = {}) {
  return request("GET", withQuery(ROUTES.search, { q, source, limit }));
}

export function listSaved({ source, tag, from, to } = {}) {
  return request("GET", withQuery(ROUTES.saved, { source, tag, from, to }));
}

export function saveResult(result, { tags = [], note = "" } = {}) {
  return request("POST", ROUTES.saved, {
    source: result.source,
    external_id: result.external_id,
    title: result.title,
    summary: result.summary,
    date: result.date,
    url: result.url,
    extra: result.extra,
    tags,
    note,
  });
}

export function deleteSaved(id) {
  return request("DELETE", ROUTES.savedItem(id));
}
