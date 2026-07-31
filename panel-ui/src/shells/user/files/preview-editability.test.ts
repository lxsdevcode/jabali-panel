import { describe, expect, it } from "vitest";

import { isPreviewEditable } from "./editability";
import type { FilePreviewResponse } from "./filesApi";

function resp(over: Partial<FilePreviewResponse>): FilePreviewResponse {
  return {
    path: "/home/u/f.txt",
    size: 10,
    content: "hello",
    mime_type: "text/plain; charset=utf-8",
    ...over,
  } as FilePreviewResponse;
}

describe("isPreviewEditable", () => {
  it("allows ordinary text", () => {
    expect(isPreviewEditable(resp({}))).toBe(true);
  });

  // JAB-191: a latin1 PHP/HTML file sniffs as text/plain and contains no NUL
  // bytes, so the mime and NUL checks both wave it through. Only is_binary
  // catches it. Its content field is EMPTY — the bytes came back base64 — so
  // opening it would show a blank editor and Save would write that blank over
  // the user's real file.
  it("rejects non-UTF-8 text flagged by the server, despite a text mime", () => {
    expect(
      isPreviewEditable(
        resp({
          is_binary: true,
          content: "",
          mime_type: "text/plain; charset=utf-8",
        }),
      ),
    ).toBe(false);
  });

  it("rejects sniffed binaries", () => {
    expect(isPreviewEditable(resp({ mime_type: "application/octet-stream" }))).toBe(false);
    expect(isPreviewEditable(resp({ mime_type: "image/png" }))).toBe(false);
  });

  it("rejects content containing NUL bytes", () => {
    expect(isPreviewEditable(resp({ content: "a\u0000b" }))).toBe(false);
  });

  it("allows text-ish mimes and an absent mime", () => {
    for (const m of [
      "application/json",
      "application/xml",
      "application/x-sh",
      "application/javascript",
      "",
      undefined,
    ]) {
      expect(isPreviewEditable(resp({ mime_type: m }))).toBe(true);
    }
  });

  it("allows an empty file", () => {
    expect(isPreviewEditable(resp({ content: "", size: 0 }))).toBe(true);
  });
});
