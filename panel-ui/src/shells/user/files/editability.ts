import type { FileEntry, FilePreviewResponse } from "./filesApi";

// EDIT_MAX_BYTES matches the server preview cap (files.read default 1 MiB).
// Gating Edit at this size means the editor always loads the FULL file, so a
// save can never truncate a larger file on disk.
export const EDIT_MAX_BYTES = 1024 * 1024;

// isTextEditable decides whether to OFFER the per-row "Edit" item. It allows
// ANY file small enough to load fully — dotfiles (.bashrc, .htaccess) and
// extensionless configs included, not just whitelisted extensions. openEditor
// still refuses real binaries (mime sniff + NUL bytes), so an accidental Edit
// on a binary fails gracefully rather than corrupting it.
//
// Empty (0-byte) files ARE editable: creating a file then editing it is the
// whole point (GH #532 — a freshly-created test.txt showed no Edit item
// because the old gate required size > 0). A 0-byte file loads as "" and
// saves fine. Only the upper bound matters.
export function isTextEditable(entry: FileEntry): boolean {
  if (entry.is_dir || entry.is_symlink) return false;
  return entry.size >= 0 && entry.size < EDIT_MAX_BYTES;
}

// isPreviewEditable is the second gate, applied AFTER the preview call returns.
// isTextEditable only sees listing metadata (size, type); this sees the bytes.
//
// Three independent signals, any one of which means "not editable text":
//   1. is_binary from the server — sniffed binary, OR content that is not valid
//      UTF-8 (JAB-191: latin1/windows-1252 PHP and HTML, common on legacy
//      sites). Those arrive with an EMPTY content field because the bytes
//      travelled as base64, so opening them would show a blank editor and Save
//      would write that blank over the user's file.
//   2. mime_type says non-text (image, octet-stream, ...).
//   3. Content contains NUL bytes — classic binary smell; text never has them,
//      and JSON preserves \u0000 literally so we actually see it.
export function isPreviewEditable(resp: FilePreviewResponse): boolean {
  if (resp.is_binary) return false;
  const mime = (resp.mime_type || "").toLowerCase();
  const mimeIsText =
    mime.startsWith("text/") ||
    mime.startsWith("application/json") ||
    mime.startsWith("application/xml") ||
    mime.startsWith("application/x-sh") ||
    mime.startsWith("application/javascript") ||
    mime === "";
  if (!mimeIsText) return false;
  return !(resp.content || "").includes("\u0000");
}
