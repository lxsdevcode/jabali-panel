package mailhook

import (
	"bytes"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"github.com/emersion/go-message"
)

const discText = "Confidential — Acme Corp"

// splitHeadersBody splits a raw RFC822 message into the [][2]string header
// form Stalwart sends (value keeps leading space + trailing CRLF) and the body.
func splitHeadersBody(t *testing.T, raw string) ([][2]string, []byte) {
	t.Helper()
	raw = strings.ReplaceAll(raw, "\n", "\r\n")
	i := strings.Index(raw, "\r\n\r\n")
	if i < 0 {
		t.Fatalf("no header/body separator")
	}
	head := raw[:i]
	body := raw[i+4:]
	var hs [][2]string
	for _, line := range strings.Split(head, "\r\n") {
		c := strings.Index(line, ":")
		if c < 0 {
			continue
		}
		hs = append(hs, [2]string{line[:c], line[c+1:] + "\r\n"})
	}
	return hs, []byte(body)
}

// parseLeaves walks the rewritten body (re-attached to its headers) and returns
// a map of media-type → decoded body string. Multiple parts of the same type
// are concatenated.
func parseLeaves(t *testing.T, headers [][2]string, body []byte) map[string]string {
	t.Helper()
	var raw bytes.Buffer
	for _, h := range headers {
		raw.WriteString(h[0] + ":" + h[1])
	}
	raw.WriteString("\r\n")
	raw.Write(body)
	ent, err := message.Read(&raw)
	if err != nil && !message.IsUnknownCharset(err) {
		t.Fatalf("re-parse: %v", err)
	}
	out := map[string]string{}
	var walk func(e *message.Entity)
	walk = func(e *message.Entity) {
		if mr := e.MultipartReader(); mr != nil {
			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("next part: %v", err)
				}
				walk(p)
			}
			return
		}
		mt, _, _ := e.Header.ContentType()
		b, _ := io.ReadAll(e.Body)
		out[mt] += string(b)
	}
	walk(ent)
	return out
}

func TestRewrite_PlainOnly(t *testing.T) {
	raw := "Content-Type: text/plain; charset=utf-8\n\nHello plain body."
	hs, body := splitHeadersBody(t, raw)
	got, err := rewriteBody(hs, body, Disclaimer{Text: discText})
	if err != nil {
		t.Fatal(err)
	}
	leaves := parseLeaves(t, hs, got)
	if !strings.Contains(leaves["text/plain"], "Hello plain body.") {
		t.Fatalf("original plain text lost: %q", leaves["text/plain"])
	}
	if !strings.Contains(leaves["text/plain"], discText) {
		t.Fatalf("disclaimer missing from plain: %q", leaves["text/plain"])
	}
	if strings.Contains(leaves["text/plain"], "<hr>") {
		t.Fatalf("plain part contains HTML markup: %q", leaves["text/plain"])
	}
}

func TestRewrite_HTMLPreservesMarkup(t *testing.T) {
	raw := "Content-Type: text/html; charset=utf-8\n\n<html><body><p>Rich <b>bold</b> body.</p></body></html>"
	hs, body := splitHeadersBody(t, raw)
	got, err := rewriteBody(hs, body, Disclaimer{Text: discText})
	if err != nil {
		t.Fatal(err)
	}
	leaves := parseLeaves(t, hs, got)
	html := leaves["text/html"]
	if !strings.Contains(html, "<b>bold</b>") {
		t.Fatalf("HTML markup destroyed: %q", html)
	}
	if !strings.Contains(html, "<hr><p>Confidential") {
		t.Fatalf("disclaimer missing from html: %q", html)
	}
	// inserted before </body>
	if strings.Index(html, "<hr>") > strings.LastIndex(html, "</body>") {
		t.Fatalf("disclaimer after </body>: %q", html)
	}
}

func TestRewrite_MultipartAlternative(t *testing.T) {
	b := "BOUND123"
	raw := "Content-Type: multipart/alternative; boundary=\"" + b + "\"\n\n" +
		"--" + b + "\nContent-Type: text/plain; charset=utf-8\n\nPlain alt.\n\n" +
		"--" + b + "\nContent-Type: text/html; charset=utf-8\n\n<html><body><p>Rich <b>x</b>.</p></body></html>\n\n" +
		"--" + b + "--\n"
	hs, body := splitHeadersBody(t, raw)
	got, err := rewriteBody(hs, body, Disclaimer{Text: discText})
	if err != nil {
		t.Fatal(err)
	}
	// boundary must be preserved so it matches the (preserved) top header
	if !bytes.Contains(got, []byte("--"+b)) {
		t.Fatalf("original boundary not preserved in body: %s", got)
	}
	leaves := parseLeaves(t, hs, got)
	if !strings.Contains(leaves["text/plain"], "Plain alt.") || !strings.Contains(leaves["text/plain"], discText) {
		t.Fatalf("plain alt wrong: %q", leaves["text/plain"])
	}
	if !strings.Contains(leaves["text/html"], "<b>x</b>") || !strings.Contains(leaves["text/html"], "<hr><p>Confidential") {
		t.Fatalf("html alt wrong: %q", leaves["text/html"])
	}
}

func TestRewrite_Base64HTML(t *testing.T) {
	htmlSrc := "<html><body><p>Enc <b>bold</b>.</p></body></html>"
	enc := base64.StdEncoding.EncodeToString([]byte(htmlSrc))
	raw := "Content-Type: text/html; charset=utf-8\nContent-Transfer-Encoding: base64\n\n" + enc
	hs, body := splitHeadersBody(t, raw)
	got, err := rewriteBody(hs, body, Disclaimer{Text: discText})
	if err != nil {
		t.Fatal(err)
	}
	leaves := parseLeaves(t, hs, got)
	if !strings.Contains(leaves["text/html"], "<b>bold</b>") || !strings.Contains(leaves["text/html"], "<hr><p>Confidential") {
		t.Fatalf("base64 html rewrite wrong: %q", leaves["text/html"])
	}
}

func TestRewrite_AttachmentUntouched(t *testing.T) {
	b := "MIX1"
	raw := "Content-Type: multipart/mixed; boundary=\"" + b + "\"\n\n" +
		"--" + b + "\nContent-Type: text/plain; charset=utf-8\n\nBody text.\n\n" +
		"--" + b + "\nContent-Type: text/plain; charset=utf-8\nContent-Disposition: attachment; filename=notes.txt\n\nATTACHMENT-CONTENT-DO-NOT-TOUCH\n\n" +
		"--" + b + "--\n"
	hs, body := splitHeadersBody(t, raw)
	got, err := rewriteBody(hs, body, Disclaimer{Text: discText})
	if err != nil {
		t.Fatal(err)
	}
	gs := string(got)
	if !strings.Contains(gs, "ATTACHMENT-CONTENT-DO-NOT-TOUCH") {
		t.Fatalf("attachment content lost")
	}
	// disclaimer must appear once (in the body text part), not in the attachment
	if strings.Count(gs, discText) != 1 {
		t.Fatalf("disclaimer appeared %d times (want 1, attachment must be skipped)", strings.Count(gs, discText))
	}
}

func TestRewrite_HTMLInjectionEscaped(t *testing.T) {
	raw := "Content-Type: text/html; charset=utf-8\n\n<html><body><p>Hi.</p></body></html>"
	hs, body := splitHeadersBody(t, raw)
	got, err := rewriteBody(hs, body, Disclaimer{Text: `<script>alert(1)</script>`})
	if err != nil {
		t.Fatal(err)
	}
	leaves := parseLeaves(t, hs, got)
	if strings.Contains(leaves["text/html"], "<script>") {
		t.Fatalf("operator text not escaped: %q", leaves["text/html"])
	}
	if !strings.Contains(leaves["text/html"], "&lt;script&gt;") {
		t.Fatalf("expected escaped script: %q", leaves["text/html"])
	}
}
