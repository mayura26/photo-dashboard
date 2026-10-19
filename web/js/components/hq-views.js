// Light-DOM custom elements that host the original App.register view paint functions.

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
defineViewHost('hq-body', 'body');
defineViewHost('hq-payouts', 'payouts');

export class HqAmbient extends HTMLElement {
  connectedCallback() {
    this.classList.add('ambient');
  }
}
customElements.define('hq-ambient', HqAmbient);
