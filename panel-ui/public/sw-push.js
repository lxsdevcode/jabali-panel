// sw-push.js — service worker for M14 Web Push delivery AND the #434 PWA
// app-shell (single SW so the two never fight over the / scope).
//
// Registered at /sw-push.js (scope = site root) from both the early PWA
// registration (registerServiceWorker, on every load) and the push-subscribe
// flow (idempotent on the same URL).
//
// The fetch handler is deliberately conservative: it ONLY touches same-origin
// GET navigations + static assets, NEVER /api/* (authenticated/dynamic — must
// not be cached). Navigations are network-first with an offline fallback to the
// cached shell; hashed static assets are stale-while-revalidate.

const SHELL_CACHE = "jabali-shell-v1";

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      try {
        const cache = await caches.open(SHELL_CACHE);
        await cache.add("/");
      } catch (_err) {
        // Offline at install / shell unavailable — non-fatal.
      }
      await self.skipWaiting();
    })(),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(
        keys
          .filter((k) => k.startsWith("jabali-shell-") && k !== SHELL_CACHE)
          .map((k) => caches.delete(k)),
      );
      await self.clients.claim();
    })(),
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  let url;
  try {
    url = new URL(req.url);
  } catch (_err) {
    return;
  }
  if (url.origin !== self.location.origin) return;
  // NEVER intercept authenticated / dynamic API traffic.
  if (url.pathname.startsWith("/api/")) return;

  // Navigations: network-first, fall back to the cached shell when offline.
  if (req.mode === "navigate") {
    event.respondWith(
      (async () => {
        try {
          return await fetch(req);
        } catch (_err) {
          const cache = await caches.open(SHELL_CACHE);
          const cached = (await cache.match("/")) || (await cache.match("/index.html"));
          return cached || Response.error();
        }
      })(),
    );
    return;
  }

  // Static same-origin assets (hashed, immutable): stale-while-revalidate.
  if (/\.(?:js|css|woff2?|png|jpe?g|svg|gif|webp|ico|json|webmanifest)$/.test(url.pathname)) {
    event.respondWith(
      (async () => {
        const cache = await caches.open(SHELL_CACHE);
        const cached = await cache.match(req);
        const network = fetch(req)
          .then((res) => {
            if (res && res.ok) cache.put(req, res.clone());
            return res;
          })
          .catch(() => cached);
        return cached || network;
      })(),
    );
  }
});

self.addEventListener("push", (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch (_err) {
    payload = {};
  }
  const title = payload.title || "Jabali Panel";
  const options = {
    body: payload.body || "",
    icon: "/favicon.svg",
    badge: "/favicon.svg",
    data: {
      deeplink: payload.deeplink || "/jabali-admin/notifications/channels",
      severity: payload.severity || "info",
    },
  };
  event.waitUntil(
    Promise.all([
      self.registration.showNotification(title, options),
      // Notify any open panel tabs so the in-app bell can
      // invalidate its inbox query immediately rather than waiting
      // for the 30s poll to catch up.
      self.clients
        .matchAll({ type: "window", includeUncontrolled: true })
        .then((list) => {
          for (const c of list) {
            c.postMessage({ type: "jabali/notification", payload });
          }
        }),
    ]),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const target = (event.notification.data && event.notification.data.deeplink) || "/";
  event.waitUntil(
    self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((list) => {
      for (const client of list) {
        if ("focus" in client) {
          client.navigate(target);
          return client.focus();
        }
      }
      if (self.clients.openWindow) {
        return self.clients.openWindow(target);
      }
      return undefined;
    }),
  );
});
