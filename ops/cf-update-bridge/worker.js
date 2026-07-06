// Cloudflare Worker — legacy `jabali update` release-API bridge.
//
// Deployed hosts have git.jabali-panel.com compiled into their binary
// (panel-api/cmd/server/update_release.go defaultReleaseAPIBase). After the
// forge move to Codeberg, this Worker sits on git.jabali-panel.com and
// transparently forwards the *release API* to codeberg.org, so existing hosts
// keep updating without needing to update first (the chicken-and-egg fix).
//
// Why this is enough: update_release.go only calls, against that base:
//   GET /releases/tags/<tag>     (release metadata)
//   GET /branches/main           (latest sha -> release-<short_sha>)
//   GET /releases?limit=20       (fallback listing)
// The tarball/sha256 assets are fetched from each asset's absolute
// browser_download_url, which Codeberg returns as https://codeberg.org/... —
// so hosts pull the (large) tarballs straight from Codeberg and this Worker
// only proxies the small JSON. Bandwidth stays off Cloudflare.
//
// Self-liquidating: the first bridged update hands the host a binary whose
// defaultReleaseAPIBase already points at codeberg.org, so subsequent updates
// bypass the bridge entirely. Once the fleet has rolled over, the Worker + the
// git.jabali-panel.com DNS can be retired.

const REPO_PREFIX = "/api/v1/repos/shukivaknin/jabali2/";
const UPSTREAM = "https://codeberg.org";

export default {
  async fetch(request) {
    const url = new URL(request.url);

    // Only the jabali2 release API is bridged. Everything else 404s (the old
    // Gitea web UI / git endpoints are intentionally not proxied — the default
    // tarball update path doesn't need them; --from-source still needs the old
    // git server if kept).
    if (!url.pathname.startsWith(REPO_PREFIX)) {
      return new Response("jabali update bridge: only the release API is proxied\n", {
        status: 404,
        headers: { "content-type": "text/plain" },
      });
    }

    // GET/HEAD only — the updater never writes.
    if (request.method !== "GET" && request.method !== "HEAD") {
      return new Response("method not allowed\n", { status: 405 });
    }

    const target = UPSTREAM + url.pathname + url.search;
    const upstream = await fetch(target, {
      method: request.method,
      headers: {
        "User-Agent": "jabali-update-bridge",
        Accept: "application/json",
      },
      // Codeberg's public release API needs no auth for a public repo.
    });

    const headers = new Headers(upstream.headers);
    headers.set("X-Jabali-Bridge", "codeberg");
    headers.delete("set-cookie");
    return new Response(upstream.body, { status: upstream.status, headers });
  },
};
