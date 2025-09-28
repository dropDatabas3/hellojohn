package audit

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

// Log writes a structured audit event. In the future this can be wired to DB or external sink.
func Log(ctx context.Context, event string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["event"] = event
	fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	b, _ := json.Marshal(fields)
	log.Printf("%s", string(b))
}
