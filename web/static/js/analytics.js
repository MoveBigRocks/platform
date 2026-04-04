(function() {
  'use strict';

  var current = document.currentScript;
  var endpoint = new URL(current.src).origin + '/api/analytics/event';
  var domain = current.getAttribute('data-domain');
  var api = window.mbr || {};
  var state = {
    hashBasedRouting: !!api.hashBasedRouting,
    props: normalizeProps(api.props),
    location: normalizeLocation(api.location)
  };

  if (/^localhost$|^127\.|^0\.0\.0\.0$/.test(location.hostname) || location.protocol === 'file:') return;
  if (window._phantom || window.__nightmare || navigator.webdriver) return;

  var lastPage;

  function normalizeString(value, max) {
    if (value === null || value === undefined) return '';
    value = String(value).trim();
    if (!value) return '';
    return value.length > max ? value.slice(0, max) : value;
  }

  function normalizeProps(input) {
    if (!input || typeof input !== 'object') return null;
    var keys = Object.keys(input);
    if (!keys.length) return null;
    var output = {};
    for (var i = 0; i < keys.length && i < 32; i++) {
      var key = normalizeString(keys[i], 64);
      if (!key) continue;
      var value = input[keys[i]];
      if (value === null || value === undefined || typeof value === 'object') continue;
      var rendered = normalizeString(value, 256);
      if (!rendered) continue;
      output[key] = rendered;
    }
    return Object.keys(output).length ? output : null;
  }

  function normalizeLocation(input) {
    if (!input || typeof input !== 'object') return null;
    var locationOverride = {};
    var country = normalizeString(input.country, 8).toUpperCase();
    var region = normalizeString(input.region, 120);
    var city = normalizeString(input.city, 120);
    if (country) locationOverride.country = country;
    if (region) locationOverride.region = region;
    if (city) locationOverride.city = city;
    return Object.keys(locationOverride).length ? locationOverride : null;
  }

  function normalizeRevenue(input) {
    if (!input || typeof input !== 'object') return null;
    var amount = Number(input.amount);
    var currency = normalizeString(input.currency, 16).toUpperCase();
    if (!isFinite(amount) || amount < 0 || !currency) return null;
    return { amount: amount, currency: currency };
  }

  function mergeProps(base, extra) {
    if (!base && !extra) return null;
    var merged = {};
    var key;
    if (base) {
      for (key in base) merged[key] = base[key];
    }
    if (extra) {
      for (key in extra) merged[key] = extra[key];
    }
    return Object.keys(merged).length ? merged : null;
  }

  function resolveURL(options) {
    var raw = options && typeof options.url === 'string' && options.url ? options.url : location.href;
    try {
      return new URL(raw, location.href).toString();
    } catch (err) {
      return location.href;
    }
  }

  function pageKey(rawURL) {
    try {
      var url = new URL(rawURL, location.href);
      return url.pathname + url.search + (state.hashBasedRouting ? url.hash : '');
    } catch (err) {
      return rawURL;
    }
  }

  function mergeState(options) {
    if (!options || typeof options !== 'object') return;
    if (Object.prototype.hasOwnProperty.call(options, 'hashBasedRouting')) {
      state.hashBasedRouting = !!options.hashBasedRouting;
    }
    if (Object.prototype.hasOwnProperty.call(options, 'props')) {
      state.props = mergeProps(state.props, normalizeProps(options.props));
    }
    if (Object.prototype.hasOwnProperty.call(options, 'location')) {
      state.location = normalizeLocation(options.location);
    }
  }

  function trigger(name, options) {
    options = options || {};
    if (!name) return;

    var rawURL = resolveURL(options);
    if (name === 'pageview') {
      var nextPage = pageKey(rawURL);
      if (lastPage === nextPage) return;
      lastPage = nextPage;
    }

    var payload = {
      n: name,
      u: rawURL,
      d: domain,
      r: typeof options.referrer === 'string' ? options.referrer : (document.referrer || null)
    };

    var props = mergeProps(state.props, normalizeProps(options.props));
    if (props) payload.h = props;

    var revenue = normalizeRevenue(options.revenue);
    if (revenue) payload.v = revenue;

    var locationOverride = normalizeLocation(options.location) || state.location;
    if (locationOverride) payload.l = locationOverride;

    fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body: JSON.stringify(payload),
      keepalive: true
    });
  }

  function handleHistoryChange() {
    trigger('pageview');
  }

  api.init = function(options) {
    mergeState(options);
    return api;
  };
  api.track = function(name, options) {
    trigger(name, options);
  };
  api.pageview = function(options) {
    trigger('pageview', options);
  };
  window.mbr = api;

  trigger('pageview');

  var originalPush = history.pushState;
  history.pushState = function() {
    originalPush.apply(this, arguments);
    handleHistoryChange();
  };

  var originalReplace = history.replaceState;
  history.replaceState = function() {
    originalReplace.apply(this, arguments);
    handleHistoryChange();
  };

  window.addEventListener('popstate', handleHistoryChange);
})();
