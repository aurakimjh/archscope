package content

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/legacy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var inputTag = regexp.MustCompile(`(?i)<input\b[^>]*>`)
var inputAttr = regexp.MustCompile(`(?i)\b(name|value)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

func DecodeLegacy(raw []byte, mime string) *models.LegacyPreview {
	mime = strings.ToLower(mime)
	var result models.LegacyPreview
	if strings.Contains(mime, "amf") {
		format := "amf0"
		if strings.Contains(mime, "amf3") {
			format = "amf3"
		} else if len(raw) >= 2 && raw[0] == 0 && (raw[1] == 0 || raw[1] == 3) {
			format = "amf_packet"
		}
		result = legacy.Decode(format, raw)
		return &result
	}
	if len(raw) > legacy.MaxBytes {
		if strings.Contains(mime, "viewstate") {
			result = models.LegacyPreview{Format: "viewstate", State: "limited", Error: "input exceeds 1 MiB"}
			return &result
		}
		return nil
	}
	text := string(raw)
	if strings.Contains(mime, "viewstate") {
		result = legacy.DecodeBase64("viewstate", strings.TrimSpace(text))
		return &result
	}
	fields := map[string]string{}
	if strings.Contains(mime, "x-www-form-urlencoded") {
		values, err := url.ParseQuery(text)
		if err != nil {
			return nil
		}
		for name, values := range values {
			if len(values) > 0 {
				fields[name] = values[0]
			}
		}
	} else if strings.Contains(mime, "html") && strings.Contains(text, "__VIEWSTATE") {
		for _, tag := range inputTag.FindAllString(text, 4096) {
			name, value := "", ""
			for _, attr := range inputAttr.FindAllStringSubmatch(tag, -1) {
				v := attr[2]
				if v == "" {
					v = attr[3]
				}
				if strings.EqualFold(attr[1], "name") {
					name = v
				} else {
					value = html.UnescapeString(v)
				}
			}
			if name != "" {
				fields[name] = value
			}
		}
	}
	if _, ok := fields["__VIEWSTATE"]; !ok {
		return nil
	}
	count := 1
	if value, ok := fields["__VIEWSTATEFIELDCOUNT"]; ok {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 64 {
			result = models.LegacyPreview{Format: "viewstate", State: "invalid", Error: "invalid ViewState chunk count"}
			return &result
		}
		count = n
	}
	var joined strings.Builder
	for i := 0; i < count; i++ {
		key := "__VIEWSTATE"
		if i > 0 {
			key += strconv.Itoa(i)
		}
		value, ok := fields[key]
		if !ok {
			result = models.LegacyPreview{Format: "viewstate", State: "invalid", Error: "missing ViewState chunk"}
			return &result
		}
		joined.WriteString(value)
	}
	result = legacy.DecodeBase64("viewstate", joined.String())
	return &result
}
func CleanLegacy(input *models.LegacyPreview, policy *redact.Policy) *models.LegacyPreview {
	if input == nil {
		return nil
	}
	out := *input
	switch out.Format {
	case "amf0", "amf3", "amf_packet", "viewstate":
	default:
		return nil
	}
	switch out.State {
	case "available", "invalid", "unsupported", "limited":
	default:
		return nil
	}
	if legacy.ValidateTree(out.Root) != nil || out.Consumed < 0 || out.Consumed > legacy.MaxBytes || out.TrailingBytes < 0 || out.TrailingBytes > legacy.MaxBytes {
		return &models.LegacyPreview{Format: out.Format, State: "limited", Error: "decoded tree exceeds output budget"}
	}
	out.Error, _ = policy.RedactBody("text/plain", out.Error)
	var clean func(*models.ContentNode) *models.ContentNode
	clean = func(node *models.ContentNode) *models.ContentNode {
		if node == nil {
			return nil
		}
		copy := *node
		copy.Children = nil
		_, secret := policy.RedactNamedValue(node.Name, "probe")
		copy.Name, _ = policy.RedactBody("text/plain", node.Name)
		if secret {
			copy.Kind = "redacted"
			copy.Value = "[REDACTED]"
			out.Redacted = true
			return &copy
		}
		value, changed := policy.RedactNamedValue(node.Name, node.Value)
		copy.Value = value
		out.Redacted = out.Redacted || changed
		for i := range node.Children {
			copy.Children = append(copy.Children, *clean(&node.Children[i]))
		}
		return &copy
	}
	out.Root = clean(input.Root)
	return &out
}
