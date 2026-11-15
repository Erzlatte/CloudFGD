import { getSources } from "./api.js";
import { state } from "./state.js";
import { $ } from "./dom.js";
import { initTabs, onTabShown } from "./tabs.js";
import { initSearch } from "./search.js";
import { initSaved, loadSaved, refreshSavedAll } from "./saved.js";

async function loadSources() {
  const { ok, data } = await getSources();
  if (!ok || !Array.isArray(data)) return;

  for (const s of data) {
    state.sources[s.name] = s.label;
    for (const select of [$("#source"), $("#f-source")]) {
      select.append(new Option(s.label, s.name));
    }
  }
}

initTabs();
initSearch();
initSaved();
onTabShown("saved", loadSaved);

loadSources().then(refreshSavedAll);
