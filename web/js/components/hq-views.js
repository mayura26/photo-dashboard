// Light-DOM custom elements that host the original App.register view paint functions.
// Importing the view modules registers them on window.App; these elements are the mount points.

function defineViewHost(tag, viewName) {
  class ViewHost extends HTMLElement {
    connectedCallback() {
      this.dataset.view = viewName;
    }
  }
  customElements.define(tag, ViewHost);
}

defineViewHost('hq-home', 'home');
defineViewHost('hq-calendar', 'calendar');
defineViewHost('hq-tasks', 'tasks');
defineViewHost('hq-shopping', 'shopping');
defineViewHost('hq-photos', 'photos');
defineViewHost('hq-control', 'control');
defineViewHost('hq-agents', 'agents');

export class HqAmbient extends HTMLElement {
  connectedCallback() {
    this.classList.add('ambient');
  }
}
customElements.define('hq-ambient', HqAmbient);
