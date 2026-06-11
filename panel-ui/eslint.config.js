// Flat ESLint config (ESLint 9) for the Vite + React + TypeScript SPA.
// Intentionally pragmatic: this is being added to a large pre-existing
// codebase, so we lint for real correctness signals (typescript-eslint
// recommended + react-hooks rules) and disable the stylistic / opinionated
// rules that would otherwise flag thousands of pre-existing lines without
// catching bugs. Tighten incrementally over time.
import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import globals from "globals";

export default tseslint.config(
  {
    // Not source — never lint build output, deps, or generated tooling data.
    ignores: ["dist/**", "node_modules/**", "**/*.config.js", ".claude-flow/**"],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: { ...globals.browser, ...globals.es2021 },
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // react-refresh: warn only — exporting constants alongside components is
      // common in this codebase and not a runtime bug.
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],
      // Pragmatic relaxations for an existing codebase. These are style /
      // strictness rules, not bug detectors; leaving them on would bury real
      // findings under thousands of pre-existing hits.
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_", caughtErrorsIgnorePattern: "^_" },
      ],
      "@typescript-eslint/no-empty-object-type": "off",
      "@typescript-eslint/no-non-null-assertion": "off",
      "no-empty": ["error", { allowEmptyCatch: true }],
    },
  },
  {
    // Service workers (public/*.js) run in the ServiceWorkerGlobalScope —
    // `self`, `caches`, `clients`, etc. are globals there, not undefined.
    files: ["public/**/*.js"],
    languageOptions: {
      globals: { ...globals.serviceworker, ...globals.browser },
    },
    rules: {
      // .js files use the base no-unused-vars (not the TS one); apply the
      // same ignore patterns so a deliberately-unused `_err` catch is fine.
      "no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_", caughtErrorsIgnorePattern: "^_" },
      ],
      // typescript-eslint also lints .js — apply the same ignore patterns so
      // an intentional `_err` catch isn't a hard error.
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_", caughtErrorsIgnorePattern: "^_" },
      ],
    },
  },
  {
    // Test files: allow the patterns test code legitimately uses.
    files: ["**/*.test.{ts,tsx}", "**/__tests__/**"],
    languageOptions: { globals: { ...globals.node } },
    rules: {
      "@typescript-eslint/no-non-null-assertion": "off",
    },
  },
);
