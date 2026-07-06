// Cloudflare Worker — legacy `jabali update` bridge: 301-redirects the jabali2
// paths on git.jabali-panel.com to codeberg.org.
//
// Deployed hosts have git.jabali-panel.com compiled in (git `origin` remote +
// release API base). `jabali update` does two ops there: (1) `git fetch origin
// main` (+ reset to origin/main or the `stable` tag) over git smart-HTTP, and
// (2) release tarball metadata via /api/v1/.../releases. We 301-redirect BOTH
// to Codeberg. git follows the info/refs redirect and remaps upload-pack to the
// new host; Go's http client follows the release-API redirect. A redirect (vs
// proxying) sends the host straight to Codeberg — no packfile/tarball bytes
// through the Worker.
//
// SCOPED to jabali2 paths only — the old Gitea still hosts other repos on this
// domain; a catch-all redirect would break them.

const PREFIXES = [
  "/api/v1/repos/shukivaknin/jabali2/", // release API
  "/shukivaknin/jabali2.git/",          // git smart-HTTP
  "/shukivaknin/jabali2/info/refs",     // info/refs (no .git suffix form)
];

export default {
  async fetch(request) {
    const url = new URL(request.url);
    if (!PREFIXES.some((p) => url.pathname.startsWith(p))) {
      return new Response("jabali update bridge: path not redirected\n", {
        status: 404,
        headers: { "content-type": "text/plain" },
      });
    }
    // 301 to the same path on Codeberg, preserving query (git-upload-pack
    // params etc.). git + the Go updater both follow.
    return Response.redirect("https://codeberg.org" + url.pathname + url.search, 301);
  },
};
