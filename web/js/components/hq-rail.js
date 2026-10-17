const ICONS = {
  home: '<path d="M3 11l9-7 9 7"/><path d="M5 10v10h14V10"/>',
  calendar: '<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>',
  tasks: '<rect x="3" y="3" width="18" height="18" rx="4"/><path d="M7.5 12l3 3 6-6"/>',
  cart: '<path d="M3 4h2.2l2.3 11h10.8L21 8H6.5"/><circle cx="10" cy="19.5" r="1.5"/><circle cx="17" cy="19.5" r="1.5"/>',
  photo: '<rect x="3" y="5" width="18" height="14" rx="2"/><circle cx="9" cy="10" r="2"/><path d="M21 16l-5-5-8 8"/>',
  sliders: '<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
  agents: '<rect x="4" y="8" width="16" height="12" rx="3"/><path d="M12 4v4M9.5 13v1.5M14.5 13v1.5M2 13v3M22 13v3"/>'
};

function svg(paths) {
  return `<svg class="i" width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${paths}</svg>`;
}

export class HqRail extends HTMLElement {
  connectedCallback() {
    if (this._built) return;
    this._built = true;
    this.innerHTML =
      `<button class="rail-item" data-go="home">${svg(ICONS.home)}<span>Today</span></button>` +
      `<button class="rail-item" data-go="calendar">${svg(ICONS.calendar)}<span>Calendar</span></button>` +
      `<button class="rail-item" data-go="tasks">${svg(ICONS.tasks)}<span>Chores</span></button>` +
      `<button class="rail-item" data-go="shopping">${svg(ICONS.cart)}<span>Shopping</span></button>` +
      `<button class="rail-item" data-go="photos">${svg(ICONS.photo)}<span>Photos</span></button>` +
      `<button class="rail-item" data-go="control">${svg(ICONS.sliders)}<span>Home</span></button>` +
      `<button class="rail-item" data-go="agents">${svg(ICONS.agents)}<span>Agents</span></button>` +
      `<button class="rail-clock" id="railClock" aria-label="Clock — press and hold for demo controls"></button>`;
  }
}

customElements.define('hq-rail', HqRail);
