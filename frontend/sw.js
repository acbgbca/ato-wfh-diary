const CACHE = 'wfh-diary-v1';
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

  e.respondWith(
    caches.match(e.request).then(cached => cached || fetch(e.request))
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
