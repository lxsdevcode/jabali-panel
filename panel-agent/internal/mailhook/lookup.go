package mailhook

import (
	"context"
	"database/sql"
	"strings"
)

// DBSource reads per-domain disclaimers from the panel database using the
// read-only jabali-stalwart-ro grant (SELECT on jabali_panel.domains). The
// panel API is the single writer; the hook only reads (ADR-0143).
type DBSource struct {
	DB *sql.DB
}

// ForDomain returns the disclaimer for a sender domain. A domain with no row,
// disclaimer disabled, email disabled, or empty text yields ok=false (the
// caller passes the message through unchanged).
func (s *DBSource) ForDomain(ctx context.Context, domain string) (Disclaimer, bool, error) {
	const q = `SELECT disclaimer_text FROM domains
	            WHERE name = ? AND disclaimer_enabled = 1 AND email_enabled = 1`
	var text sql.NullString
	err := s.DB.QueryRowContext(ctx, q, domain).Scan(&text)
	if err == sql.ErrNoRows {
		return Disclaimer{}, false, nil
	}
	if err != nil {
		return Disclaimer{}, false, err
	}
	if !text.Valid || strings.TrimSpace(text.String) == "" {
		return Disclaimer{}, false, nil
	}
	return Disclaimer{Text: text.String}, true, nil
}
