/**
 * Recognise a failed dynamic import.
 *
 * Vite/Rollup surface this differently per browser and there is no typed error
 * to match on, so the message is all we have. Matching loosely is deliberate: a
 * false positive costs one page reload, a false negative costs a blank screen.
 *
 * Lives apart from RouteErrorBoundary so that file only exports a component
 * (React Fast Refresh requirement).
 */
export function isChunkLoadError(error: unknown): boolean {
  const msg = error instanceof Error ? `${error.name}: ${error.message}` : String(error ?? "");
  return /failed to fetch dynamically imported module|error loading dynamically imported module|importing a module script failed|chunkloaderror|dynamically imported module/i.test(
    msg,
  );
}
