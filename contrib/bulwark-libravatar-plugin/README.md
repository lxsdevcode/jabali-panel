# Bulwark Libravatar plugin → submitted upstream

The Libravatar avatar plugin for [Bulwark webmail](https://github.com/bulwarkmail/webmail)
now lives in Bulwark's plugin marketplace repo, submitted as a PR:

**https://github.com/bulwarkmail/plugins/pull/3**

Source on the fork:
https://github.com/shukiv/plugins/tree/add-libravatar-plugin/libravatar

## Background

This came out of jabali #214 (mailbox avatars). The first attempt patched
`components/ui/avatar.tsx` in webmail core (PR
[bulwarkmail/webmail#447](https://github.com/bulwarkmail/webmail/pull/447)),
but upstream added a first-class `onAvatarResolve` plugin hook (Gravatar is
the bundled example), so #447 was closed and the feature reworked as a
proper `hook`-type plugin modelled on the official `gravatar` plugin:
SHA-256 email hash → `seccdn.libravatar.org`, HEAD existence check with
`d=404` fallthrough, persistent cache, configurable size + fallback style.

Caveat: client-sandboxed plugins can't do Libravatar's DNS-SRV delegation,
so it uses the central CDN only. Not yet load-tested on a live Bulwark
instance — pending maintainer review on the PR.
