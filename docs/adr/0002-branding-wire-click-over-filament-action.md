# ADR-0002: Native wire:click for Panel Branding instead of Filament FormAction

**Date**: 2026-04-02
**Status**: accepted
**Deciders**: Shuki Vaknin

## Context

The Server Settings page (`app/Filament/Admin/Pages/ServerSettings.php`) uses Filament v5's form schema for most settings. The Panel Branding section needs file upload buttons for light/dark logos and a text input for the panel name.

When we placed `FormAction` buttons inside the Filament form schema with `->action(fn (array $data) => ...)` closures, the closures silently never executed — no error, no server log, no exception. This affected both inline schema actions and header modal actions. Multiple approaches were tried over several hours (footer actions, hidden header actions with `mountAction()`, `Action::make()` in various positions). All failed with the same silent no-op behavior. This appears to be a Filament v5 bug specific to `FormAction` closures in custom page schema contexts.

## Decision

We bypass Filament's action system entirely for the branding section. The branding UI is rendered in the Blade view (`server-settings.blade.php`) after the Filament form, using native HTML file inputs, `<x-filament::button wire:click="...">` components, and Livewire's `WithFileUploads` trait. Logo uploads use `saveLivewireUpload()` which stores the file via `Storage::disk('public')->putFileAs()` and updates the setting. Logo previews use relative `/storage/...` URLs (not `asset()`) because `APP_URL` contains the server IP while users access via hostname.

## Alternatives Considered

### Alternative 1: Filament FormAction with closures inside the form schema
- **Pros**: Consistent with rest of the page; uses Filament's built-in modal/form system
- **Cons**: Closures silently fail in this page's schema context — no error, no execution
- **Why not**: Exhaustive debugging (6+ approaches) confirmed this is a framework bug. No workaround found within Filament's action system for this specific page.

### Alternative 2: Separate Livewire component for branding
- **Pros**: Clean separation; fully testable; no Filament dependency
- **Cons**: Extra file; needs to be mounted inside the Filament page layout; duplicates settings persistence logic
- **Why not**: The wire:click approach in the existing Blade view achieves the same result with less code and no new component.

### Alternative 3: Alpine.js with direct fetch() calls to an API endpoint
- **Pros**: No Livewire round-trip; instant client-side preview
- **Cons**: Requires a new API route; file upload via fetch is more complex; loses Livewire validation
- **Why not**: Livewire file uploads are simpler and already available via `WithFileUploads`.

## Consequences

### Positive
- Logo upload and branding settings work reliably
- No dependency on Filament's action system for this section
- File inputs with native browser UI are accessible and familiar

### Negative
- The branding section uses a different pattern than the rest of the page (wire:click vs Filament schema) — developers need to know this when modifying it
- If Filament fixes the underlying bug, we could migrate back but there's no pressing reason to

### Risks
- Future Filament upgrades could change the Blade component API (`<x-filament::button>`, `<x-filament::section>`). Mitigation: these are stable public components unlikely to break without a major version bump.
