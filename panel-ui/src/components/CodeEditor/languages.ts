// Language grammars for the file-manager editor, loaded on demand.
//
// Each entry is a dynamic import, so opening a .php file pulls the PHP grammar
// and nothing else. This mirrors what Monaco's `_.contribution` barrel did by
// lazily registering languages — except here the laziness reaches the BUILD as
// well, because Rollup emits one chunk per grammar instead of folding the lot
// into the editor chunk.
//
// The keys are the same identifiers FileManagerPage's languageByExt already
// produces, so that map did not have to change.
import type { Extension } from "@codemirror/state";

type LanguageLoader = () => Promise<Extension>;

// StreamLanguage wraps the CodeMirror-5 era modes for the handful of languages
// with no dedicated CM6 package. They are lower fidelity than a Lezer grammar
// but correct for highlighting, and the alternative is no highlighting at all.
const legacy = (mode: string, pick: (m: Record<string, unknown>) => unknown): LanguageLoader =>
  async () => {
    const [{ StreamLanguage }, m] = await Promise.all([
      import("@codemirror/language"),
      import(/* @vite-ignore */ `@codemirror/legacy-modes/mode/${mode}`),
    ]);
    return StreamLanguage.define(pick(m as Record<string, unknown>) as never);
  };

export const languageLoaders: Record<string, LanguageLoader> = {
  javascript: async () => (await import("@codemirror/lang-javascript")).javascript({ jsx: true }),
  typescript: async () =>
    (await import("@codemirror/lang-javascript")).javascript({ jsx: true, typescript: true }),
  json: async () => (await import("@codemirror/lang-json")).json(),
  html: async () => (await import("@codemirror/lang-html")).html(),
  css: async () => (await import("@codemirror/lang-css")).css(),
  // scss and less share the Sass grammar; `indented: false` is the brace
  // syntax both of these extensions actually use.
  scss: async () => (await import("@codemirror/lang-sass")).sass({ indented: false }),
  less: async () => (await import("@codemirror/lang-sass")).sass({ indented: false }),
  markdown: async () => (await import("@codemirror/lang-markdown")).markdown(),
  php: async () => (await import("@codemirror/lang-php")).php(),
  python: async () => (await import("@codemirror/lang-python")).python(),
  sql: async () => (await import("@codemirror/lang-sql")).sql(),
  xml: async () => (await import("@codemirror/lang-xml")).xml(),
  yaml: async () => (await import("@codemirror/lang-yaml")).yaml(),
  java: async () => (await import("@codemirror/lang-java")).java(),
  rust: async () => (await import("@codemirror/lang-rust")).rust(),
  go: async () => (await import("@codemirror/lang-go")).go(),

  // No dedicated CM6 packages for these yet.
  ruby: legacy("ruby", (m) => m.ruby),
  shell: legacy("shell", (m) => m.shell),
  kotlin: legacy("clike", (m) => m.kotlin),
  // .ini / .conf / .env / .toml all map here. The properties mode covers
  // key=value with comments, which is what these files are in practice.
  ini: legacy("properties", (m) => m.properties),
};

// loadLanguage resolves a grammar, or null for plaintext and anything we have
// no mode for. Callers treat null as "no highlighting", which is the same
// outcome the old editor gave for unmapped extensions.
export async function loadLanguage(language: string): Promise<Extension | null> {
  const loader = languageLoaders[language];
  if (!loader) return null;
  try {
    return await loader();
  } catch {
    // A missing or broken grammar must never stop someone editing a file —
    // degrade to plaintext rather than failing the editor.
    return null;
  }
}
