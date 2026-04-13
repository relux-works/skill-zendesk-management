package artifacts

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reAuthorizationBearer = regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)([^\s]+)`)
	reSecretAssignment    = regexp.MustCompile(`(?i)\b(password|passwd|pwd|token|secret|api[_-]?key|access[_-]?token|refresh[_-]?token)\b(\s*[:=]\s*)([^\s,;'"<>]+)`)
	reLoginAssignment     = regexp.MustCompile(`(?i)\b(user(name)?|login)\b(\s*[:=]\s*)([^\s,;'"<>]+)`)
	reURL                 = regexp.MustCompile(`https?://[^\s"'<>]+`)
	reLabeledPath         = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9 ]{1,30}:/[^\s"'<>]+`)
	reWindowsPath         = regexp.MustCompile(`\b[A-Za-z]:\\[^\r\n\t"'<>|]+`)
	reUnixPath            = regexp.MustCompile(`(^|[\s"'(=])(/(?:[^/\s"'<>]+/?)+)`)
	reEmail               = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	reIPv4                = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	reHostnameAssignment  = regexp.MustCompile(`(?i)\b(host(name)?|server)\b(\s*[:=]\s*)([A-Za-z0-9._\-]+)`)
	reHostname            = regexp.MustCompile(`\b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}\b`)
)

type Redactor struct {
	salt string
}

func NewRedactor(salt string) *Redactor {
	return &Redactor{salt: strings.TrimSpace(salt)}
}

func (r *Redactor) SanitizeText(input string) string {
	if strings.TrimSpace(input) == "" {
		return input
	}

	text := input
	text = reAuthorizationBearer.ReplaceAllStringFunc(text, func(match string) string {
		parts := reAuthorizationBearer.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		return parts[1] + r.token("token", parts[2])
	})
	text = reSecretAssignment.ReplaceAllStringFunc(text, func(match string) string {
		parts := reSecretAssignment.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		label := "secret"
		if strings.Contains(strings.ToLower(parts[1]), "token") {
			label = "token"
		}
		return parts[1] + parts[2] + r.token(label, parts[3])
	})
	text = reLoginAssignment.ReplaceAllStringFunc(text, func(match string) string {
		parts := reLoginAssignment.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}
		return parts[1] + parts[3] + r.token("login", parts[4])
	})
	text = reHostnameAssignment.ReplaceAllStringFunc(text, func(match string) string {
		parts := reHostnameAssignment.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}
		return parts[1] + parts[3] + r.token("host", parts[4])
	})
	text = reURL.ReplaceAllStringFunc(text, r.redactURL)
	text = reLabeledPath.ReplaceAllStringFunc(text, r.redactPath)
	text = reWindowsPath.ReplaceAllStringFunc(text, r.redactPath)
	text = reUnixPath.ReplaceAllString(text, "${1}${2}")
	text = reUnixPath.ReplaceAllStringFunc(text, func(match string) string {
		prefix := ""
		pathValue := match
		if strings.HasPrefix(match, " ") || strings.HasPrefix(match, "\"") || strings.HasPrefix(match, "'") || strings.HasPrefix(match, "(") || strings.HasPrefix(match, "=") {
			prefix = match[:1]
			pathValue = match[1:]
		}
		return prefix + r.redactPath(pathValue)
	})
	text = reEmail.ReplaceAllStringFunc(text, func(match string) string {
		return r.token("email", match)
	})
	text = reIPv4.ReplaceAllStringFunc(text, func(match string) string {
		return r.token("ip", match)
	})
	text = reHostname.ReplaceAllStringFunc(text, func(match string) string {
		return r.token("host", match)
	})
	return text
}

func (r *Redactor) redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return r.token("url", raw)
	}

	host := parsed.Hostname()
	if host == "" {
		return r.token("url", raw)
	}

	var b strings.Builder
	b.WriteString(parsed.Scheme)
	b.WriteString("://")
	b.WriteString(r.token("host", host))
	if port := parsed.Port(); port != "" {
		b.WriteString(":")
		b.WriteString(port)
	}

	if parsed.Path != "" && parsed.Path != "/" {
		b.WriteString(r.redactedPathBody(parsed.Path, "/"))
	}

	if parsed.RawQuery != "" {
		values, err := url.ParseQuery(parsed.RawQuery)
		if err == nil && len(values) > 0 {
			first := true
			b.WriteByte('?')
			for key, items := range values {
				if !first {
					b.WriteByte('&')
				}
				first = false
				b.WriteString(key)
				b.WriteByte('=')
				if len(items) == 0 {
					b.WriteString(r.token("param", key))
					continue
				}
				b.WriteString(r.token("param", strings.Join(items, ",")))
			}
		}
	}

	return b.String()
}

func (r *Redactor) redactPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}

	sep := "/"
	if strings.Contains(value, `\`) {
		sep = `\`
	}

	prefix := ""
	body := value
	if strings.Contains(value, ":/") && len(strings.SplitN(value, ":", 2)[0]) > 1 {
		parts := strings.SplitN(value, ":", 2)
		prefix = parts[0] + ":"
		body = parts[1]
	} else if len(value) >= 2 && value[1] == ':' {
		prefix = value[:2]
		body = value[2:]
	}

	return prefix + r.redactedPathBody(body, sep)
}

func (r *Redactor) redactedPathBody(body, sep string) string {
	value := strings.ReplaceAll(body, `\`, "/")
	leadingSlash := strings.HasPrefix(value, "/")
	trailingSlash := strings.HasSuffix(value, "/")
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		if leadingSlash {
			return "/"
		}
		return ""
	}

	var out []string
	for idx, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		if idx == len(parts)-1 {
			out = append(out, preserveTail(part))
			continue
		}
		out = append(out, r.token("seg", part))
	}
	if len(out) == 0 {
		return ""
	}

	result := strings.Join(out, "/")
	if leadingSlash {
		result = "/" + result
	}
	if trailingSlash {
		result += "/"
	}
	if sep == `\` {
		result = strings.ReplaceAll(result, "/", `\`)
	}
	return result
}

func preserveTail(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	if value == "" || value == "." || value == ".." {
		return "tail"
	}
	return value
}

func (r *Redactor) token(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "<" + label + ":empty>"
	}
	mac := hmac.New(sha256.New, []byte(r.salt))
	_, _ = mac.Write([]byte(label))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	sum := hex.EncodeToString(mac.Sum(nil))
	return "<" + label + ":" + sum[:10] + ">"
}
