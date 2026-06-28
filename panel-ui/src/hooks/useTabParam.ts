// useTabParam — URL-backed tab state (#315). Drop-in for the
// `useState(defaultKey)` pattern that drove AntD tab groups from local state,
// so switching a sub-tab updates the URL (?tab=…) and back/forward + deep-links
// work. The default key is stripped from the URL so the canonical list view
// stays clean (no ?tab=general noise).
import { useSearchParams } from "react-router";

export function useTabParam<T extends string>(
  defaultKey: T,
  paramKey = "tab",
): [T, (k: T) => void] {
  const [searchParams, setSearchParams] = useSearchParams();
  const active = (searchParams.get(paramKey) as T | null) ?? defaultKey;

  const setActive = (k: T) => {
    const next = new URLSearchParams(searchParams);
    if (k === defaultKey) next.delete(paramKey);
    else next.set(paramKey, k);
    setSearchParams(next);
  };

  return [active, setActive];
}
