export const state = {
  sources: {},
  savedAll: [],
};

export const EXTRA_LABELS = {
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

export function sourceLabel(name) {
  return state.sources[name] || name;
}

export function isSaved(result) {
  return state.savedAll.some(
    (s) => s.source === result.source && s.external_id === result.external_id,
  );
}
