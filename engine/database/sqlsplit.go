package database

import "strings"

func SplitSQL(text string) []string {
	statements := make([]string, 0)
	var b strings.Builder
	var quote byte
	lineComment := false
	blockComment := false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if lineComment {
			if ch == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if ch == '*' && i+1 < len(text) && text[i+1] == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			b.WriteByte(ch)
			if ch == '\\' && i+1 < len(text) {
				i++
				b.WriteByte(text[i])
				continue
			}
			if ch == quote {
				if i+1 < len(text) && text[i+1] == quote {
					i++
					b.WriteByte(text[i])
					continue
				}
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			quote = ch
			b.WriteByte(ch)
			continue
		}
		if ch == '#' || (ch == '-' && i+1 < len(text) && text[i+1] == '-' && (i+2 == len(text) || text[i+2] == ' ' || text[i+2] == '\t' || text[i+2] == '\r' || text[i+2] == '\n')) {
			lineComment = true
			if ch == '-' {
				i++
			}
			continue
		}
		if ch == '/' && i+1 < len(text) && text[i+1] == '*' {
			blockComment = true
			i++
			continue
		}
		if ch == ';' {
			if statement := strings.TrimSpace(b.String()); statement != "" {
				statements = append(statements, statement)
			}
			b.Reset()
			continue
		}
		b.WriteByte(ch)
	}
	if statement := strings.TrimSpace(b.String()); statement != "" {
		statements = append(statements, statement)
	}
	return statements
}
