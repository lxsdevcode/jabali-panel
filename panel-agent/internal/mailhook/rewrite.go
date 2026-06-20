// Package mailhook implements the jabali-mailhook service: a loopback-only
// HTTP endpoint Stalwart calls as an MTA hook at the SMTP DATA stage. It
// appends a per-domain disclaimer to outgoing mail, rewriting the MIME body
// losslessly (preserving HTML markup, encodings, boundaries, and attachments)
// — something the previous Sieve approach could not do (GH #233, ADR-0143).
package mailhook

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"
)

// Disclaimer is the operator-configured text for one domain. Text is plain
// (as typed in the panel); the HTML part gets an HTML-escaped rendering.
type Disclaimer struct {
	Text string
}

// rewriteBody appends the disclaimer to every displayable text part of a
// message and returns the new BODY ONLY (Stalwart's replaceContents replaces
// the body and preserves the original top-level headers — ADR-0143).
//
// headers is the top-level header block exactly as Stalwart delivered it
// (each value still carries its leading space + trailing CRLF); body is the
// raw body (parts with their original Content-Transfer-Encoding). We rejoin
// them, parse, transform, then strip the headers back off so the returned
// value is body-only and keeps the original boundary.
func rewriteBody(headers [][2]string, body []byte, d Disclaimer) ([]byte, error) {
	var raw bytes.Buffer
	for _, h := range headers {
		// h[1] already includes the leading space and trailing CRLF.
		raw.WriteString(h[0])
		raw.WriteByte(':')
		raw.WriteString(h[1])
	}
	raw.WriteString("\r\n")
	raw.Write(body)

	ent, err := message.Read(&raw)
	if err != nil && message.IsUnknownCharset(err) {
		// Unknown charset is non-fatal; ent is still usable.
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("parse message: %w", err)
	}

	var out bytes.Buffer
	w, err := message.CreateWriter(&out, ent.Header)
	if err != nil {
		return nil, fmt.Errorf("create writer: %w", err)
	}
	if err := transform(w, ent, d); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	// Strip the re-emitted top headers; return body only so it slots in behind
	// the headers Stalwart preserves (and keeps the original boundary).
	full := out.Bytes()
	if i := bytes.Index(full, []byte("\r\n\r\n")); i >= 0 {
		return full[i+4:], nil
	}
	if i := bytes.Index(full, []byte("\n\n")); i >= 0 {
		return full[i+2:], nil
	}
	return full, nil
}

// transform recursively copies an entity into w, appending the disclaimer to
// leaf text/plain and text/html parts (skipping attachments).
func transform(w *message.Writer, e *message.Entity, d Disclaimer) error {
	if mr := e.MultipartReader(); mr != nil {
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("next part: %w", err)
			}
			pw, err := w.CreatePart(p.Header)
			if err != nil {
				return fmt.Errorf("create part: %w", err)
			}
			if err := transform(pw, p, d); err != nil {
				return err
			}
			if err := pw.Close(); err != nil {
				return fmt.Errorf("close part: %w", err)
			}
		}
		return nil
	}

	bodyBytes, err := io.ReadAll(e.Body)
	if err != nil {
		return fmt.Errorf("read part body: %w", err)
	}

	mediaType, _, _ := e.Header.ContentType()
	disp, _, _ := e.Header.ContentDisposition()
	if disp == "attachment" {
		// Never touch attachments, even text/* ones.
		_, err = io.Copy(w, bytes.NewReader(bodyBytes))
		return err
	}

	switch mediaType {
	case "text/plain":
		bodyBytes = appendPlain(bodyBytes, d.Text)
	case "text/html":
		bodyBytes = appendHTML(bodyBytes, d.Text)
	}
	_, err = io.Copy(w, bytes.NewReader(bodyBytes))
	return err
}

// appendPlain adds a signature-delimited disclaimer to a plain-text body.
func appendPlain(body []byte, text string) []byte {
	var b bytes.Buffer
	b.Write(body)
	if !bytes.HasSuffix(body, []byte("\n")) {
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n-- \r\n")
	b.WriteString(text)
	b.WriteString("\r\n")
	return b.Bytes()
}

// appendHTML inserts the disclaimer just before the last </body> (case
// insensitive); if there is no body tag it appends to the end. The operator
// text is HTML-escaped so it can't inject markup.
func appendHTML(body []byte, text string) []byte {
	snippet := "<hr><p>" + htmlEscape(text) + "</p>"
	lower := strings.ToLower(string(body))
	if idx := strings.LastIndex(lower, "</body>"); idx >= 0 {
		var b bytes.Buffer
		b.Write(body[:idx])
		b.WriteString(snippet)
		b.Write(body[idx:])
		return b.Bytes()
	}
	var b bytes.Buffer
	b.Write(body)
	b.WriteString(snippet)
	return b.Bytes()
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}
