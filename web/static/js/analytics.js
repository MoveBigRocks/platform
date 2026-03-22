(function() {
  'use strict';
  var endpoint = new URL(document.currentScript.src).origin + '/api/analytics/event';
  var domain = document.currentScript.getAttribute('data-domain');

  if (/^localhost$|^127\.|^0\.0\.0\.0$/.test(location.hostname) || location.protocol === 'file:') return;
  if (window._phantom || window.__nightmare || navigator.webdriver) return;

  var lastPage;

  function trigger(name) {
    if (lastPage === location.pathname && name === 'pageview') return;
    if (name === 'pageview') lastPage = location.pathname;

    fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body: JSON.stringify({
        n: name,
        u: location.href,
        d: domain,
        r: document.referrer || null
      }),
      keepalive: true
    });
  }

  trigger('pageview');

  var originalPush = history.pushState;
  history.pushState = function() {
    originalPush.apply(this, arguments);
    trigger('pageview');
  };
  window.addEventListener('popstate', function() { trigger('pageview'); });

  window.mbr = window.mbr || {};
  window.mbr.track = function(name) { trigger(name); };
})();
