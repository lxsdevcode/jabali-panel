// CodeEditor — the file-manager's code editor, backed by CodeMirror 6.
//
// Replaces Monaco. The editor only ever needed value/onChange, a language for
// highlighting, and a light/dark theme — no IntelliSense, no diff view, and the
// language SERVICES were already dropped in JAB-146. CodeMirror covers that
// surface at a fraction of the cost:
//
//   build heap floor   1400 MB -> ~950 MB   (measured by bisecting
//                                            --max-old-space-size)
//   modules            8382 -> 7398
//   node_modules       76 MB -> ~7 MB
//
// The floor is what matters: a 2 GB host caps node at 50% of RAM (986 MB), so
// Monaco could not build there at all — that is why `jabali update` fell back
// to a 4-hour build and then died. Under this the same host builds with the
// default cap and no special handling.
//
// Language grammars load on demand (see ./languages), so opening a .php file
// fetches the PHP grammar chunk and nothing else.
import { useEffect, useRef } from "react";
import { Compartment, EditorState } from "@codemirror/state";
import { EditorView, keymap, lineNumbers, highlightActiveLine } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentWithTab } from "@codemirror/commands";
import {
  bracketMatching,
  foldGutter,
  indentOnInput,
  syntaxHighlighting,
  defaultHighlightStyle,
} from "@codemirror/language";
import { highlightSelectionMatches, searchKeymap } from "@codemirror/search";
import { oneDark } from "@codemirror/theme-one-dark";
import { loadLanguage } from "./languages";

export interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  /** Same identifiers FileManagerPage's languageByExt produces. */
  language?: string;
  dark?: boolean;
  fontSize?: number;
  readOnly?: boolean;
  height?: string | number;
}

export const CodeEditor = ({
  value,
  onChange,
  language = "plaintext",
  dark = false,
  fontSize = 13,
  readOnly = false,
  height = "100%",
}: CodeEditorProps) => {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const viewRef = useRef<EditorView | null>(null);

  // Compartments let language/theme/read-only be swapped by dispatching a
  // reconfigure, instead of tearing down and rebuilding the view. Rebuilding
  // would drop undo history and scroll position every time someone opened a
  // different file type.
  const langComp = useRef(new Compartment()).current;
  const themeComp = useRef(new Compartment()).current;
  const roComp = useRef(new Compartment()).current;
  const fontComp = useRef(new Compartment()).current;

  // onChange is read through a ref so the updateListener closure created once
  // at mount always calls the latest handler — without it, a re-render would
  // leave the editor calling a stale setState.
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  // Guards the controlled-component feedback loop: when we push an external
  // value in, the resulting update must not be reported back as a user edit.
  const applyingExternal = useRef(false);

  const fontTheme = (size: number) =>
    EditorView.theme({
      "&": { height: typeof height === "number" ? `${height}px` : height, fontSize: `${size}px` },
      ".cm-scroller": { fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" },
    });

  useEffect(() => {
    if (!hostRef.current || viewRef.current) return;

    const view = new EditorView({
      parent: hostRef.current,
      state: EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          foldGutter(),
          history(),
          indentOnInput(),
          bracketMatching(),
          highlightActiveLine(),
          highlightSelectionMatches(),
          syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
          keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap, indentWithTab]),
          EditorView.lineWrapping,
          langComp.of([]),
          themeComp.of(dark ? oneDark : []),
          roComp.of(EditorState.readOnly.of(readOnly)),
          fontComp.of(fontTheme(fontSize)),
          EditorView.updateListener.of((u) => {
            if (!u.docChanged || applyingExternal.current) return;
            onChangeRef.current(u.state.doc.toString());
          }),
        ],
      }),
    });
    viewRef.current = view;
    return () => {
      view.destroy();
      viewRef.current = null;
    };
    // Mount once. Every prop is applied through a compartment below, so this
    // deliberately does not re-run — see the comment on the compartments.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // External value -> document. Skipped when the text already matches, so
  // typing (which round-trips through onChange and back into this prop) does
  // not dispatch a redundant transaction and fight the cursor.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current === value) return;
    applyingExternal.current = true;
    view.dispatch({ changes: { from: 0, to: current.length, insert: value } });
    applyingExternal.current = false;
  }, [value]);

  // Language: resolved asynchronously, so guard against the file being closed
  // or switched again before the grammar chunk arrives.
  useEffect(() => {
    let cancelled = false;
    void loadLanguage(language).then((ext) => {
      const view = viewRef.current;
      if (cancelled || !view) return;
      view.dispatch({ effects: langComp.reconfigure(ext ?? []) });
    });
    return () => {
      cancelled = true;
    };
  }, [language, langComp]);

  useEffect(() => {
    viewRef.current?.dispatch({ effects: themeComp.reconfigure(dark ? oneDark : []) });
  }, [dark, themeComp]);

  useEffect(() => {
    viewRef.current?.dispatch({ effects: roComp.reconfigure(EditorState.readOnly.of(readOnly)) });
  }, [readOnly, roComp]);

  useEffect(() => {
    viewRef.current?.dispatch({ effects: fontComp.reconfigure(fontTheme(fontSize)) });
    // fontTheme closes over `height`, which is static per mount in practice.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fontSize, fontComp]);

  return <div ref={hostRef} style={{ height, overflow: "hidden" }} />;
};

export default CodeEditor;
