import { render, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CodeEditor } from "./CodeEditor";
import { loadLanguage, languageLoaders } from "./languages";

// The editor replaced Monaco, so "it compiles" is nowhere near enough — these
// exercise the behaviours the file manager actually depends on: showing the
// file, reporting edits, and taking a new file's contents without stacking it
// onto the old ones.

const getDoc = (c: HTMLElement) => c.querySelector(".cm-content")?.textContent ?? "";

describe("CodeEditor", () => {
  it("renders the initial value into the document", async () => {
    const { container } = render(<CodeEditor value="<?php echo 1;" onChange={() => {}} />);
    await waitFor(() => expect(container.querySelector(".cm-editor")).toBeTruthy());
    expect(getDoc(container)).toContain("<?php echo 1;");
  });

  it("replaces the document when the value prop changes", async () => {
    // Opening a second file must REPLACE the buffer. An earlier draft that
    // appended instead of replacing would have silently concatenated files.
    const { container, rerender } = render(<CodeEditor value="first" onChange={() => {}} />);
    await waitFor(() => expect(getDoc(container)).toContain("first"));

    rerender(<CodeEditor value="second" onChange={() => {}} />);
    await waitFor(() => expect(getDoc(container)).toContain("second"));
    expect(getDoc(container)).not.toContain("first");
  });

  it("does not fire onChange while applying an external value", async () => {
    // The controlled-component loop: value -> editor -> onChange -> setState ->
    // value. Without the guard this re-enters and can clobber what the user is
    // typing, which is the classic way a controlled editor eats keystrokes.
    const onChange = vi.fn();
    const { container, rerender } = render(<CodeEditor value="a" onChange={onChange} />);
    await waitFor(() => expect(container.querySelector(".cm-editor")).toBeTruthy());
    onChange.mockClear();

    rerender(<CodeEditor value="b" onChange={onChange} />);
    await waitFor(() => expect(getDoc(container)).toContain("b"));
    expect(onChange).not.toHaveBeenCalled();
  });

  it("re-renders without tearing down the editor instance", async () => {
    // Language and theme are swapped through compartments precisely so the
    // view survives; rebuilding it would drop undo history and scroll position
    // every time someone opened a different file type.
    const { container, rerender } = render(
      <CodeEditor value="x" onChange={() => {}} language="php" />,
    );
    await waitFor(() => expect(container.querySelector(".cm-editor")).toBeTruthy());
    const first = container.querySelector(".cm-editor");

    rerender(<CodeEditor value="x" onChange={() => {}} language="python" dark />);
    await waitFor(() => expect(container.querySelector(".cm-editor")).toBeTruthy());
    expect(container.querySelector(".cm-editor")).toBe(first);
  });

  it("accepts a read-only document", async () => {
    const { container } = render(<CodeEditor value="ro" onChange={() => {}} readOnly />);
    await waitFor(() => expect(container.querySelector(".cm-editor")).toBeTruthy());
    expect(getDoc(container)).toContain("ro");
  });
});

describe("language loading", () => {
  it("covers every identifier languageByExt produces", async () => {
    // FileManagerPage maps extensions to these names. A name with no loader
    // silently degrades to plaintext, so the mapping is asserted here rather
    // than discovered by a user opening a .kt file and seeing no colours.
    const produced = [
      "javascript", "typescript", "json", "html", "css", "scss", "less",
      "markdown", "php", "python", "ruby", "go", "rust", "java", "kotlin",
      "shell", "sql", "xml", "yaml", "ini",
    ];
    for (const name of produced) {
      expect(languageLoaders[name], `no grammar loader for ${name}`).toBeTypeOf("function");
    }
  });

  it("returns null for plaintext and unknown languages instead of throwing", async () => {
    // languageFor() returns "plaintext" for unmapped extensions; that must be
    // a quiet no-highlighting case, never an error that breaks the editor.
    expect(await loadLanguage("plaintext")).toBeNull();
    expect(await loadLanguage("not-a-language")).toBeNull();
  });

  it("resolves a real grammar", async () => {
    expect(await loadLanguage("php")).not.toBeNull();
  });
});
