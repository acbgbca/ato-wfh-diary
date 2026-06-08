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
