package zendesk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func JSONValue(results []Result) any {
	if len(results) == 1 {
		return results[0].jsonValue()
	}

	values := make([]any, 0, len(results))
	for _, result := range results {
		values = append(values, result.jsonValue())
	}
	return values
}

func RenderCompact(results []Result) (string, error) {
	if len(results) == 0 {
		return "", nil
	}

	sections := make([]string, 0, len(results))
	for idx, result := range results {
		section, err := renderCompactResult(result)
		if err != nil {
			return "", err
		}
		if len(results) > 1 {
			section = fmt.Sprintf("query:%d operation:%s\n%s", idx+1, result.Operation, section)
		}
		sections = append(sections, strings.TrimRight(section, "\n"))
	}

	return strings.Join(sections, "\n\n"), nil
}

func renderCompactResult(result Result) (string, error) {
	switch result.Kind {
	case ResultKindObject:
		return renderCompactObject(result.Object, result.Columns), nil
	case ResultKindList:
		return renderCompactList(result.Items, result.Columns, result.Page), nil
	default:
		return "", fmt.Errorf("unsupported result kind %q", result.Kind)
	}
}

func renderCompactObject(object map[string]any, columns []string) string {
	if len(columns) == 0 {
		columns = sortedKeys(object)
	}

	lines := make([]string, 0, len(columns))
	for _, column := range columns {
		lines = append(lines, fmt.Sprintf("%s:%s", column, compactValue(object[column])))
	}
	return strings.Join(lines, "\n")
}

func renderCompactList(items []map[string]any, columns []string, page *PageInfo) string {
	var builder strings.Builder
	if len(columns) == 0 && len(items) > 0 {
		columns = sortedKeys(items[0])
	}

	if len(columns) > 0 {
		builder.WriteString(strings.Join(columns, ","))
		builder.WriteByte('\n')
	}

	for _, item := range items {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			values = append(values, csvValue(compactValue(item[column])))
		}
		builder.WriteString(strings.Join(values, ","))
		builder.WriteByte('\n')
	}

	if page != nil {
		pageLine := compactPage(*page)
		if pageLine != "" {
			builder.WriteByte('\n')
			builder.WriteString(pageLine)
		}
	}

	return strings.TrimRight(builder.String(), "\n")
}

func compactPage(page PageInfo) string {
	var parts []string
	if page.Mode != "" {
		parts = append(parts, "mode="+page.Mode)
	}
	if page.HasMore {
		parts = append(parts, "has_more=true")
	}
	if page.AfterCursor != "" {
		parts = append(parts, "after_cursor="+page.AfterCursor)
	}
	if page.NextPage != "" {
		parts = append(parts, "next_page="+page.NextPage)
	}
	if page.PreviousPage != "" {
		parts = append(parts, "previous_page="+page.PreviousPage)
	}
	if page.Count != nil {
		parts = append(parts, fmt.Sprintf("count=%d", *page.Count))
	}
	if len(parts) == 0 {
		return ""
	}
	return "page:" + strings.Join(parts, " ")
}

func compactValue(value any) string {
	if rendered, ok := compactAttachmentRefs(value); ok {
		return rendered
	}

	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.ReplaceAll(typed, "\n", "\\n")
	case fmt.Stringer:
		return strings.ReplaceAll(typed.String(), "\n", "\\n")
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8:
		return fmt.Sprintf("%v", typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}

func compactAttachmentRefs(value any) (string, bool) {
	refs, ok := normalizeAttachmentRefs(value)
	if !ok {
		return "", false
	}
	if len(refs) == 0 {
		return "", true
	}

	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		id := compactValue(ref["id"])
		fileName := compactValue(ref["file_name"])
		if fileName == "" {
			fileName = compactValue(ref["name"])
		}

		switch {
		case id != "" && fileName != "":
			parts = append(parts, id+" "+fileName)
		case id != "":
			parts = append(parts, id)
		case fileName != "":
			parts = append(parts, fileName)
		}
	}

	return strings.Join(parts, " | "), true
}

func normalizeAttachmentRefs(value any) ([]map[string]any, bool) {
	switch typed := value.(type) {
	case []map[string]any:
		if !looksLikeAttachmentRefs(typed) {
			return nil, false
		}
		return typed, true
	case []any:
		refs := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			ref, ok := item.(map[string]any)
			if !ok {
				return nil, false
			}
			refs = append(refs, ref)
		}
		if !looksLikeAttachmentRefs(refs) {
			return nil, false
		}
		return refs, true
	default:
		return nil, false
	}
}

func looksLikeAttachmentRefs(refs []map[string]any) bool {
	for _, ref := range refs {
		if ref == nil {
			return false
		}
		if _, hasID := ref["id"]; !hasID {
			return false
		}
		if _, hasFileName := ref["file_name"]; hasFileName {
			continue
		}
		if _, hasName := ref["name"]; hasName {
			continue
		}
		return false
	}
	return true
}

func csvValue(value string) string {
	if value == "" {
		return ""
	}
	if !strings.ContainsAny(value, ",\"\n") {
		return value
	}

	var buffer bytes.Buffer
	buffer.WriteByte('"')
	for _, r := range value {
		if r == '"' {
			buffer.WriteString(`""`)
			continue
		}
		buffer.WriteRune(r)
	}
	buffer.WriteByte('"')
	return buffer.String()
}
