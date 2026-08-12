package gqldb

import "strings"

// Rejects caller-supplied GQL that selects or replaces a graph.
//
// Intended for multi-tenant deployments that pin every request to one graph
// via QueryConfig.GraphName and let end users contribute part of the query
// text. Request.graph_name is only a per-request override (see
// GqlResponse.current_graph in gqldb.proto): an embedded USE wins over it,
// so the pin alone does not contain a tenant.
//
// Interim measure - not a permanent driver feature. The blocked list below
// is fixed, so any graph-selection syntax the server adds later is silently
// let through, and a change to the lexical rules quoted at the bottom can
// turn a covered form into a bypass. Before extending this, check whether
// the server can now enforce the constraint instead; once it can, this file
// and its flag are deprecated and removed at the next major version.
//
// This is defense-in-depth, not a security boundary. It inspects query
// text; it cannot constrain what the connected account is allowed to touch.
// A tenant boundary has to come from per-tenant database users and RBAC.
// SHOW GRAPHS still enumerates every graph on the server regardless of this
// guard.
//
// Blocked: a statement whose leading keyword is USE (with or without GRAPH),
// which switches the session graph and wins over the pin.
//
// Graph-lifecycle DDL is deliberately NOT blocked. DROP GRAPH,
// CREATE GRAPH ... AS COPY OF ... and ALTER GRAPH ... RENAME TO ... are
// cross-graph by design, and on a live server they reach another tenant's
// data with no USE anywhere (DROP GRAPH <own> then CREATE GRAPH <own> AS COPY
// OF <victim> leaves the pin pointing at the victim's rows). That chain is
// out of scope by decision, so this guard is only meaningful on requests that
// also set read_only - read-only requests are rejected server-side for any
// write ([4016]), which closes those chains independently.
//
// Why a hand-written scanner rather than a regex: on a live server every one
// of `USE b`, `use graph b`, `UsE gRaPh b`, "USE\n\tGRAPH b", "USE b"
// (NBSP), `;USE b`, `USE b;;MATCH`, `/*c*/USE b`, "//c\nUSE b", "--c\nUSE b"
// and `MATCH ...; USE b` switched graphs, while `RETURN 'USE GRAPH b'` and
// `INSERT (:N {name:'USE GRAPH b'})` are legitimate queries a keyword regex
// would wrongly reject. Distinguishing the two needs comment- and
// string-aware statement splitting, so that is what this does.
//
// Lexical rules confirmed against the server: line comments // and --
// (# is not a comment); block comments /* */ and they NEST; string
// delimiters ', " and `, with backslash escapes (doubled '' is not an
// escape); and any of space / tab / CR / LF / FF / VT / NBSP separates USE
// from the graph name - so the scanner reads identifier tokens rather than
// matching a whitespace class, since Go's regexp \s does not match NBSP.

// unparseableKeyword is reported when the text cannot be scanned, so the
// caller fails closed.
const unparseableKeyword = "<unparseable>"

// blockedKeyword is the only leading keyword that switches the session
// graph.
const blockedKeyword = "USE"

// maxGuardTokens: only the first token of each statement is needed.
const maxGuardTokens = 1

// findBlockedKeyword returns the offending leading keyword phrase,
// unparseableKeyword if the text cannot be scanned, or "" if the query is
// clean.
func findBlockedKeyword(query string) string {
	statements, ok := leadingTokens(query)
	if !ok {
		return unparseableKeyword
	}
	for _, tokens := range statements {
		if len(tokens) > 0 && strings.EqualFold(tokens[0], blockedKeyword) {
			return blockedKeyword
		}
	}
	return ""
}

// leadingTokens returns the leading word tokens of each top-level statement.
//
// Comments and string literals are skipped; `;` outside them splits
// statements. A statement whose first significant character does not start a
// word contributes an empty token slice. The bool is false if the text
// cannot be scanned (unterminated comment or string) - such a query is a
// parse error server-side anyway, so the caller fails closed rather than
// guessing.
func leadingTokens(query string) ([][]string, bool) {
	runes := []rune(query)
	n := len(runes)
	statements := make([][]string, 0, 2)
	tokens := make([]string, 0, maxGuardTokens)
	// False once the current statement has produced a non-word element, so
	// we stop collecting until the next `;`.
	collecting := true

	hasPrefix := func(i int, prefix string) bool {
		p := []rune(prefix)
		if i+len(p) > n {
			return false
		}
		for k, r := range p {
			if runes[i+k] != r {
				return false
			}
		}
		return true
	}

	for i := 0; i < n; {
		ch := runes[i]

		// --- comments -------------------------------------------------
		if hasPrefix(i, "/*") {
			depth := 1
			i += 2
			for i < n && depth > 0 {
				switch {
				case hasPrefix(i, "/*"):
					depth++
					i += 2
				case hasPrefix(i, "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth > 0 {
				return nil, false // unterminated block comment
			}
			continue
		}
		if hasPrefix(i, "//") || hasPrefix(i, "--") {
			for i < n && runes[i] != '\n' {
				i++
			}
			continue
		}

		// --- string literals ------------------------------------------
		if ch == '\'' || ch == '"' || ch == '`' {
			quote := ch
			closed := false
			i++
			for i < n {
				if runes[i] == '\\' {
					i += 2
					continue
				}
				if runes[i] == quote {
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, false // unterminated string literal
			}
			collecting = false // a literal is not a leading keyword
			continue
		}

		// --- statement separator --------------------------------------
		if ch == ';' {
			statements = append(statements, tokens)
			tokens = make([]string, 0, maxGuardTokens)
			collecting = true
			i++
			continue
		}

		// --- whitespace (any kind, including NBSP) --------------------
		if isGuardSpace(ch) {
			i++
			continue
		}

		// --- words -----------------------------------------------------
		if isGuardWordRune(ch) {
			start := i
			for i < n && isGuardWordRune(runes[i]) {
				i++
			}
			if collecting && len(tokens) < maxGuardTokens {
				tokens = append(tokens, string(runes[start:i]))
				if len(tokens) >= maxGuardTokens {
					collecting = false
				}
			}
			continue
		}

		// --- anything else ends the leading-keyword run ----------------
		collecting = false
		i++
	}

	statements = append(statements, tokens)
	return statements, true
}

// isGuardWordRune reports whether r is an identifier character. Anything
// else (including NBSP and every other separator the server accepts) ends
// the token.
func isGuardWordRune(r rune) bool {
	return r == '_' ||
		(r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		r > 0x7F && !isGuardSpace(r)
}

// isGuardSpace covers the separators the server accepts between USE and the
// graph name, including NBSP (U+00A0), which Go's unicode.IsSpace does treat
// as space but regexp's \s does not.
func isGuardSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x85, 0xA0:
		return true
	}
	// Other Unicode space separators.
	return r > 0xFF && (r == 0x1680 || (r >= 0x2000 && r <= 0x200A) ||
		r == 0x2028 || r == 0x2029 || r == 0x202F || r == 0x205F || r == 0x3000)
}
