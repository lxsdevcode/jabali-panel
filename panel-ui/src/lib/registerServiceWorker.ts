// registerServiceWorker — registers the single app service worker (/sw-push.js)
// on every load so the panel is an installable PWA (#434). The same SW also
// handles M14 Web Push; the push-subscribe flow re-registers the same URL
// idempotently, so this early registration never conflicts with it. Skipped on
// insecure origins / browsers without SW support (registration would throw).
export function registerServiceWorker(): void {
  if (typeof navigator === "undefined" || !("serviceWorker" in navigator)) {
    return;
  }
  window.addEventListener("load", () => {
    navigator.serviceWorker
      .register("/sw-push.js", { scope: "/" })
      .catch((err) => {
        // Cert-untrusted dev boxes / http origins reject registration — non-fatal.
        console.info("[pwa] service worker registration skipped", err);
      });
  });
}
