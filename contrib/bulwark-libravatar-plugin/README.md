# Bulwark Libravatar plugin

A [Bulwark webmail](https://github.com/bulwarkmail/webmail) plugin that
resolves sender/recipient avatars from [Libravatar](https://www.libravatar.org/)
— a federated, privacy-friendly Gravatar alternative — via the host's
`onAvatarResolve` hook.

## Why a plugin (and not a core patch)

The original approach (jabali #214, Bulwark PR
[bulwarkmail/webmail#447](https://github.com/bulwarkmail/webmail/pull/447))
wired Libravatar straight into `components/ui/avatar.tsx`. Upstream has
since added a first-class **`onAvatarResolve` plugin hook** (with Gravatar
as the documented example), so the core patch is superseded — this plugin
uses that hook instead. PR #447 should be closed in favour of this.

## How it works

The host calls the `onAvatarResolve` transform hook with
`(currentUrl, { email, name })` and uses the returned URL as the avatar
`<img src>` (host priority: contact photo → plugin avatar → custom avatar →
profile picture → favicon → initials). The plugin:

1. respects any URL an earlier plugin already resolved (`current`),
2. otherwise SHA-256-hashes the lowercased, trimmed email,
3. returns `https://seccdn.libravatar.org/avatar/<sha256>?d=404&s=160`.

`d=404` makes Libravatar return 404 when an address has no avatar, so the
host's `<img>` `onError` falls through to favicon/initials.

## Files

| File | Purpose |
|------|---------|
| `manifest.json` | plugin manifest (`type: hook`, `entrypoint: index.js`) |
| `index.js` | ES-module bundle exporting `{ hooks: { onAvatarResolve }, activate }` |

## Packaging & install

Bulwark plugins ship as a ZIP (manifest + entrypoint) submitted to the
Bulwark extension marketplace, which validates (and may sign) them at
install time. To build a local install zip:

```bash
cd contrib/bulwark-libravatar-plugin
zip -r ../libravatar-plugin.zip manifest.json index.js
```

Then install via the webmail admin → Plugins UI (or the marketplace once
published).

## Caveats / TODO before submitting

- **Not loaded-tested here.** Authored against upstream's plugin types
  (`lib/plugin-types.ts`, `lib/plugin-sandbox/*`); needs a real Bulwark
  instance to verify it loads, registers the hook, and renders avatars.
- **Host CSP `img-src`** must permit `https://seccdn.libravatar.org`. The
  Gravatar example implies the host allows avatar-CDN image sources; confirm
  on the target instance (or have the marketplace declare it).
- **No DNS-SRV federation.** The deprecated core approach did a server-side
  SRV lookup to honour a domain's self-hosted Libravatar server. Plugins run
  in a client sandbox with no DNS, so this uses the public
  `seccdn.libravatar.org` CDN only — which still covers the common case and
  federates server-side avatars that opt into the central CDN.
- Optional: add `icon`/`screenshots` to the manifest for the marketplace
  card, and a `settingsSchema` toggle if a per-user on/off switch is wanted
  (the host already lets users disable plugins).
