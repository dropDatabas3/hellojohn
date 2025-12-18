package pg

import "fmt"

// uuidToString convierte el formato [16]uint8 (usado por pgx para UUIDs scan into any) a string.
func uuidToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.([16]uint8); ok {
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	}
	return ""
}
