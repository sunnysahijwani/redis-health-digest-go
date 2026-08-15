package output

import (
	"bytes"
	"fmt"
	"html"
	"net"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/config"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
)

// BuildMessage assembles the raw RFC 5322 message (headers + HTML body). It is
// separated from Send so it can be tested without a mail server.
func BuildMessage(cfg config.EmailConfig, r *report.Report) []byte {
	subject := fmt.Sprintf("[%s] %s — %s", r.Status, cfg.Subject, r.Environment)

	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", cfg.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(cfg.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(renderHTML(r))
	return b.Bytes()
}

// Send delivers the digest over SMTP (STARTTLS is negotiated automatically by
// net/smtp when the server advertises it).
func Send(cfg config.EmailConfig, r *report.Report) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return smtp.SendMail(addr, auth, cfg.From, cfg.To, BuildMessage(cfg, r))
}

func renderHTML(r *report.Report) string {
	accent := "#1e8449"
	switch r.Status {
	case report.StatusWarn:
		accent = "#d68910"
	case report.StatusCritical:
		accent = "#c0392b"
	}

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><body style="font-family:Segoe UI,Arial,sans-serif;color:#2c3e50;background:#f4f5f7;padding:24px;">`)
	fmt.Fprintf(&b, `<table style="max-width:640px;margin:0 auto;border-collapse:collapse;width:100%%;">`)
	fmt.Fprintf(&b,
		`<tr><td style="background:%s;color:#fff;padding:18px 24px;border-radius:8px 8px 0 0;">`+
			`<div style="font-size:13px;text-transform:uppercase;opacity:.85;">Redis Health Digest</div>`+
			`<div style="font-size:24px;font-weight:700;">%s</div>`+
			`<div style="font-size:13px;opacity:.9;">%s · sampled %s</div></td></tr>`,
		accent, r.Status, html.EscapeString(r.Environment), r.SampledAt.Format("2006-01-02T15:04:05Z07:00"))

	b.WriteString(`<tr><td style="background:#fff;padding:8px 24px;"><table style="width:100%;font-size:14px;border-collapse:collapse;">`)
	for _, row := range rows(r) {
		fmt.Fprintf(&b,
			`<tr><td style="padding:10px 0;border-bottom:1px solid #eceff1;color:#5f6b76;">%s</td>`+
				`<td style="padding:10px 0;border-bottom:1px solid #eceff1;text-align:right;font-weight:600;">%s</td></tr>`,
			html.EscapeString(row[0]), html.EscapeString(row[1]))
	}
	b.WriteString(`</table></td></tr>`)

	fmt.Fprintf(&b, `<tr><td style="background:#fff;padding:16px 24px 24px;border-radius:0 0 8px 8px;">`+
		`<div style="border-left:4px solid %s;padding:8px 14px;background:#fafbfc;">`, accent)
	for _, msg := range r.ReasonMessages() {
		fmt.Fprintf(&b, `<div style="font-size:13px;color:#5f6b76;padding:2px 0;">• %s</div>`, html.EscapeString(msg))
	}
	b.WriteString(`</div></td></tr></table></body></html>`)
	return b.String()
}
