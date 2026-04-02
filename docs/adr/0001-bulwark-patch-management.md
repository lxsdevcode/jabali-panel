# ADR-0001: Bulwark patch management via shared function with version markers

**Date**: 2026-04-02
**Status**: accepted
**Deciders**: Shuki Vaknin

## Context

Jabali embeds Bulwark Webmail (a third-party Next.js JMAP client) at `/opt/bulwark`. We apply several local patches to the Bulwark source tree before building: basePath (`/webmail`), locale routing, proxy.ts rewrite, client-side fetch path fixes, an SSO API route, and an auth-store fallback that checks the session cookie on page load.

The `upgrade_bulwark()` function runs `git reset --hard origin/main` to pull upstream updates, which wipes all local patches. Previously, the patches were only applied during initial install — upgrades silently lost them, breaking SSO and webmail routing. There was no mechanism to detect or recover from this.

Additionally, when our patches change (e.g. upstream changed the session GET endpoint to strip passwords, requiring us to switch the SSO fallback from GET to PUT), there was no way to force a rebuild without a coinciding upstream update.

## Decision

We extract all Bulwark patches into a single `patch_bulwark()` shell function in `install.sh`, called from both the initial install path and `upgrade_bulwark()`. A `jabali_patch_version` variable (currently `"3"`) is compared against a `.jabali-patch-version` marker file in the Bulwark directory. When the version changes, the upgrade function resets to clean upstream, re-applies patches, rebuilds, and restarts. The marker is written only after a successful build to avoid false-positive "already patched" states from failed builds.

## Alternatives Considered

### Alternative 1: Fork Bulwark and maintain patches in our own repo
- **Pros**: No runtime patching; clean git history; CI can test the fork
- **Cons**: Ongoing merge burden with upstream; need to host and maintain a separate repo; delays upstream security fixes
- **Why not**: Bulwark is actively developed and our patches are small (SSO route + config tweaks). The merge cost outweighs the benefit at our current patch count.

### Alternative 2: Use git format-patch / git am to apply patches
- **Pros**: Standard git workflow; patches are versioned files in our repo
- **Cons**: Patches break when upstream changes the target files; `git am` failures require manual resolution on the server; more complex than sed/cat for config-level changes
- **Why not**: Our patches mix file creation (SSO route), config edits (basePath), and code injection (auth-store). A shell function with idempotent checks is simpler and more reliable for this mix.

### Alternative 3: No version marker — always re-patch and rebuild on every upgrade
- **Pros**: Simpler logic; guaranteed fresh state
- **Cons**: Bulwark build takes ~60 seconds; rebuilding on every `jabali update` even when nothing changed wastes time and risks transient npm failures
- **Why not**: The version marker adds minimal complexity and avoids unnecessary rebuilds.

## Consequences

### Positive
- SSO and webmail routing survive Bulwark upstream updates automatically
- Changing a patch (e.g. GET → PUT for session) only requires bumping `jabali_patch_version` to propagate to all servers on next `jabali update`
- Failed builds don't leave a false "patched" marker — the next update retries

### Negative
- All patch logic lives in `install.sh` as shell code — harder to test than application code
- The auth-store patch uses Python string replacement which is fragile if upstream restructures that file significantly

### Risks
- Upstream Bulwark could rename/remove files our patches target (e.g. `stores/auth-store.ts`, `next.config.ts`). Mitigation: each patch step has `2>/dev/null || true` guards and the build still succeeds without the auth-store patch (SSO degrades gracefully to manual login).
