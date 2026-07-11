package sqlxtenant

import "strings"

type sqlTokenKind uint8

const (
	sqlWord sqlTokenKind = iota
	sqlQuotedIdentifier
	sqlString
	sqlSymbol
)

type sqlToken struct {
	kind       sqlTokenKind
	text       string
	start, end int
	depth      int
}

func (token sqlToken) isWord(word string) bool {
	return token.kind == sqlWord && strings.EqualFold(token.text, word)
}

type tenantRewriteAnalysis struct {
	whereEnd int
}

func analyzeTenantRewriteSQL(sql, tenantField string) (tenantRewriteAnalysis, error) {
	tokens, err := scanSQL(sql)
	if err != nil || len(tokens) == 0 || tokens[0].kind != sqlWord || tokens[0].depth != 0 {
		return tenantRewriteAnalysis{}, ErrUnsafeSQL
	}

	for index, token := range tokens {
		if index > 0 && isStatementKeyword(token) {
			return tenantRewriteAnalysis{}, ErrUnsafeSQL
		}
		if token.depth == 0 && isUnsupportedTopLevelKeyword(token) {
			return tenantRewriteAnalysis{}, ErrUnsafeSQL
		}
	}
	if last := tokens[len(tokens)-1]; last.isWord("WHERE") || last.isWord("AND") || last.isWord("OR") {
		return tenantRewriteAnalysis{}, ErrUnsafeSQL
	}

	var whereIndex int
	switch {
	case tokens[0].isWord("SELECT"):
		whereIndex, err = analyzeSelect(tokens)
	case tokens[0].isWord("UPDATE"):
		whereIndex, err = analyzeUpdate(tokens, tenantField)
	case tokens[0].isWord("DELETE"):
		whereIndex, err = analyzeDelete(tokens)
	default:
		err = ErrUnsafeSQL
	}
	if err != nil {
		return tenantRewriteAnalysis{}, err
	}

	analysis := tenantRewriteAnalysis{whereEnd: -1}
	if whereIndex >= 0 {
		analysis.whereEnd = tokens[whereIndex].end
	}
	return analysis, nil
}

func analyzeSelect(tokens []sqlToken) (int, error) {
	from, ok := uniqueTopLevelWord(tokens, 1, "FROM")
	if !ok || from == 1 {
		return -1, ErrUnsafeSQL
	}
	where, ok := optionalUniqueTopLevelWord(tokens, from+1, "WHERE")
	if !ok {
		return -1, ErrUnsafeSQL
	}
	end := len(tokens)
	if where >= 0 {
		end = where
		if where == len(tokens)-1 {
			return -1, ErrUnsafeSQL
		}
	}
	if !isSimpleRelation(tokens[from+1 : end]) {
		return -1, ErrUnsafeSQL
	}
	return where, nil
}

func analyzeUpdate(tokens []sqlToken, tenantField string) (int, error) {
	set, ok := uniqueTopLevelWord(tokens, 1, "SET")
	if !ok || set <= 1 {
		return -1, ErrUnsafeSQL
	}
	if !isSimpleRelation(tokens[1:set]) {
		return -1, ErrUnsafeSQL
	}
	if from, unique := optionalUniqueTopLevelWord(tokens, set+1, "FROM"); !unique || from >= 0 {
		return -1, ErrUnsafeSQL
	}
	where, ok := optionalUniqueTopLevelWord(tokens, set+1, "WHERE")
	if !ok {
		return -1, ErrUnsafeSQL
	}
	end := len(tokens)
	if where >= 0 {
		end = where
		if where == len(tokens)-1 {
			return -1, ErrUnsafeSQL
		}
	}
	if err := validateAssignments(tokens[set+1:end], tenantField); err != nil {
		return -1, err
	}
	return where, nil
}

func analyzeDelete(tokens []sqlToken) (int, error) {
	if len(tokens) < 3 || !tokens[1].isWord("FROM") || tokens[1].depth != 0 {
		return -1, ErrUnsafeSQL
	}
	from, ok := uniqueTopLevelWord(tokens, 1, "FROM")
	if !ok || from != 1 {
		return -1, ErrUnsafeSQL
	}
	where, ok := optionalUniqueTopLevelWord(tokens, from+1, "WHERE")
	if !ok {
		return -1, ErrUnsafeSQL
	}
	end := len(tokens)
	if where >= 0 {
		end = where
		if where == len(tokens)-1 {
			return -1, ErrUnsafeSQL
		}
	}
	if !isSimpleRelation(tokens[from+1 : end]) {
		return -1, ErrUnsafeSQL
	}
	return where, nil
}

func addTenantCondition(sql, condition string, analysis tenantRewriteAnalysis) string {
	if analysis.whereEnd < 0 {
		return sql + " WHERE " + condition
	}
	prefix := strings.TrimSpace(sql[:analysis.whereEnd])
	predicate := strings.TrimSpace(sql[analysis.whereEnd:])
	return prefix + " (" + predicate + ") AND " + condition
}

func uniqueTopLevelWord(tokens []sqlToken, start int, word string) (int, bool) {
	index := -1
	for i := start; i < len(tokens); i++ {
		if tokens[i].depth == 0 && tokens[i].isWord(word) {
			if index >= 0 {
				return -1, false
			}
			index = i
		}
	}
	return index, index >= 0
}

func optionalUniqueTopLevelWord(tokens []sqlToken, start int, word string) (int, bool) {
	index := -1
	for i := start; i < len(tokens); i++ {
		if tokens[i].depth == 0 && tokens[i].isWord(word) {
			if index >= 0 {
				return -1, false
			}
			index = i
		}
	}
	return index, true
}

func isSimpleRelation(tokens []sqlToken) bool {
	if len(tokens) == 0 {
		return false
	}
	index, ok := consumeIdentifierPath(tokens, 0)
	if !ok {
		return false
	}
	if index == len(tokens) {
		return true
	}
	if tokens[index].isWord("AS") {
		index++
	}
	if index >= len(tokens) || !isIdentifier(tokens[index]) {
		return false
	}
	return index+1 == len(tokens)
}

func validateAssignments(tokens []sqlToken, tenantField string) error {
	if len(tokens) == 0 {
		return ErrUnsafeSQL
	}
	tenantColumn := configuredColumnName(tenantField)
	start := 0
	for index := 0; index <= len(tokens); index++ {
		if index < len(tokens) && !(tokens[index].depth == 0 && tokens[index].kind == sqlSymbol && tokens[index].text == ",") {
			continue
		}
		assignment := tokens[start:index]
		if len(assignment) == 0 {
			return ErrUnsafeSQL
		}
		equals := -1
		for i, token := range assignment {
			if token.depth == 0 && token.kind == sqlSymbol && token.text == "=" {
				equals = i
				break
			}
		}
		if equals <= 0 || equals == len(assignment)-1 {
			return ErrUnsafeSQL
		}
		end, ok := consumeIdentifierPath(assignment[:equals], 0)
		if !ok || end != equals {
			return ErrUnsafeSQL
		}
		column := identifierValue(assignment[equals-1])
		if tenantColumn != "" && strings.EqualFold(column, tenantColumn) {
			return ErrTenantFieldUpdate
		}
		start = index + 1
	}
	return nil
}

func consumeIdentifierPath(tokens []sqlToken, start int) (int, bool) {
	if start >= len(tokens) || !isIdentifier(tokens[start]) || tokens[start].depth != 0 {
		return start, false
	}
	index := start + 1
	for index < len(tokens) {
		if tokens[index].depth != 0 || tokens[index].kind != sqlSymbol || tokens[index].text != "." {
			break
		}
		if index+1 >= len(tokens) || !isIdentifier(tokens[index+1]) || tokens[index+1].depth != 0 {
			return index, false
		}
		index += 2
	}
	return index, true
}

func isIdentifier(token sqlToken) bool {
	return token.kind == sqlWord || token.kind == sqlQuotedIdentifier
}

func identifierValue(token sqlToken) string {
	if token.kind != sqlQuotedIdentifier || len(token.text) < 2 {
		return token.text
	}
	value := token.text[1 : len(token.text)-1]
	switch token.text[0] {
	case '[':
		return strings.ReplaceAll(value, "]]", "]")
	case '`':
		return strings.ReplaceAll(value, "``", "`")
	case '"':
		return strings.ReplaceAll(value, `""`, `"`)
	default:
		return value
	}
}

func configuredColumnName(field string) string {
	field = strings.TrimSpace(field)
	if index := strings.LastIndexByte(field, '.'); index >= 0 {
		field = field[index+1:]
	}
	return strings.Trim(field, "`\"[]")
}

func isStatementKeyword(token sqlToken) bool {
	return token.isWord("SELECT") || token.isWord("UPDATE") || token.isWord("DELETE") || token.isWord("INSERT")
}

func isUnsupportedTopLevelKeyword(token sqlToken) bool {
	if token.kind != sqlWord {
		return false
	}
	switch strings.ToUpper(token.text) {
	case "JOIN", "UNION", "INTERSECT", "EXCEPT", "RETURNING", "ORDER", "GROUP", "HAVING",
		"LIMIT", "OFFSET", "FETCH", "FOR", "INTO", "USING", "WINDOW", "QUALIFY", "OUTPUT",
		"LOCK", "OPTION", "OVER", "CONNECT", "START", "MODEL":
		return true
	default:
		return false
	}
}

func scanSQL(sql string) ([]sqlToken, error) {
	tokens := make([]sqlToken, 0, len(sql)/4)
	depth := 0
	for index := 0; index < len(sql); {
		if isSQLSpace(sql[index]) {
			index++
			continue
		}
		if sql[index] == ';' || sql[index] == '#' || sql[index] == 0 ||
			strings.HasPrefix(sql[index:], "--") || strings.HasPrefix(sql[index:], "/*") || strings.HasPrefix(sql[index:], "*/") {
			return nil, ErrUnsafeSQL
		}
		if delimiter, ok := dollarQuoteDelimiter(sql, index); ok {
			end := strings.Index(sql[index+len(delimiter):], delimiter)
			if end < 0 {
				return nil, ErrUnsafeSQL
			}
			end += index + len(delimiter)*2
			tokens = append(tokens, sqlToken{kind: sqlString, text: sql[index:end], start: index, end: end, depth: depth})
			index = end
			continue
		}
		switch sql[index] {
		case '\'', '"', '`':
			end, ok := scanQuoted(sql, index, sql[index])
			if !ok {
				return nil, ErrUnsafeSQL
			}
			kind := sqlQuotedIdentifier
			if sql[index] == '\'' {
				kind = sqlString
			}
			tokens = append(tokens, sqlToken{kind: kind, text: sql[index:end], start: index, end: end, depth: depth})
			index = end
			continue
		case '[':
			end, ok := scanBracketIdentifier(sql, index)
			if !ok {
				return nil, ErrUnsafeSQL
			}
			tokens = append(tokens, sqlToken{kind: sqlQuotedIdentifier, text: sql[index:end], start: index, end: end, depth: depth})
			index = end
			continue
		case '(':
			tokens = append(tokens, sqlToken{kind: sqlSymbol, text: "(", start: index, end: index + 1, depth: depth})
			depth++
			index++
			continue
		case ')':
			if depth == 0 {
				return nil, ErrUnsafeSQL
			}
			depth--
			tokens = append(tokens, sqlToken{kind: sqlSymbol, text: ")", start: index, end: index + 1, depth: depth})
			index++
			continue
		}
		if isIdentifierStart(sql[index]) {
			end := index + 1
			for end < len(sql) && isIdentifierPart(sql[end]) {
				end++
			}
			tokens = append(tokens, sqlToken{kind: sqlWord, text: sql[index:end], start: index, end: end, depth: depth})
			index = end
			continue
		}
		tokens = append(tokens, sqlToken{kind: sqlSymbol, text: sql[index : index+1], start: index, end: index + 1, depth: depth})
		index++
	}
	if depth != 0 {
		return nil, ErrUnsafeSQL
	}
	return tokens, nil
}

func scanQuoted(sql string, start int, quote byte) (int, bool) {
	for index := start + 1; index < len(sql); index++ {
		if sql[index] == '\\' {
			// Backslash quote escaping is dialect and mode dependent. Reject it instead of
			// risking a scanner/database disagreement about where quoted content ends.
			return 0, false
		}
		if sql[index] != quote {
			continue
		}
		if index+1 < len(sql) && sql[index+1] == quote {
			index++
			continue
		}
		return index + 1, true
	}
	return 0, false
}

func scanBracketIdentifier(sql string, start int) (int, bool) {
	for index := start + 1; index < len(sql); index++ {
		if sql[index] != ']' {
			continue
		}
		if index+1 < len(sql) && sql[index+1] == ']' {
			index++
			continue
		}
		return index + 1, true
	}
	return 0, false
}

func dollarQuoteDelimiter(sql string, start int) (string, bool) {
	if sql[start] != '$' || start+1 >= len(sql) {
		return "", false
	}
	if sql[start+1] == '$' {
		return "$$", true
	}
	if !isDollarTagStart(sql[start+1]) {
		return "", false
	}
	index := start + 2
	for index < len(sql) && isDollarTagPart(sql[index]) {
		index++
	}
	if index >= len(sql) || sql[index] != '$' {
		return "", false
	}
	return sql[start : index+1], true
}

func isDollarTagStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isDollarTagPart(value byte) bool {
	return isDollarTagStart(value) || value >= '0' && value <= '9'
}

func isSQLSpace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}

func isIdentifierStart(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || value >= '0' && value <= '9' || value == '$'
}
