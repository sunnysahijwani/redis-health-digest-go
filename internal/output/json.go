package output

import (
	"encoding/json"
	"io"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
)

// JSON writes the report as pretty-printed JSON (uses report.MarshalJSON).
func JSON(w io.Writer, r *report.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
