// registerServiceWorker — registers the single app service worker (/sw-push.js)
// on every load so the panel is an installable PWA (#434). The same SW also
// handles M14 Web Push; the push-subscribe flow re-registers the same URL
// idempotently, so this early registration never conflicts with it. Skipped on
// insecure origins / browsers without SW support (registration would throw).
//
// updateViaCache: "none" is LOAD-BEARING: without it the browser may serve the
// SW script itself from the HTTP cache (the panel's static assets ship
// `Cache-Control: public, immutable`), so a stale/broken SW keeps controlling
// the page across updates → every deploy changes the app-shell asset hashes, the
// old SW mishandles them, and the panel renders a blank screen. "none" forces
// the browser to always revalidate /sw-push.js, so a fixed SW actually reaches
// hosts. The controllerchange → one-shot reload makes the takeover seamless:
// when the new SW (skipWaiting + clientsClaim) assumes control, reload once so
// the page runs against fresh assets instead of half-old/half-new.
export function registerServiceWorker(): void {
  if (typeof navigator === "undefined" || !("serviceWorker" in navigator)) {
    return;
  }

  let reloading = false;
  navigator.serviceWorker.addEventListener("controllerchange", () => {
    if (reloading) return;
    reloading = true;
    window.location.reload();
  });

  window.addEventListener("load", () => {
    navigator.serviceWorker
      .register("/sw-push.js", { scope: "/", updateViaCache: "none" })
      .then((reg) => {
        // Force an immediate update check so a new SW is picked up without
        // waiting for the browser's periodic (up to 24h) revalidation.
        reg.update().catch(() => {});
      })
      .catch((err) => {
        // Cert-untrusted dev boxes / http origins reject registration — non-fatal.
        console.info("[pwa] service worker registration skipped", err);
      });
  });
}
