// Libravatar avatar resolver — Bulwark webmail plugin (type: hook).
//
// Registers the `onAvatarResolve` transform hook. The host calls it with
// `(currentUrl, { email, name })` and uses the returned URL as the avatar
// image src (priority: contact photo > plugin avatar > custom > profile >
// favicon > initials). We return the Libravatar URL for the address; the
// `d=404` parameter makes Libravatar 404 when the address has no avatar, so
// the host's <img> onError falls through to the next source.
//
// Hashing: Libravatar accepts an MD5 or SHA-256 hex digest of the
// lowercased, trimmed email. We use SHA-256 (crypto.subtle) — no MD5 needed.

async function sha256Hex(input) {
  const data = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

// Transform hook: respect any higher-priority URL already resolved by an
// earlier plugin (`current`), otherwise resolve from Libravatar.
async function onAvatarResolve(current, ctx) {
  if (current) return current;
  const email = (ctx && ctx.email ? String(ctx.email) : "").trim().toLowerCase();
  if (!email || email.indexOf("@") === -1) return current;
  try {
    const hash = await sha256Hex(email);
    // s=160 covers retina rendering of the largest avatar; d=404 = fall
    // through when there's no avatar for this address.
    return "https://seccdn.libravatar.org/avatar/" + hash + "?d=404&s=160";
  } catch {
    return current;
  }
}

export const hooks = { onAvatarResolve };

export function activate() {
  // No setup required — the hook export above is all the host needs.
}
