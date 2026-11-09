package store

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func ValidateSearch(query string, fields []string) error {
	if len(fields) > 7 {
		return fmt.Errorf("too many search fields")
	}
	if len(query) > 4096 {
		return fmt.Errorf("search text exceeds 4096 bytes")
	}
	for _, field := range fields {
		switch field {
		case "url", "headers", "body", "cookies", "query", "post", "stream":
		default:
			return fmt.Errorf("unknown search field %q", field)
		}
	}
	return nil
}

func matchesSearch(tx models.CaptureTransaction, query string, fields []string) bool {
	if query == "" {
		return true
	}
	needle := strings.ToLower(query)
	if len(fields) == 0 {
		fields = []string{"url"}
	}
	contains := func(value string) bool { return strings.Contains(strings.ToLower(value), needle) }
	headerText := func(fields []models.HeaderField) string {
		var text strings.Builder
		for _, field := range fields {
			text.WriteString(field.Name)
			text.WriteByte(':')
			text.WriteString(field.Value)
			text.WriteByte('\n')
		}
		return text.String()
	}
	for _, field := range fields {
		switch field {
		case "url":
			if contains(tx.URL) {
				return true
			}
		case "query":
			query := tx.Query
			if parsed, err := url.Parse(tx.URL); err == nil && query == "" {
				query = parsed.RawQuery
			}
			decoded, _ := url.QueryUnescape(query)
			if contains(query) || contains(decoded) {
				return true
			}
		case "headers":
			if contains(headerText(tx.Request.Headers)) || contains(headerText(tx.Response.Headers)) {
				return true
			}
		case "cookies":
			if contains(headerText(tx.Request.Cookies)) || contains(headerText(tx.Response.Cookies)) {
				return true
			}
		case "post":
			decoded, _ := url.QueryUnescape(tx.Request.BodyPreview)
			if contains(tx.Request.BodyPreview) || contains(decoded) {
				return true
			}
		case "body":
			if contains(tx.Request.BodyPreview) || contains(tx.Response.BodyPreview) {
				return true
			}
		case "stream":
			if contains(tx.Method+" "+tx.URL+" "+tx.HTTPVersion) || contains(headerText(tx.Request.Headers)) || contains(headerText(tx.Response.Headers)) || contains(tx.Request.BodyPreview) || contains(tx.Response.BodyPreview) {
				return true
			}
		}
	}
	return false
}
