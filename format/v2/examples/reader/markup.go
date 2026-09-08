package main

import (
	"net/url"
	"strings"
)

// plainText flattens one AnyBlock inline-markup string to readable text.
//
// It is deliberately not a Markdown parser. Four rules are the ones a
// CommonMark parser gets wrong on this dialect, and they are the four this
// function implements:
//
//  1. `<u>`, `<font …>` and `<mention object_id="…">` are marks, not raw HTML.
//  2. A backslash escape is CommonMark's, and canonical output escapes `<`
//     before ANY tag-shaped run, so `\<sub>` in a document is literal prose.
//  3. A code span is literal: a tag inside one is text, never a mark.
//  4. `[t](anytype://object?objectId=…)` is a link to an object in the space,
//     and ONLY in its exact one-parameter form. Anything else `anytype://` is
//     an ordinary link, left alone.
//
// Emphasis (`**`, `*`, `~~`) is left in place: it means exactly what CommonMark
// says, so there is nothing to translate and no reason to risk mis-pairing it.
// Blocks of type code and embed never reach this function at all (SPEC §8.4).
func plainText(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		switch s[i] {
		case '\\':
			// CommonMark: a backslash before ASCII punctuation escapes it.
			if i+1 < len(s) && isASCIIPunct(s[i+1]) {
				out.WriteByte(s[i+1])
				i += 2
				continue
			}
			out.WriteByte('\\')
			i++
		case '`':
			if span, next := codeSpan(s, i); next > i {
				out.WriteString(span) // verbatim, backticks and all
				i = next
				continue
			}
			out.WriteByte('`')
			i++
		case '<':
			if text, next := inlineTag(s, i); next > i {
				out.WriteString(text)
				i = next
				continue
			}
			out.WriteByte('<')
			i++
		case '[':
			if text, next := inlineLink(s, i); next > i {
				out.WriteString(text)
				i = next
				continue
			}
			out.WriteByte('[')
			i++
		default:
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

// codeSpan returns the whole span, delimiters included, when a matching
// backtick run closes it. Nothing inside is markup.
func codeSpan(s string, start int) (string, int) {
	run := 0
	for start+run < len(s) && s[start+run] == '`' {
		run++
	}
	fence := s[start : start+run]
	rest := s[start+run:]
	for offset := 0; ; {
		hit := strings.Index(rest[offset:], fence)
		if hit < 0 {
			return "", start
		}
		close := offset + hit
		after := close + run
		if after < len(rest) && rest[after] == '`' {
			// A longer run is not this fence.
			for after < len(rest) && rest[after] == '`' {
				after++
			}
			offset = after
			continue
		}
		return s[start : start+run+after], start + run + after
	}
}

// inlineTag handles the three whitelisted tags. A `<` that does not open one is
// not markup — this dialect admits no other HTML.
func inlineTag(s string, start int) (string, int) {
	close := strings.IndexByte(s[start:], '>')
	if close < 0 {
		return "", start
	}
	open := s[start : start+close+1]
	name := tagName(open)
	switch name {
	case "u", "font":
		end := strings.Index(s[start+close+1:], "</"+name+">")
		if end < 0 {
			return "", start
		}
		inner := s[start+close+1 : start+close+1+end]
		return plainText(inner), start + close + 1 + end + len(name) + 3
	case "mention":
		end := strings.Index(s[start+close+1:], "</mention>")
		if end < 0 {
			return "", start
		}
		inner := s[start+close+1 : start+close+1+end]
		target := attribute(open, "object_id")
		return plainText(inner) + " [@" + target + "]", start + close + 1 + end + len("</mention>")
	}
	return "", start
}

func tagName(tag string) string {
	body := strings.TrimSuffix(strings.TrimPrefix(tag, "<"), ">")
	if i := strings.IndexAny(body, " \t"); i >= 0 {
		body = body[:i]
	}
	return body
}

// attribute reads one double-quoted attribute out of an opening tag. Canonical
// output writes double quotes; a hand-written document may use single ones.
func attribute(tag, name string) string {
	for _, quote := range []string{`"`, `'`} {
		key := name + "=" + quote
		at := strings.Index(tag, key)
		if at < 0 {
			continue
		}
		rest := tag[at+len(key):]
		if end := strings.Index(rest, quote); end >= 0 {
			return rest[:end]
		}
	}
	return ""
}

// inlineLink handles `[label](destination)`, and tells an object link from an
// ordinary one by the exact deep-link shape.
func inlineLink(s string, start int) (string, int) {
	label, after := bracketed(s, start, '[', ']')
	if after == start || after >= len(s) || s[after] != '(' {
		return "", start
	}
	dest, end := bracketed(s, after, '(', ')')
	if end == after {
		return "", start
	}
	if id, ok := objectDeepLink(dest); ok {
		return plainText(label) + " [→object:" + id + "]", end
	}
	return plainText(label) + " [→" + dest + "]", end
}

func bracketed(s string, start int, open, close byte) (string, int) {
	if s[start] != open {
		return "", start
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start+1 : i], i + 1
			}
		}
	}
	return "", start
}

// objectDeepLink answers whether a link destination addresses an object in the
// space. The form is exact: scheme anytype, host object, one objectId
// parameter, no path. `anytype://object?objectId=X&spaceId=Y` is NOT one of
// these — it stays an ordinary link, because guessing which half is the id and
// guessing wrong is unrecoverable.
func objectDeepLink(dest string) (string, bool) {
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "anytype" || u.Host != "object" || u.Path != "" {
		return "", false
	}
	q := u.Query()
	if len(q) != 1 || len(q["objectId"]) != 1 || q["objectId"][0] == "" {
		return "", false
	}
	return q["objectId"][0], true
}

func isASCIIPunct(c byte) bool {
	return c >= '!' && c <= '/' || c >= ':' && c <= '@' ||
		c >= '[' && c <= '`' || c >= '{' && c <= '~'
}
