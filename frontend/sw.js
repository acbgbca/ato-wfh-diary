// Cache name includes the build hash so a new deployment automatically
// invalidates the old cache and re-fetches all assets.
const CACHE = 'wfh-diary-{{.BuildHash}}';
const ASSETS = ['/', '/css/app.css', '/js/app.js', '/js/api.js', '/manifest.json', '/icons/icon.svg'];

self.addEventListener('install', e => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(ASSETS)));
  self.skipWaiting();
});

self.addEventListener('activate', e => {
  e.waitUntil(
    caches.keys().then(keys =>
      Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k)))
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', e => {
  // Always fetch API calls from the network
  if (new URL(e.request.url).pathname.startsWith('/api/')) return;

  // Network-first for top-level navigations so that when the reverse-proxy auth
  // session (Authelia) has expired, the proxy's redirect to the login page
  // actually reaches the network instead of being short-circuited by the cached
  // app shell. Serving the cached '/' here would swallow the reauthentication
  // redirect, leaving the PWA stuck showing the shell with zero data. Fall back
  // to the cached shell only when the network is unreachable (offline).
  if (e.request.mode === 'navigate') {
    e.respondWith(
      fetch(e.request).catch(() => caches.match('/', { ignoreSearch: true }))
    );
    return;
  }

  e.respondWith(
    // ignoreSearch so versioned URLs like /js/app.js?v=abc123 match the
    // bare-URL entries stored in the cache at install time.
    caches.match(e.request, { ignoreSearch: true }).then(cached => cached || fetch(e.request))
  );
});

self.addEventListener('push', e => {
  const data = e.data ? e.data.json() : {};
  const title = data.title || 'WFH Diary';
  const options = {
    body: data.body || 'Time to log your hours for this week',
    data: { weekStart: data.week_start },
  };
  e.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', e => {
  e.notification.close();
  const weekStart = e.notification.data && e.notification.data.weekStart;
  const url = weekStart ? `/?week=${weekStart}` : '/';
  e.waitUntil(clients.openWindow(url));
});

// The browser can invalidate a push subscription while the install is still in
// place — a key rotation, or a long stretch of inactivity. It fires
// pushsubscriptionchange when it does. Without this handler the server keeps
// pushing to an endpoint the push service no longer recognises, and the user
// silently receives nothing until the app next starts and re-registers.
self.addEventListener('pushsubscriptionchange', e => {
  e.waitUntil(resubscribe(e.oldSubscription));
});

// Creates a fresh push subscription and swaps it for the invalidated one on the
// server. Errors reject the waitUntil promise: the app re-registers on its next
// start, and the server prunes the dead endpoint the first time a send to it
// fails, so a failure here costs at most a missed notification.
async function resubscribe(oldSubscription) {
  const { vapid_public_key: vapidKey } = await (await fetch('/api/notifications/vapid-key')).json();

  const sub = await self.registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(vapidKey),
  });

  const json = sub.toJSON();
  await fetch('/api/notifications/subscribe', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      endpoint:   json.endpoint,
      p256dh_key: json.keys.p256dh,
      auth_key:   json.keys.auth,
    }),
  });

  // Not every browser supplies oldSubscription, so this is best-effort cleanup.
  if (oldSubscription && oldSubscription.endpoint) {
    await fetch('/api/notifications/subscribe', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint: oldSubscription.endpoint }),
    });
  }
}

// Converts a URL-safe base64 string to a Uint8Array for the Push API.
// Duplicated from js/app.js — a service worker cannot import from the page.
function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  return Uint8Array.from([...raw].map(c => c.charCodeAt(0)));
}
