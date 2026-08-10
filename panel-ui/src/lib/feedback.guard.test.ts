// GH #970 regression gate. AntD's STATIC `message` / `notification` /
// `Modal.confirm`-style methods render outside the React tree: they ignore
// the ConfigProvider theme and direction and fall back to their own default
// placements. That is exactly how the panel ended up with toasts in
// different positions, sizes, and light/dark backgrounds depending on the
// page — and how it crept back twice after the first sweep, because the
// old sweeps only grepped single-line imports and 62 multiline `import {`
// blocks slipped through.
//
// The rule: components either use `App.useApp()` (inside a component) or
// `feedback` from lib/feedback (anywhere). Nobody imports `message` or
// `notification` from antd, and nobody calls the static Modal methods.
//
// If this fires on new code: replace `message.x(...)` with
// `feedback.message.x(...)` (import { feedback } from ".../lib/feedback")
// or destructure from App.useApp() inside the component.
import { readFileSync, readdirSync } from "node:fs";
import { join, relative, sep } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(__dirname, "..");

// The only two legitimate touchpoints for the static instances:
// main.tsx applies the global position/size config to them; lib/feedback
// wraps them as the pre-bridge fallback.
const ALLOWED = new Set(["main.tsx", "lib" + sep + "feedback.ts"]);

const importRe = /import \{([^}]*)\} from "antd"/gs;
const staticModalRe = /(?<![.\w])Modal\.(confirm|info|success|error|warning)\(/;

function sourceFiles(): string[] {
  const out: string[] = [];
  const walk = (dir: string) => {
    for (const e of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, e.name);
      if (e.isDirectory()) {
        walk(full);
        continue;
      }
      if (!/\.(ts|tsx)$/.test(e.name)) continue;
      if (/\.test\.|\.d\.ts$/.test(e.name)) continue;
      out.push(relative(SRC, full));
    }
  };
  walk(SRC);
  return out;
}

describe("feedback guard (GH #970)", () => {
  it("no file imports the static message/notification from antd", () => {
    const offenders: string[] = [];
    let checked = 0;
    for (const f of sourceFiles()) {
      if (ALLOWED.has(f)) continue;
      const src = readFileSync(join(SRC, f), "utf8");
      checked++;
      for (const m of src.matchAll(importRe)) {
        const names = m[1].split(",").map((s) => s.trim());
        if (names.includes("message") || names.includes("notification")) {
          offenders.push(relative(SRC, join(SRC, f)));
        }
      }
    }
    // The layout changing under the scanner must fail loudly, not pass
    // silently by scanning nothing.
    expect(checked).toBeGreaterThan(100);
    expect(
      offenders,
      "static antd toasts render outside the theme — use feedback from lib/feedback or App.useApp()",
    ).toEqual([]);
  });

  it("no file calls the static Modal methods", () => {
    const offenders: string[] = [];
    for (const f of sourceFiles()) {
      if (ALLOWED.has(f)) continue;
      const src = readFileSync(join(SRC, f), "utf8");
      const m = src.match(staticModalRe);
      if (m) offenders.push(`${f}: Modal.${m[1]}(`);
    }
    expect(
      offenders,
      "static Modal dialogs render outside the theme — use feedback.modal or App.useApp().modal",
    ).toEqual([]);
  });
});
