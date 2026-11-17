const TABS = ["search", "saved", "help"];
const onShow = {};

export function onTabShown(name, callback) {
  onShow[name] = callback;
}

export function showTab(name) {
  for (const tab of TABS) {
    const active = tab === name;
    document.getElementById(`tab-${tab}`).setAttribute("aria-selected", String(active));
    document.getElementById(`panel-${tab}`).hidden = !active;
  }
  if (onShow[name]) onShow[name]();
}

export function initTabs() {
  for (const tab of TABS) {
    document.getElementById(`tab-${tab}`).addEventListener("click", () => showTab(tab));
  }
  for (const el of document.querySelectorAll("[data-open-tab]")) {
    el.addEventListener("click", () => showTab(el.dataset.openTab));
  }
}
