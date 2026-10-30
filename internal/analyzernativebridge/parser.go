package analyzernativebridge

import s "strings"

const rejectUnsupported = "UNSUPPORTED_SCHEMA"

func lexicalLines(body []byte) ([]sourceLine, string) {
	if len(body) == 0 || len(body) > maxInput || body[len(body)-1] != '\n' {
		return nil, "MALFORMED_INPUT"
	}
	clean, ok := strip(body)
	if !ok {
		return nil, "UNSUPPORTED_SCHEMA"
	}
	return scanLines(clean), ""
}

func parseCBody(req request, in input, body []byte, collector *factCollector) string {
	lines, reason := lexicalLines(body)
	if reason != "" {
		return reason
	}
	return parseC(req, in, lines, collector)
}

func parseObjectiveCBody(req request, in input, body []byte, collector *factCollector) string {
	lines, reason := lexicalLines(body)
	if reason != "" {
		return reason
	}
	return parseObjectiveC(req, in, lines, collector)
}

func parseJavaBody(req request, in input, body []byte, collector *factCollector) string {
	if len(body) == 0 || len(body) > maxInput || body[len(body)-1] != '\n' {
		return "MALFORMED_INPUT"
	}
	normalized, ok := normalizeJavaUnicodeEscapes(body)
	if !ok {
		return rejectUnsupported
	}
	clean, ok := strip(normalized)
	if !ok {
		return rejectUnsupported
	}
	libraries, dynamic := javaLibraryCalls(string(normalized), string(clean))
	if dynamic {
		return "DYNAMIC_INPUT"
	}
	for _, value := range libraries {
		if reason := collector.add(newFact(req, in, "java.jni.library", in.Path, "loads-library", value, span{}, value)); reason != "" {
			return reason
		}
	}
	lines := scanLines(clean)
	return parseJava(req, in, lines, collector)
}

func normalizeJavaUnicodeEscapes(body []byte) ([]byte, bool) {
	result := make([]byte, 0, len(body))
	for i := 0; i < len(body); {
		if body[i] != '\\' || i+1 >= len(body) || body[i+1] != 'u' {
			result = append(result, body[i])
			i++
			continue
		}
		if len(result) != 0 && result[len(result)-1] == '\\' {
			return nil, false
		}
		cursor := i + 1
		for cursor < len(body) && body[cursor] == 'u' {
			cursor++
		}
		if cursor+4 > len(body) {
			return nil, false
		}
		value := 0
		for _, digit := range body[cursor : cursor+4] {
			value <<= 4
			switch {
			case digit >= '0' && digit <= '9':
				value += int(digit - '0')
			case digit >= 'a' && digit <= 'f':
				value += int(digit-'a') + 10
			case digit >= 'A' && digit <= 'F':
				value += int(digit-'A') + 10
			default:
				return nil, false
			}
		}
		if value == 0 || value > 0x7f || value == '\\' || value == '\r' || value == '\t' {
			return nil, false
		}
		result = append(result, byte(value))
		i = cursor + 4
	}
	return result, true
}

type sourceLine struct {
	clean string
	start int
	line  int
}

func scanLines(clean []byte) []sourceLine {
	lines := make([]sourceLine, 0, 64)
	cleanText := string(clean)
	start := 0
	for i := 0; i < len(cleanText); i++ {
		value := cleanText[i]
		if value != '\n' {
			continue
		}
		lines = append(lines, sourceLine{cleanText[start:i], start, len(lines) + 1})
		start = i + 1
	}
	return lines
}

func (line sourceLine) span(text string) span {
	column := s.Index(line.clean, text)
	if column < 0 {
		column = 0
	}
	return span{line.start + column, line.start + column + len(text), line.line, column + 1, line.line, column + len(text) + 1}
}

func parseC(req request, in input, lines []sourceLine, collector *factCollector) string {
	if lines == nil {
		return rejectUnsupported
	}
	if in.Family == "c.cmake" {
		return parseCMake(req, in, lines, collector)
	}
	if !balancedDelimiters(lines) {
		return rejectUnsupported
	}
	if closedBodyReason(lines, c) != "" {
		return rejectUnsupported
	}
	if !validDirectiveSequence(lines) {
		return rejectUnsupported
	}
	depth := 0
	jniHeader := false
	jniParameters := false
	var jniState jniParameterState
	for _, line := range lines {
		trimmed := s.TrimSpace(line.clean)
		if trimmed == "" {
			continue
		}
		priorJNIHeader := jniHeader
		jniHeader = false
		at := line.span(trimmed)
		if s.HasPrefix(trimmed, "#") {
			value, kind, supported := cDirective(trimmed)
			if !supported {
				return rejectUnsupported
			}
			if kind != "" {
				if reason := collector.add(newFact(req, in, kind, in.Path, "declares", value, at, value)); reason != "" {
					return reason
				}
			}
			continue
		}
		if jniParameters {
			complete, next, ok := jniParameterContinuation(trimmed, jniState)
			if !ok {
				return rejectUnsupported
			}
			jniState = next
			jniParameters = !complete
			if !complete {
				continue
			}
		}
		if !safeSourceLine(trimmed) || malformedTokenSequence(trimmed) || (depth == 0 && !knownCTopLevel(trimmed)) {
			return rejectUnsupported
		}
		if name := cFunction(trimmed); name != "" {
			if reason := collector.add(newFact(req, in, "c.function.declaration", in.Path, "declares-function", name, at, name)); reason != "" {
				return reason
			}
		}
		name := jniSymbol(trimmed)
		if name != "" {
			if depth != 0 {
				return rejectUnsupported
			}
			valid, pending, state := jniDeclaration(trimmed, priorJNIHeader)
			if !valid {
				return rejectUnsupported
			}
			jniState = state
			jniParameters = pending
			if reason := collector.add(newFact(req, in, "c.jni.symbol", in.Path, "declares-jni-symbol", name, at, name)); reason != "" {
				return reason
			}
		}
		if in.Family == "c.header" && cPrototype(trimmed) {
			return rejectUnsupported
		}
		jniHeader = depth == 0 && validJNIHeader(trimmed) && name == ""
		depth += braceDelta(trimmed)
		if depth < 0 {
			return rejectUnsupported
		}
	}
	if depth != 0 || jniHeader || jniParameters {
		return rejectUnsupported
	}
	return ""
}

func parseCMake(req request, in input, lines []sourceLine, collector *factCollector) string {
	for _, line := range lines {
		trimmed := s.TrimSpace(line.clean)
		if trimmed == "" {
			continue
		}
		matched := false
		for _, call := range []string{"add_library", "target_link_libraries", "target_include_directories"} {
			name, ok := cmakeCall(trimmed, call)
			if !ok {
				continue
			}
			if reason := collector.add(newFact(req, in, "c.cmake.declaration", in.Path, "declares-"+call, name, line.span(trimmed), name)); reason != "" {
				return reason
			}
			matched = true
			break
		}
		if !matched {
			return rejectUnsupported
		}
	}
	return ""
}

func cmakeCall(line, call string) (string, bool) {
	if !s.HasPrefix(s.ToLower(line), call) {
		return "", false
	}
	rest := s.TrimSpace(line[len(call):])
	if !s.HasPrefix(rest, "(") || !s.HasSuffix(rest, ")") {
		return "", false
	}
	fields := s.Fields(s.TrimSpace(rest[1 : len(rest)-1]))
	if len(fields) != 2 || !symbol(fields[0]) || !symbol(fields[1]) {
		return "", false
	}
	return fields[0], true
}

func cDirective(line string) (string, string, bool) {
	if s.HasSuffix(s.TrimRight(line, " \f\v"), "\\") {
		return "", "", false
	}
	fields := s.Fields(line)
	if len(fields) == 0 {
		return "", "", false
	}
	switch fields[0] {
	case "#include", "#import":
		if len(fields) != 2 || len(fields[1]) < 3 {
			return "", "", false
		}
		token := fields[1]
		if (token[0] != '<' || token[len(token)-1] != '>') && (token[0] != '"' || token[len(token)-1] != '"') {
			return "", "", false
		}
		value := token[1 : len(token)-1]
		if !sourceAtom(value) {
			return "", "", false
		}
		return value, "c.include", true
	case "#define":
		if len(fields) < 2 {
			return "", "", false
		}
		name := s.SplitN(fields[1], "(", 2)[0]
		if !symbol(name) {
			return "", "", false
		}
		return name, "c.macro", true
	case "#ifdef", "#ifndef":
		if len(fields) != 2 || !symbol(fields[1]) {
			return "", "", false
		}
		return fields[0][1:], "c.preprocessor.conditional", true
	case "#if", "#elif":
		condition := s.TrimSpace(s.TrimPrefix(line, fields[0]))
		if !preprocessorCondition(condition) {
			return "", "", false
		}
		return fields[0][1:], "c.preprocessor.conditional", true
	case "#else", "#endif":
		if len(fields) != 1 {
			return "", "", false
		}
		return fields[0][1:], "c.preprocessor.conditional", true
	}
	return "", "", false
}

type conditionalDirective struct{ seenElse bool }

func validDirectiveSequence(lines []sourceLine) bool {
	var stack []conditionalDirective
	for _, line := range lines {
		trimmed := s.TrimSpace(line.clean)
		if !s.HasPrefix(trimmed, "#") {
			continue
		}
		_, _, ok := cDirective(trimmed)
		if !ok {
			return false
		}
		switch s.Fields(trimmed)[0] {
		case "#if", "#ifdef", "#ifndef":
			stack = append(stack, conditionalDirective{})
		case "#elif":
			if len(stack) == 0 || stack[len(stack)-1].seenElse {
				return false
			}
		case "#else":
			if len(stack) == 0 || stack[len(stack)-1].seenElse {
				return false
			}
			stack[len(stack)-1].seenElse = true
		case "#endif":
			if len(stack) == 0 {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	return len(stack) == 0
}

func preprocessorCondition(value string) bool {
	if value == "" || s.ContainsAny(value, ";{}\\") {
		return false
	}
	depth := 0
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func cFunction(line string) string {
	open := s.IndexByte(line, '(')
	if open < 0 || !s.Contains(line[open+1:], ")") || !s.Contains(line, "{") {
		return ""
	}
	parts := s.Fields(s.TrimSpace(line[:open]))
	if len(parts) < 2 {
		return ""
	}
	name := s.Trim(parts[len(parts)-1], "*")
	if symbol(name) {
		return name
	}
	return ""
}

func jniSymbol(line string) string {
	start := s.Index(line, "Java_")
	if start < 0 || start > 0 && grammarIdentifierPart(line[start-1], c) {
		return ""
	}
	name := leadingSymbol(line[start:])
	if s.HasPrefix(name, "Java_") {
		return name
	}
	return ""
}

type jniParameterState struct {
	count        int
	firstJNIEnv  bool
	secondObject bool
}

func jniDeclaration(line string, priorHeader bool) (bool, bool, jniParameterState) {
	name := jniSymbol(line)
	if !jniName(name) {
		return false, false, jniParameterState{}
	}
	prefix := line[:s.Index(line, name)]
	if priorHeader && s.TrimSpace(prefix) != "" || !priorHeader && !validJNIHeader(s.TrimSpace(prefix)) {
		return false, false, jniParameterState{}
	}
	rest := s.TrimSpace(line[s.Index(line, name)+len(name):])
	if !s.HasPrefix(rest, "(") {
		return false, false, jniParameterState{}
	}
	if close := s.IndexByte(rest, ')'); close >= 0 {
		state, ok := jniParameters(rest[1:close], jniParameterState{}, false)
		if !ok || !validJNISignature(state) || !s.HasPrefix(s.TrimSpace(rest[close+1:]), "{") {
			return false, false, jniParameterState{}
		}
		return true, false, state
	}
	if rest == "(" {
		return true, true, jniParameterState{}
	}
	if !s.HasSuffix(rest, ",") {
		return false, false, jniParameterState{}
	}
	state, ok := jniParameters(rest[1:], jniParameterState{}, true)
	if !ok {
		return false, false, jniParameterState{}
	}
	return true, true, state
}

func validJNIHeader(line string) bool {
	cursor := 0
	first := nextSpaceWord(line, &cursor)
	second := nextSpaceWord(line, &cursor)
	third := nextSpaceWord(line, &cursor)
	return first == "JNIEXPORT" && jniReturnType(second) && third == "JNICALL" && nextSpaceWord(line, &cursor) == ""
}

func jniParameterContinuation(line string, state jniParameterState) (bool, jniParameterState, bool) {
	if close := s.IndexByte(line, ')'); close >= 0 {
		next, ok := jniParameters(line[:close], state, false)
		return true, next, ok && validJNISignature(next) && s.HasPrefix(s.TrimSpace(line[close+1:]), "{")
	}
	if !s.HasSuffix(line, ",") {
		return false, jniParameterState{}, false
	}
	next, ok := jniParameters(line, state, true)
	return false, next, ok
}

func jniParameters(value string, state jniParameterState, trailingComma bool) (jniParameterState, bool) {
	if trailingComma {
		value = s.TrimSpace(s.TrimSuffix(value, ","))
	}
	if value == "" {
		return jniParameterState{}, false
	}
	for {
		comma := s.IndexByte(value, ',')
		part := value
		if comma >= 0 {
			part = value[:comma]
		}
		cursor := 0
		typeName := nextSpaceWord(part, &cursor)
		name := nextSpaceWord(part, &cursor)
		if !jniType(typeName) || name == "" || nextSpaceWord(part, &cursor) != "" {
			return jniParameterState{}, false
		}
		pointers := 0
		for s.HasPrefix(name, "*") {
			pointers++
			name = name[1:]
		}
		if !symbol(name) {
			return jniParameterState{}, false
		}
		switch state.count {
		case 0:
			if typeName != "JNIEnv" || pointers != 1 {
				return jniParameterState{}, false
			}
			state.firstJNIEnv = true
		case 1:
			if (typeName != "jobject" && typeName != "jclass") || pointers != 0 {
				return jniParameterState{}, false
			}
			state.secondObject = true
		default:
			if typeName == "JNIEnv" || pointers != 0 {
				return jniParameterState{}, false
			}
		}
		state.count++
		if comma < 0 {
			break
		}
		value = value[comma+1:]
	}
	return state, true
}

func validJNISignature(state jniParameterState) bool {
	return state.count >= 2 && state.firstJNIEnv && state.secondObject
}

func nextSpaceWord(value string, cursor *int) string {
	for *cursor < len(value) && asciiSpace(value[*cursor]) {
		*cursor++
	}
	start := *cursor
	for *cursor < len(value) && !asciiSpace(value[*cursor]) {
		*cursor++
	}
	return value[start:*cursor]
}

func jniType(value string) bool {
	switch value {
	case "JNIEnv", "void", "jboolean", "jbyte", "jbyteArray", "jchar", "jclass", "jdouble", "jfloat", "jint", "jlong", "jmethodID", "jobject", "jobjectArray", "jshort", "jsize", "jstring", "jthrowable":
		return true
	}
	return false
}

func jniReturnType(value string) bool {
	return value != "JNIEnv" && jniType(value)
}

func jniName(value string) bool {
	if !s.HasPrefix(value, "Java_") || !symbol(value) {
		return false
	}
	return s.ContainsRune(value[len("Java_"):], '_')
}

func knownCTopLevel(line string) bool {
	if line == "static ;" || line == "extern ;" || line == "const ;" {
		return false
	}
	if line == "}" || s.HasPrefix(line, "using namespace ") && s.HasSuffix(line, ";") {
		return true
	}
	if cFunction(line) != "" || jniSymbol(line) != "" || s.HasPrefix(line, "JNIEXPORT ") || s.HasPrefix(line, "JNICALL ") || s.HasSuffix(line, "{") && s.Contains(line, ")") {
		return true
	}
	words := s.Fields(line)
	if len(words) == 0 {
		return true
	}
	allowed := map[string]bool{"static": true, "extern": true, "typedef": true, "struct": true, "union": true, "enum": true, "const": true, "constant": true, "volatile": true, "unsigned": true, "signed": true, "long": true, "short": true, "void": true, "int": true, "char": true, "float": true, "double": true, "bool": true, "float2": true, "float3": true, "float4": true, "jlong": true, "jint": true, "jdouble": true, "jstring": true, "jboolean": true, "jobject": true, "jclass": true, "JNIEnv": true, "JavaVM": true, "NULL": true, "thread": true, "device": true, "vertex": true, "fragment": true, "kernel": true, "inline": true}
	if !allowed[s.TrimLeft(words[0], "_")] {
		return false
	}
	return s.ContainsAny(line, ";{") || s.Contains(line, "(") || len(words) == 1
}

func cPrototype(line string) bool {
	if !s.HasSuffix(line, ";") {
		return false
	}
	open := s.IndexByte(line, '(')
	if open < 0 {
		return false
	}
	words := s.Fields(line[:open])
	if len(words) != 2 || !symbol(words[1]) {
		return false
	}
	switch words[0] {
	case "void", "int", "long", "char", "float", "double":
		return true
	}
	return false
}

func parseObjectiveC(req request, in input, lines []sourceLine, collector *factCollector) string {
	if !balancedDelimiters(lines) {
		return rejectUnsupported
	}
	if closedBodyReason(lines, objc) != "" {
		return rejectUnsupported
	}
	if !validDirectiveSequence(lines) {
		return rejectUnsupported
	}
	depth := 0
	declarations := 0
	for _, line := range lines {
		trimmed := s.TrimSpace(line.clean)
		if trimmed == "" {
			continue
		}
		at := line.span(trimmed)
		if s.HasPrefix(trimmed, "#") {
			value, kind, ok := cDirective(trimmed)
			if !ok {
				return rejectUnsupported
			}
			if kind != "" {
				if reason := collector.add(newFact(req, in, kind, in.Path, "declares", value, at, value)); reason != "" {
					return reason
				}
			}
			continue
		}
		if name, kind, directive := objcDeclaration(trimmed); directive {
			if name == "" || declarations != 0 {
				return rejectUnsupported
			}
			declarations++
			depth += braceDelta(trimmed)
			if reason := collector.add(newFact(req, in, kind, in.Path, "declares", name, at, name)); reason != "" {
				return reason
			}
			continue
		}
		if s.HasPrefix(trimmed, "@end") {
			if trimmed != "@end" || declarations == 0 || depth != 0 {
				return rejectUnsupported
			}
			declarations--
			continue
		}
		if s.HasPrefix(trimmed, "@property") && declarations == 0 {
			return rejectUnsupported
		}
		if (s.HasPrefix(trimmed, "- (") || s.HasPrefix(trimmed, "+ (")) && declarations == 0 {
			return rejectUnsupported
		}
		if !safeSourceLine(trimmed) || malformedTokenSequence(trimmed) || (depth == 0 && !knownObjectiveCTopLevel(trimmed)) {
			return rejectUnsupported
		}
		if s.HasPrefix(trimmed, "- (") || s.HasPrefix(trimmed, "+ (") {
			if !s.HasSuffix(trimmed, ";") && !s.Contains(trimmed, "{") {
				return rejectUnsupported
			}
			if s.HasSuffix(trimmed, ";") {
				name := objcMethod(trimmed)
				if name == "" {
					return rejectUnsupported
				}
				if reason := collector.add(newFact(req, in, "objc.method.declaration", in.Path, "declares-method", name, at, name)); reason != "" {
					return reason
				}
			}
		}
		depth += braceDelta(trimmed)
		if depth < 0 {
			return rejectUnsupported
		}
	}
	if depth != 0 || declarations != 0 {
		return rejectUnsupported
	}
	return ""
}

func objcDeclaration(line string) (string, string, bool) {
	for _, declaration := range []struct{ prefix, kind string }{{"@interface", "objc.interface"}, {"@implementation", "objc.implementation"}, {"@protocol", "objc.protocol"}} {
		if line != declaration.prefix && !s.HasPrefix(line, declaration.prefix+" ") {
			continue
		}
		remainder := s.TrimSpace(s.TrimPrefix(line, declaration.prefix))
		name := leadingSymbol(remainder)
		if name == "" {
			return "", "", true
		}
		return name, declaration.kind, true
	}
	return "", "", false
}

func leadingSymbol(value string) string {
	end := 0
	for end < len(value) && (value[end] == '_' || value[end] >= 'A' && value[end] <= 'Z' || value[end] >= 'a' && value[end] <= 'z' || value[end] >= '0' && value[end] <= '9') {
		end++
	}
	name := value[:end]
	if !symbol(name) {
		return ""
	}
	return name
}

func objcMethod(line string) string {
	end := s.IndexByte(line, ')')
	if end < 0 {
		return ""
	}
	tail := s.TrimSpace(line[end+1:])
	fields := s.FieldsFunc(tail, func(value rune) bool { return value == ':' || value == ' ' || value == '{' || value == ';' })
	if len(fields) == 0 || !symbol(fields[0]) {
		return ""
	}
	return fields[0]
}

func knownObjectiveCTopLevel(line string) bool {
	if s.HasPrefix(line, "@end") || s.HasPrefix(line, "@property") || s.HasPrefix(line, "@autoreleasepool") || s.HasPrefix(line, "@interface") || s.HasPrefix(line, "@implementation") || s.HasPrefix(line, "@protocol") || s.HasPrefix(line, "- (") || s.HasPrefix(line, "+ (") {
		return true
	}
	return knownCTopLevel(line)
}

func parseJava(req request, in input, lines []sourceLine, collector *factCollector) string {
	if !balancedDelimiters(lines) {
		return rejectUnsupported
	}
	if closedBodyReason(lines, java) != "" {
		return rejectUnsupported
	}
	depth := 0
	for _, line := range lines {
		trimmed := s.TrimSpace(line.clean)
		if trimmed == "" {
			continue
		}
		at := line.span(trimmed)
		if s.HasPrefix(trimmed, "package ") || s.HasPrefix(trimmed, "import ") {
			prefix, kind, predicate := "package ", "java.package", "declares-package"
			if s.HasPrefix(trimmed, "import ") {
				prefix, kind, predicate = "import ", "java.import", "imports"
			}
			if !s.HasSuffix(trimmed, ";") {
				return rejectUnsupported
			}
			value := s.TrimSuffix(s.TrimSpace(s.TrimPrefix(trimmed, prefix)), ";")
			if kind == "java.import" && s.HasPrefix(value, "static ") {
				value = s.TrimPrefix(value, "static ")
			}
			if !qualified(value) {
				return rejectUnsupported
			}
			if reason := collector.add(newFact(req, in, kind, in.Path, predicate, value, at, value)); reason != "" {
				return reason
			}
			continue
		}
		if !safeSourceLine(trimmed) || malformedTokenSequence(trimmed) || (depth == 0 && !knownJavaTopLevel(trimmed)) {
			return rejectUnsupported
		}
		if name, kind := javaType(trimmed); name != "" {
			if reason := collector.add(newFact(req, in, kind, in.Path, "declares", name, at, name)); reason != "" {
				return reason
			}
		}
		if s.Contains(trimmed, " native ") || s.HasPrefix(trimmed, "native ") {
			name := javaMethod(trimmed)
			if name == "" {
				return rejectUnsupported
			}
			if reason := collector.add(newFact(req, in, "java.native.method", in.Path, "declares-native-method", name, at, name)); reason != "" {
				return reason
			}
		}
		depth += braceDelta(trimmed)
		if depth < 0 {
			return rejectUnsupported
		}
	}
	if depth != 0 {
		return rejectUnsupported
	}
	return ""
}

func javaType(line string) (string, string) {
	fields := s.Fields(line)
	for i, field := range fields {
		kind := javaTypeKind(field)
		if kind == "" || i+1 == len(fields) {
			continue
		}
		name := leadingJavaIdentifier(fields[i+1])
		if name != "" {
			return name, kind
		}
	}
	return "", ""
}

func javaTypeKind(value string) string {
	switch value {
	case "class":
		return "java.class"
	case "interface":
		return "java.interface"
	case "record":
		return "java.record"
	case "enum":
		return "java.enum"
	}
	return ""
}

func leadingJavaIdentifier(value string) string {
	end := 0
	for end < len(value) && javaIdentifierPart(value[end]) {
		end++
	}
	name := value[:end]
	if name == "" || !javaIdentifierStart(name[0]) {
		return ""
	}
	return name
}

func javaMethod(line string) string {
	open := s.IndexByte(line, '(')
	if open < 0 {
		return ""
	}
	fields := s.Fields(s.TrimSpace(line[:open]))
	if len(fields) == 0 || !symbol(fields[len(fields)-1]) {
		return ""
	}
	return fields[len(fields)-1]
}

func javaLibraryCalls(raw, clean string) ([]string, bool) {
	values := make([]string, 0, 2)
	for i := 0; i < len(clean); {
		start, word := nextJavaIdentifier(clean, i)
		if word == "" {
			break
		}
		i = start + len(word)
		if word == "Runtime" && javaRuntimeConstruction(clean, i) {
			return nil, true
		}
		if word == "Method" || word == "getMethod" || word == "getDeclaredMethod" || word == "invoke" {
			return nil, true
		}
		if word == "Class" && javaClassForName(clean, i) {
			return nil, true
		}
		if word != "System" {
			continue
		}
		cursor := skipSpace(clean, i)
		if javaSystemClassReflection(clean, cursor) {
			return nil, true
		}
		if s.HasPrefix(clean[cursor:], "::") {
			cursor = skipSpace(clean, cursor+2)
			if hasJavaIdentifier(clean, cursor, "load") || hasJavaIdentifier(clean, cursor, "loadLibrary") {
				return nil, true
			}
			continue
		}
		if cursor >= len(clean) || clean[cursor] != '.' {
			continue
		}
		cursor = skipSpace(clean, cursor+1)
		if hasJavaIdentifier(clean, cursor, "load") {
			return nil, true
		}
		if !hasJavaIdentifier(clean, cursor, "loadLibrary") {
			continue
		}
		cursor = skipSpace(clean, cursor+len("loadLibrary"))
		if cursor >= len(clean) || clean[cursor] != '(' {
			return nil, true
		}
		cursor = skipSpace(clean, cursor+1)
		if cursor >= len(clean) || clean[cursor] != '"' {
			return nil, true
		}
		end := cursor + 1
		for end < len(clean) && clean[end] != '"' {
			end++
		}
		if end == len(clean) {
			return nil, true
		}
		value := raw[cursor+1 : end]
		cursor = skipSpace(clean, end+1)
		if cursor >= len(clean) || clean[cursor] != ')' || !symbol(value) {
			return nil, true
		}
		values = append(values, value)
		i = cursor + 1
	}
	return values, false
}

func javaClassForName(clean string, afterClass int) bool {
	cursor := skipSpace(clean, afterClass)
	if cursor >= len(clean) || clean[cursor] != '.' {
		return false
	}
	cursor = skipSpace(clean, cursor+1)
	if !hasJavaIdentifier(clean, cursor, "forName") {
		return false
	}
	cursor = skipSpace(clean, cursor+len("forName"))
	if cursor >= len(clean) || clean[cursor] != '(' {
		return false
	}
	return true
}

func javaRuntimeConstruction(clean string, afterRuntime int) bool {
	cursor := skipSpace(clean, afterRuntime)
	for _, token := range []string{".", "getRuntime", "("} {
		if token == "getRuntime" {
			if !hasJavaIdentifier(clean, cursor, token) {
				return false
			}
			cursor += len(token)
		} else {
			if !s.HasPrefix(clean[cursor:], token) {
				return false
			}
			cursor += len(token)
		}
		cursor = skipSpace(clean, cursor)
	}
	return true
}

func javaSystemClassReflection(clean string, cursor int) bool {
	for _, token := range []string{".", "class", "."} {
		if token == "class" {
			if !hasJavaIdentifier(clean, cursor, token) {
				return false
			}
			cursor += len(token)
		} else {
			if !s.HasPrefix(clean[cursor:], token) {
				return false
			}
			cursor += len(token)
		}
		cursor = skipSpace(clean, cursor)
	}
	return hasJavaIdentifier(clean, cursor, "getMethod") || hasJavaIdentifier(clean, cursor, "getDeclaredMethod")
}

func nextJavaIdentifier(value string, start int) (int, string) {
	for start < len(value) && !javaIdentifierStart(value[start]) {
		start++
	}
	if start == len(value) {
		return start, ""
	}
	end := start + 1
	for end < len(value) && javaIdentifierPart(value[end]) {
		end++
	}
	return start, value[start:end]
}

func hasJavaIdentifier(value string, start int, word string) bool {
	end := start + len(word)
	return end <= len(value) && value[start:end] == word && (end == len(value) || !javaIdentifierPart(value[end]))
}

func javaIdentifierStart(value byte) bool {
	return value == '_' || value == '$' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func javaIdentifierPart(value byte) bool {
	return javaIdentifierStart(value) || value >= '0' && value <= '9'
}

func skipSpace(value string, i int) int {
	for i < len(value) && (value[i] == ' ' || value[i] == '\t' || value[i] == '\n' || value[i] == '\f') {
		i++
	}
	return i
}

func knownJavaTopLevel(line string) bool {
	if _, kind := javaType(line); kind != "" {
		return true
	}
	if s.HasPrefix(line, "extends ") || s.HasPrefix(line, "implements ") || s.HasPrefix(line, "permits ") {
		return true
	}
	if s.HasPrefix(line, "@") {
		return len(line) > 1 && symbol(annotationName(line[1:]))
	}
	return false
}

func annotationName(line string) string {
	for i, value := range line {
		if value == '(' || value == ' ' || value == '\t' {
			return line[:i]
		}
	}
	return line
}

func safeSourceLine(line string) bool {
	if s.Contains(line, "@@") || s.Contains(line, "???") {
		return false
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_#$@()[]{};,.=+-*/%&|!<>?:~^\"' \t"
	for i := range line {
		if !s.ContainsRune(alphabet, rune(line[i])) {
			return false
		}
	}
	return true
}

func malformedTokenSequence(line string) bool {
	previous := byte(0)
	for i := 0; i < len(line); i++ {
		value := line[i]
		if asciiSpace(value) {
			continue
		}
		if value == ';' && previous == '=' {
			return true
		}
		previous = value
	}
	for _, control := range []string{"if()", "while()", "switch()", "catch()"} {
		if containsIgnoringSpace(line, control) {
			return true
		}
	}
	return false
}

func containsIgnoringSpace(value, pattern string) bool {
	for start := 0; start < len(value); start++ {
		matched, cursor := 0, start
		for cursor < len(value) && matched < len(pattern) {
			if asciiSpace(value[cursor]) {
				cursor++
				continue
			}
			if value[cursor] != pattern[matched] {
				break
			}
			cursor++
			matched++
		}
		if matched == len(pattern) {
			return true
		}
	}
	return false
}

type bodyLanguage uint8

const (
	c bodyLanguage = iota
	objc
	java
)

type bodyToken struct {
	text string
	word bool
}

type objcMessageRole uint8

const (
	objcReceiver objcMessageRole = iota
	objcSelector
	objcArgument
)

type bodyFrame struct {
	aggregate, memberList bool
	parentParen           int
}

func closedBodyReason(lines []sourceLine, lang bodyLanguage) string {
	ts, ok := bodyTokens(lines, lang)
	if !ok {
		return "tokens"
	}
	if reason := expr(ts, lang, nil); reason != "" {
		return "punctuation:" + reason
	}
	if !balancedTypeGenerics(ts, lang) {
		return "generic"
	}
	frames := []bodyFrame{{}}
	segments := make([][]bodyToken, 1, 32)
	parenDepth := 0
	for _, token := range ts {
		switch token.text {
		case "(":
			parenDepth++
			segments[len(segments)-1] = append(segments[len(segments)-1], token)
		case ")":
			parenDepth--
			segments[len(segments)-1] = append(segments[len(segments)-1], token)
		case "{":
			segment := segments[len(segments)-1]
			parent := frames[len(frames)-1]
			aggregate := parent.aggregate || aggregateOpening(segment)
			memberList := memberListOpening(segment)
			if !aggregate && !memberList && !blockOpening(segment, lang) {
				return "block"
			}
			frames = append(frames, bodyFrame{aggregate: aggregate, memberList: memberList, parentParen: parenDepth})
			segments = append(segments, nil)
			parenDepth = 0
		case "}":
			if len(frames) == 1 {
				return "close-root"
			}
			if parenDepth != 0 {
				return "close-paren"
			}
			frame := frames[len(frames)-1]
			segment := segments[len(segments)-1]
			if len(segment) != 0 && (!frame.aggregate && !frame.memberList || !validAggregateTail(segment)) {
				return "tail"
			}
			frames = frames[:len(frames)-1]
			segments = segments[:len(segments)-1]
			parenDepth = frame.parentParen
			if frame.memberList && lang != java {
				segments[len(segments)-1] = append(segments[len(segments)-1], bodyToken{text: "}"})
			} else {
				segments[len(segments)-1] = nil
			}
		case ";":
			if parenDepth > 0 {
				segments[len(segments)-1] = append(segments[len(segments)-1], token)
				continue
			}
			if !terminatedStatement(segments[len(segments)-1], lang) {
				return "statement"
			}
			segments[len(segments)-1] = nil
		default:
			segments[len(segments)-1] = append(segments[len(segments)-1], token)
		}
	}
	if len(frames) != 1 || parenDepth != 0 || len(segments[0]) != 0 {
		return "eof"
	}
	return ""
}

func bodyTokens(lines []sourceLine, lang bodyLanguage) ([]bodyToken, bool) {
	inputSize := 0
	for _, line := range lines {
		inputSize += len(line.clean) + 1
		if lang == objc {
			trimmed := s.TrimSpace(line.clean)
			if trimmed == "@end" || s.HasPrefix(trimmed, "@interface ") || s.HasPrefix(trimmed, "@implementation ") || s.HasPrefix(trimmed, "@protocol ") {
				inputSize++
			}
		}
	}
	input := make([]byte, 0, inputSize)
	for _, line := range lines {
		if s.HasPrefix(s.TrimSpace(line.clean), "#") {
			continue
		}
		input = append(input, line.clean...)
		input = append(input, '\n')
		if lang == objc {
			trimmed := s.TrimSpace(line.clean)
			if trimmed == "@end" || s.HasPrefix(trimmed, "@interface ") || s.HasPrefix(trimmed, "@implementation ") || s.HasPrefix(trimmed, "@protocol ") {
				input = append(input, ';')
			}
		}
	}
	source := string(input)
	ts := make([]bodyToken, 0, len(source)/4)
	for i := 0; i < len(source); {
		if asciiSpace(source[i]) {
			i++
			continue
		}
		start := i
		if grammarIdentifierStart(source[i], lang) {
			i++
			for i < len(source) && grammarIdentifierPart(source[i], lang) {
				i++
			}
			ts = append(ts, bodyToken{text: source[start:i], word: true})
			continue
		}
		if source[i] >= '0' && source[i] <= '9' {
			i++
			for i < len(source) && (grammarIdentifierPart(source[i], lang) || source[i] == '.') {
				i++
			}
			ts = append(ts, bodyToken{text: source[start:i]})
			continue
		}
		if source[i] == '"' || source[i] == '\'' {
			quote := source[i]
			i++
			for i < len(source) && source[i] != quote {
				i++
			}
			if i == len(source) {
				return nil, false
			}
			i++
			ts = append(ts, bodyToken{text: source[start:i]})
			continue
		}
		if !s.ContainsRune("()[]{};,.=+-*/%&|!<>?:~^@", rune(source[i])) {
			return nil, false
		}
		if source[i] == '.' && i+3 <= len(source) && source[i:i+3] == "..." {
			i += 3
			ts = append(ts, bodyToken{text: "..."})
			continue
		}
		i++
		if i < len(source) {
			pair := source[start : i+1]
			if grammarOperatorPair(pair) {
				i++
				if (pair == "<<" || pair == ">>") && i < len(source) && source[i] == '=' {
					i++
				}
			}
		}
		ts = append(ts, bodyToken{text: source[start:i]})
		if len(ts) > maxInput {
			return nil, false
		}
	}
	return ts, true
}

func grammarOperatorPair(value string) bool {
	switch value {
	case "==", "!=", "<=", ">=", "&&", "||", "++", "--", "->", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>", "::":
		return true
	}
	return false
}

func grammarIdentifierStart(value byte, lang bodyLanguage) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || lang == java && value == '$'
}

func grammarIdentifierPart(value byte, lang bodyLanguage) bool {
	return grammarIdentifierStart(value, lang) || value >= '0' && value <= '9'
}

func expr(ts []bodyToken, lang bodyLanguage, work *int) string {
	ternary := 0
	blocks := []bool{}
	// A switch block needs the full header: switch, a nonempty balanced
	// parenthesized selector, then {. States: 1 wants (, 2 inside, 3 wants {.
	headerState, headerDepth, headerStart := 0, 0, 0
	for i, token := range ts {
		if work != nil {
			*work = *work + 1
		}
		previous := bodyToken{}
		next := bodyToken{}
		if i > 0 {
			previous = ts[i-1]
		}
		if i+1 < len(ts) {
			next = ts[i+1]
		}
		switch {
		case token.text == "switch":
			headerState, headerDepth = 1, 0
		case headerState == 1:
			if token.text == "(" {
				headerState, headerDepth, headerStart = 2, 1, i+1
			} else {
				return "switch"
			}
		case headerState == 2:
			switch token.text {
			case "(":
				headerDepth++
			case ")":
				headerDepth--
				if headerDepth == 0 {
					if !switchSelector(ts[headerStart:i], lang) {
						return "switch"
					}
					headerState = 3
				}
			case ";", "{", "}":
				return "switch"
			}
		case headerState == 3:
			if token.text != "{" {
				return "switch"
			}
		}
		switch token.text {
		case "{":
			blocks = append(blocks, headerState == 3)
			headerState = 0
		case "}":
			if len(blocks) != 0 {
				blocks = blocks[:len(blocks)-1]
			}
		case ".", "->":
			if !next.word || !expressionEnd(previous) && previous.text != "{" && previous.text != "," {
				return token.text
			}
		case ",":
			if !commaEnd(previous, lang) || !commaStart(next, lang) {
				return token.text
			}
		case "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=":
			if !bin(ts, i, true) {
				return token.text
			}
		case "&&", "||", "==", "!=", "<=", ">=", "<<", ">>", "/", "%", "|":
			if !bin(ts, i, false) {
				return token.text
			}
		case "<":
			if !bin(ts, i, false) {
				return token.text
			}
		case ">":
			if genericClose(ts, i, lang) {
				continue
			}
			if !bin(ts, i, false) {
				return token.text
			}
		case "+", "-", "*", "&", "^":
			if !unary(ts, i, lang) {
				return token.text
			}
		case "!", "~":
			if expressionEnd(previous) && previous.text != "return" && previous.text != "throw" || !expressionStart(next) {
				return token.text
			}
		case "++", "--":
			if expressionEnd(previous) {
				if operandToken(next) {
					return token.text
				}
			} else if !expressionStart(next) {
				return token.text
			}
		case "?":
			if lang == java && previous.text == "<" && next.text == ">" {
				continue
			}
			if !expressionEnd(previous) || !expressionStart(next) {
				return token.text
			}
			ternary++
		case ":":
			if len(blocks) != 0 && blocks[len(blocks)-1] && switchLabelColon(ts, i, lang) {
				continue
			}
			if !expressionEnd(previous) || !expressionStart(next) {
				return token.text
			}
			if lang != objc && ternary == 0 {
				return token.text
			}
			if ternary > 0 {
				ternary--
			}
		case "::":
			if !expressionEnd(previous) || !next.word {
				return token.text
			}
		case "...":
			if lang == java {
				if !previous.word || !next.word {
					return token.text
				}
			} else if (previous.text != "(" && previous.text != ",") || next.text != ")" {
				return token.text
			}
		case "(":
			if next.text == ")" && !previous.word {
				return token.text
			}
		case "@":
			if lang == c || next.text != "{" && !next.word && next.text != "\"\"" {
				return token.text
			}
		}
		if token.text == ";" && ternary != 0 {
			return token.text
		}
	}
	if ternary != 0 {
		return "?"
	}
	if headerState != 0 {
		return "switch"
	}
	return ""
}

// switchSelector admits one complete expression, rather than the declaration
// shapes that the enclosing statement grammar also permits.
func switchSelector(ts []bodyToken, lang bodyLanguage) bool {
	if len(ts) == 0 || !selectorStart(ts[0], lang) || !selectorEnd(ts[len(ts)-1], lang) || expr(ts, lang, nil) != "" {
		return false
	}
	for _, token := range ts {
		if literalShaped(token) && !literalToken(token, lang) {
			return false
		}
	}
	for i := 0; i+1 < len(ts); i++ {
		if selectorEnd(ts[i], lang) && selectorStart(ts[i+1], lang) && ts[i+1].text != "(" && ts[i+1].text != "[" && !(ts[i].text == ")" && selectorCastClose(ts, i)) {
			return false
		}
	}
	return true
}

// literalShaped: a token beginning with a digit or quote must be a literal.
func literalShaped(token bodyToken) bool {
	if token.text == "" {
		return false
	}
	first := token.text[0]
	return first >= '0' && first <= '9' || first == '\'' || first == '"'
}

func selectorStart(token bodyToken, lang bodyLanguage) bool {
	return token.word || literalToken(token, lang) || s.Contains("([!~+-*&", token.text)
}

func selectorEnd(token bodyToken, lang bodyLanguage) bool {
	return token.word || literalToken(token, lang) || token.text == ")" || token.text == "]" || token.text == "}" || token.text == ">" || token.text == "++" || token.text == "--"
}

func selectorCastClose(ts []bodyToken, close int) bool {
	depth := 0
	for i := close - 1; i >= 0; i-- {
		switch ts[i].text {
		case ")":
			depth++
		case "(":
			if depth != 0 {
				depth--
				continue
			}
			if i+1 == close {
				return false
			}
			for _, token := range ts[i+1 : close] {
				if !token.word && token.text != "*" && token.text != "." && token.text != "<" && token.text != ">" && token.text != "," {
					return false
				}
			}
			return true
		}
	}
	return false
}

// switchLabelColon: the colon closes a case or default label directly inside
// a switch block — one literal or dot-qualified word after case, nothing
// after default.
func switchLabelColon(ts []bodyToken, index int, lang bodyLanguage) bool {
	start := 0
	for j := index - 1; j >= 0; j-- {
		text := ts[j].text
		if text == ";" || text == "{" || text == "}" || text == ":" || text == "?" {
			start = j + 1
			break
		}
	}
	if start >= index || !ts[start].word {
		return false
	}
	if ts[start].text == "default" {
		return start == index-1
	}
	return ts[start].text == "case" && labelOperand(ts[start+1:index], lang)
}

// labelOperand: exactly one literal, or one dot-qualified all-word name.
func labelOperand(ts []bodyToken, lang bodyLanguage) bool {
	if len(ts) == 1 && literalToken(ts[0], lang) {
		return true
	}
	if len(ts) == 0 || len(ts)%2 == 0 {
		return false
	}
	for position, token := range ts {
		if position%2 == 1 && token.text != "." {
			return false
		}
		if position%2 == 0 && !token.word {
			return false
		}
	}
	return true
}

func literalToken(token bodyToken, lang bodyLanguage) bool {
	if token.text == "" {
		return false
	}
	if token.text[0] == '\'' || token.text[0] == '"' {
		return quotedLiteral(token.text, lang)
	}
	return numericLiteral(token.text, lang)
}

func quotedLiteral(value string, lang bodyLanguage) bool {
	quote := value[0]
	if (quote != '\'' && quote != '"') || len(value) < 2 || value[len(value)-1] != quote {
		return false
	}
	count := 0
	for i := 1; i < len(value)-1; i++ {
		if value[i] < ' ' || value[i] == quote {
			return false
		}
		if value[i] == '\\' {
			i++
			if i == len(value)-1 {
				return false
			}
		}
		count++
	}
	return quote == '"' || count != 0 && (lang != java || count == 1)
}

func numericLiteral(value string, lang bodyLanguage) bool {
	if len(value) == 0 || value[0] < '0' || value[0] > '9' {
		return false
	}
	i, base, floating, hexExponent := 0, 10, false, false
	if len(value) > 2 && value[0] == '0' && (value[1] == 'x' || value[1] == 'X') {
		i, base = 2, 16
	} else if len(value) > 2 && value[0] == '0' && (value[1] == 'b' || value[1] == 'B') {
		if lang != java {
			return false
		}
		i, base = 2, 2
	}
	var digits bool
	i, digits = literalDigits(value, i, base, lang)
	if !digits {
		return false
	}
	if i < len(value) && value[i] == '.' {
		floating, i = true, i+1
		i, _ = literalDigits(value, i, base, lang)
	}
	if i < len(value) && (value[i] == 'e' || value[i] == 'E' || base == 16 && (value[i] == 'p' || value[i] == 'P')) {
		if base == 16 && value[i] != 'p' && value[i] != 'P' {
			return false
		}
		floating, hexExponent, i = true, base == 16, i+1
		if i < len(value) && (value[i] == '+' || value[i] == '-') {
			i++
		}
		var exponent bool
		i, exponent = literalDigits(value, i, 10, lang)
		if !exponent {
			return false
		}
	}
	if base == 16 && floating && !hexExponent {
		return false
	}
	if base == 10 && !floating && value[0] == '0' && invalidOctal(value[:i]) {
		return false
	}
	if i < len(value) && (value[i] == 'f' || value[i] == 'F' || lang == java && (value[i] == 'd' || value[i] == 'D')) {
		if lang != java && !floating {
			return false
		}
		floating = true
	}
	return literalSuffix(value[i:], floating, lang)
}

func literalDigits(value string, i, base int, lang bodyLanguage) (int, bool) {
	start := i
	underscore := false
	for i < len(value) {
		if literalDigit(value[i], base) {
			underscore = false
			i++
			continue
		}
		if lang != java || value[i] != '_' || i == start || underscore || i+1 == len(value) || !literalDigit(value[i+1], base) {
			break
		}
		underscore = true
		i++
	}
	return i, i != start && !underscore
}

func literalDigit(value byte, base int) bool {
	if value >= '0' && value <= '9' {
		return int(value-'0') < base
	}
	if value >= 'a' && value <= 'f' {
		return base == 16
	}
	return value >= 'A' && value <= 'F' && base == 16
}

func invalidOctal(value string) bool {
	for i := range value {
		if value[i] == '8' || value[i] == '9' {
			return true
		}
	}
	return false
}

func literalSuffix(value string, floating bool, lang bodyLanguage) bool {
	if value == "" {
		return true
	}
	if lang == java {
		if len(value) != 1 {
			return false
		}
		if value[0] == 'l' || value[0] == 'L' {
			return !floating
		}
		return floating && (value[0] == 'f' || value[0] == 'F' || value[0] == 'd' || value[0] == 'D')
	}
	if floating {
		return len(value) == 1 && (value[0] == 'f' || value[0] == 'F' || value[0] == 'l' || value[0] == 'L')
	}
	// C: at most one u/U and one adjacent same-case long run, either order.
	sawUnsigned, sawLong := false, false
	for i := 0; i < len(value); {
		c := value[i]
		if (c == 'u' || c == 'U') && !sawUnsigned {
			sawUnsigned = true
			i++
			continue
		}
		if (c == 'l' || c == 'L') && !sawLong {
			sawLong = true
			i++
			if i < len(value) && value[i] == c {
				i++
			}
			continue
		}
		return false
	}
	return true
}

func bin(ts []bodyToken, i int, aggregate bool) bool {
	previous, next := bodyToken{}, bodyToken{}
	if i > 0 {
		previous = ts[i-1]
	}
	if i+1 < len(ts) {
		next = ts[i+1]
	}
	if !expressionEnd(previous) || !expressionStart(next) && !(aggregate && next.text == "{") {
		return false
	}
	return true
}

func unary(ts []bodyToken, i int, lang bodyLanguage) bool {
	previous, next := bodyToken{}, bodyToken{}
	if i > 0 {
		previous = ts[i-1]
	}
	if i+1 < len(ts) {
		next = ts[i+1]
	}
	if ts[i].text == "*" && next.text == ")" && lang != java && pointerTypeTail(ts, i) {
		return true
	}
	if ts[i].text == "*" && lang == objc && previous.word && (next.text == "," || next.text == ">") {
		return true
	}
	if !expressionEnd(previous) {
		if lang == java && (ts[i].text == "*" || ts[i].text == "&") {
			return false
		}
		if ts[i].text == "^" {
			return lang == objc && next.text == "{"
		}
	}
	if expressionEnd(previous) {
		return expressionStart(next)
	}
	return expressionStart(next) || lang == objc && ts[i].text == "^" && next.text == "{"
}

func commaStart(token bodyToken, lang bodyLanguage) bool {
	return expressionStart(token) || token.text == "}" || lang != java && (token.text == "." || token.text == "{" || lang == objc && token.text == "^") || lang == java && token.text == "@"
}

func commaEnd(token bodyToken, lang bodyLanguage) bool {
	return expressionEnd(token) || lang != java && token.text == "*"
}

func pointerTypeTail(ts []bodyToken, i int) bool {
	open := i - 1
	for open >= 0 && ts[open].text != "(" {
		if !ts[open].word && ts[open].text != "*" {
			return false
		}
		open--
	}
	return open >= 0 && open+1 < i
}

func genericTail(token bodyToken, lang bodyLanguage) bool {
	return lang == java && (token.text == ">" || token.text == "{" || token.text == "&") || lang == objc && (token.text == ";" || token.text == "*")
}

func genericClose(ts []bodyToken, i int, lang bodyLanguage) bool {
	if i == 0 || i+1 == len(ts) || !genericTail(ts[i+1], lang) || !expressionEnd(ts[i-1]) && !(lang == objc && ts[i-1].text == "*") {
		return false
	}
	start := i - 64
	if start < 0 {
		start = 0
	}
	for cursor := i - 1; cursor >= start; cursor-- {
		switch ts[cursor].text {
		case "<":
			return cursor > 0 && ts[cursor-1].word
		case ";", "{", "}":
			return false
		}
	}
	return false
}

func balancedTypeGenerics(ts []bodyToken, lang bodyLanguage) bool {
	if lang == c {
		return true
	}
	depth := 0
	for i, token := range ts {
		switch token.text {
		case "<":
			if genericOpening(ts, i, lang) {
				depth++
			}
		case ">":
			if depth != 0 {
				depth--
			}
		case ">>":
			if depth != 0 {
				if depth < 2 {
					return false
				}
				depth -= 2
			}
		case ";", "{", "}":
			if depth != 0 {
				return false
			}
		}
	}
	return depth == 0
}

func genericOpening(ts []bodyToken, i int, lang bodyLanguage) bool {
	if i == 0 || i+1 == len(ts) || !ts[i-1].word || !ts[i+1].word {
		return false
	}
	left, right := ts[i-1].text, ts[i+1].text
	if !upperIdentifier(right) {
		return false
	}
	return upperIdentifier(left) || lang == objc && left == "id"
}

func upperIdentifier(value string) bool {
	return len(value) != 0 && value[0] >= 'A' && value[0] <= 'Z'
}

func operandToken(token bodyToken) bool {
	return token.word || token.text != "" && (token.text[0] >= '0' && token.text[0] <= '9' || token.text[0] == '\'' || token.text[0] == '"' || token.text == "(" || token.text == "[" || token.text == "{")
}

func expressionStart(token bodyToken) bool {
	return token.word || token.text != "" && (token.text[0] >= '0' && token.text[0] <= '9' || token.text[0] == '\'' || token.text[0] == '"' || s.Contains("([!~+-*&", token.text))
}

func expressionEnd(token bodyToken) bool {
	return token.word || token.text != "" && (token.text[0] >= '0' && token.text[0] <= '9' || token.text[0] == '\'' || token.text[0] == '"' || token.text == ")" || token.text == "]" || token.text == "}" || token.text == ">" || token.text == "++" || token.text == "--")
}

func aggregateOpening(ts []bodyToken) bool {
	for _, token := range ts {
		if token.text == "=" || token.text == "new" || token.text == "[" {
			return true
		}
	}
	return false
}

func memberListOpening(ts []bodyToken) bool {
	for _, token := range ts {
		if token.text == "struct" || token.text == "union" || token.text == "enum" {
			return true
		}
	}
	return false
}

func blockOpening(ts []bodyToken, lang bodyLanguage) bool {
	if len(ts) == 0 {
		return false
	}
	if ts[len(ts)-1].text == "^" && lang == objc || ts[len(ts)-1].text == "->" && lang == java {
		return true
	}
	for _, token := range ts {
		switch token.text {
		case "if", "else", "for", "while", "switch", "catch", "try", "finally", "do", "synchronized", "class", "interface", "record", "enum", "static", "throws", "@autoreleasepool":
			return true
		}
	}
	if lang == objc && len(ts) >= 2 && ts[0].text == "@" && (ts[1].text == "interface" || ts[1].text == "implementation" || ts[1].text == "protocol") {
		return true
	}
	if lang == objc && len(ts) >= 2 && ts[0].text == "@" && ts[1].text == "autoreleasepool" {
		return true
	}
	if lang == objc && (ts[0].text == "-" || ts[0].text == "+") {
		return true
	}
	return ts[len(ts)-1].text == ")"
}

func terminatedStatement(ts []bodyToken, lang bodyLanguage) bool {
	if len(ts) == 0 {
		return true
	}
	if invalidAdjacency(ts, lang) {
		return false
	}
	last := ts[len(ts)-1]
	if !expressionEnd(last) && last.text != "return" && last.text != "break" && last.text != "continue" {
		return false
	}
	if last.text == "," || last.text == ":" {
		return false
	}
	onlyWords := true
	for _, token := range ts {
		onlyWords = onlyWords && token.word
	}
	if onlyWords {
		return validBareDeclaration(ts)
	}
	return true
}

func invalidAdjacency(ts []bodyToken, lang bodyLanguage) bool {
	assignment := -1
	for i, token := range ts {
		if assignmentOperator(token.text) {
			assignment = i
			break
		}
	}
	bracketMessage := []bool{}
	bracketParens := []int{}
	bracketRole := []objcMessageRole{}
	parens := 0
	for i := 0; i+1 < len(ts); i++ {
		switch ts[i].text {
		case "(":
			parens++
		case ")":
			if parens != 0 {
				parens--
			}
		case "[":
			bracketMessage = append(bracketMessage, lang == objc && (i == 0 || !expressionEnd(ts[i-1])))
			bracketParens = append(bracketParens, parens)
			bracketRole = append(bracketRole, objcReceiver)
		case "]":
			if len(bracketMessage) != 0 {
				bracketMessage = bracketMessage[:len(bracketMessage)-1]
				bracketParens = bracketParens[:len(bracketParens)-1]
				bracketRole = bracketRole[:len(bracketRole)-1]
			}
		case ":":
			if len(bracketMessage) != 0 && parens == bracketParens[len(bracketParens)-1] {
				bracketRole[len(bracketRole)-1] = objcArgument
			}
		}
		if len(bracketMessage) != 0 && bracketMessage[len(bracketMessage)-1] && parens == bracketParens[len(bracketParens)-1] && bracketRole[len(bracketRole)-1] == objcArgument && expressionEnd(ts[i]) && !(ts[i].text == ")" && pointerCastClose(ts, i)) {
			if ts[i+1].word {
				bracketRole[len(bracketRole)-1] = objcSelector
				if i+2 == len(ts) || ts[i+2].text != ":" {
					return true
				}
			}
			// A new primary after a completed argument is a second
			// unlabeled argument.
			if ts[i+1].text == "@" || literalShaped(ts[i+1]) {
				return true
			}
		}
		if !ts[i].word || !ts[i+1].word {
			continue
		}
		// A word pair directly inside brackets is valid only as ObjC
		// message adjacency: a selector word is followed by a colon, or
		// closes a colonless unary send. Subscripts admit no word pairs.
		if len(bracketMessage) != 0 && parens == bracketParens[len(bracketParens)-1] {
			if bracketMessage[len(bracketMessage)-1] && i+2 < len(ts) && (ts[i+2].text == ":" || ts[i+2].text == "]" && bracketRole[len(bracketRole)-1] != objcArgument) {
				continue
			}
			return true
		}
		if assignment >= 0 && i > assignment && !expressionWordPair(ts[i].text) {
			if lang != java && parens != 0 && pointerCastGroup(ts, i+2) {
				continue
			}
			return true
		}
		if i+2 < len(ts) && binaryOperator(ts[i+2].text) && !expressionWordPair(ts[i].text) {
			if i+2 == assignment {
				continue
			}
			if ts[i+2].text == "*" || ts[i+2].text == "&" {
				continue
			}
			return true
		}
	}
	for i := 2; i < len(ts); i++ {
		if !ts[i-1].word || !ts[i-2].word {
			continue
		}
		switch ts[i].text {
		case "+", "-", "/", "%", "|", "^", "&&", "||", "==", "!=", "<=", ">=", "<<", ">>":
			return ts[i-2].text != "return" && ts[i-2].text != "throw" && ts[i-2].text != "new"
		case "*", "&":
			if ts[i].text == "*" && i+1 < len(ts) && ts[i+1].text == ")" && pointerTypeTail(ts, i) {
				continue
			}
			return lang == java || !pointerPrefix(ts[:i])
		}
	}
	return false
}

// pointerCastGroup: the innermost open paren group closes as a pointer
// cast — its final token before the closing parenthesis is *.
func pointerCastGroup(ts []bodyToken, index int) bool {
	depth := 0
	for j := index; j < len(ts); j++ {
		switch ts[j].text {
		case "(":
			depth++
		case ")":
			if depth == 0 {
				return j > 0 && ts[j-1].text == "*"
			}
			depth--
		}
	}
	return false
}

func pointerCastClose(ts []bodyToken, close int) bool {
	depth := 0
	for i := close - 1; i >= 0; i-- {
		switch ts[i].text {
		case ")":
			depth++
		case "(":
			if depth != 0 {
				depth--
				continue
			}
			return i+1 < close && ts[close-1].text == "*"
		}
	}
	return false
}

func assignmentOperator(value string) bool {
	switch value {
	case "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=":
		return true
	}
	return false
}

func binaryOperator(value string) bool {
	switch value {
	case "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=", "+", "-", "*", "/", "%", "&", "|", "^", "&&", "||", "==", "!=", "<", "<=", ">", ">=", "<<", ">>":
		return true
	}
	return false
}

func expressionWordPair(left string) bool {
	switch left {
	case "return", "throw", "new", "instanceof", "extends", "implements", "permits":
		return true
	}
	return false
}

func pointerPrefix(ts []bodyToken) bool {
	if len(ts) == 1 {
		return true
	}
	if !ts[0].word {
		return false
	}
	switch ts[0].text {
	case "__strong", "__unsafe_unretained", "__weak", "const", "constant", "extern", "long", "short", "signed", "static", "unsigned", "void", "volatile":
		return true
	}
	return false
}

func validBareDeclaration(ts []bodyToken) bool {
	if len(ts) == 1 {
		return ts[0].text == "return" || ts[0].text == "break" || ts[0].text == "continue"
	}
	if len(ts) == 2 {
		return true
	}
	if len(ts) == 3 && ts[0].text == "using" && ts[1].text == "namespace" {
		return true
	}
	for i := 0; i < len(ts)-2; i++ {
		switch ts[i].text {
		case "const", "final", "private", "protected", "public", "static", "volatile", "transient", "unsigned", "signed", "long", "short", "extern", "register", "mutable":
		default:
			return false
		}
	}
	return true
}

func validAggregateTail(ts []bodyToken) bool {
	if len(ts) == 1 && ts[0].text == "," {
		return true
	}
	return len(ts) != 0 && expressionEnd(ts[len(ts)-1])
}

func asciiSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\f' || value == '\v' || value == '\n' || value == '\r'
}

func braceDelta(line string) int {
	return s.Count(line, "{") - s.Count(line, "}")
}

func balancedDelimiters(lines []sourceLine) bool {
	stack := make([]byte, 0, 32)
	for _, line := range lines {
		for i := 0; i < len(line.clean); i++ {
			switch line.clean[i] {
			case '(', '[', '{':
				stack = append(stack, line.clean[i])
			case ')', ']', '}':
				if len(stack) == 0 || !matchingDelimiter(stack[len(stack)-1], line.clean[i]) {
					return false
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
	return len(stack) == 0
}

func matchingDelimiter(open, close byte) bool {
	return open == '(' && close == ')' || open == '[' && close == ']' || open == '{' && close == '}'
}

func strip(body []byte) ([]byte, bool) {
	result := append([]byte(nil), body...)
	block, quote, character, escape, header := false, false, false, false, false
	lineStart := 0
	for i := 0; i < len(result); i++ {
		value := result[i]
		if value == 0 || value == '\r' {
			return nil, false
		}
		if value == '\n' {
			lineStart = i + 1
		}
		if block {
			result[i] = ' '
			if value == '*' && i+1 < len(result) && result[i+1] == '/' {
				result[i+1] = ' '
				i++
				block = false
			}
			continue
		}
		if header {
			if value == '\n' {
				return nil, false
			}
			if value == '"' {
				header = false
			}
			continue
		}
		if quote || character {
			if escape {
				result[i] = ' '
				escape = false
				continue
			}
			if value == '\\' {
				result[i] = ' '
				escape = true
				continue
			}
			if quote && value == '"' {
				quote = false
				continue
			}
			if character && value == '\'' {
				character = false
				continue
			}
			if value == '\n' {
				return nil, false
			}
			result[i] = ' '
			continue
		}
		if value == '/' && i+1 < len(result) && result[i+1] == '/' {
			for i < len(result) && result[i] != '\n' {
				result[i] = ' '
				i++
			}
			continue
		}
		if value == '/' && i+1 < len(result) && result[i+1] == '*' {
			result[i], result[i+1] = ' ', ' '
			i++
			block = true
			continue
		}
		if value == '"' {
			if headerQuote(result, lineStart, i) {
				header = true
			} else {
				quote = true
			}
			continue
		}
		if value == '\'' {
			character = true
		}
	}
	return result, !block && !quote && !character && !escape && !header
}

// headerQuote reports whether the quote at index opens an include operand:
// header-names are their own lexical category, never string literals.
func headerQuote(clean []byte, start, index int) bool {
	for start < index && (clean[start] == ' ' || clean[start] == '\t') {
		start++
	}
	for index > start && (clean[index-1] == ' ' || clean[index-1] == '\t') {
		index--
	}
	return directiveWord(clean[start:index], "#include") || directiveWord(clean[start:index], "#import")
}

func directiveWord(prefix []byte, word string) bool {
	if len(prefix) != len(word) {
		return false
	}
	for k := range prefix {
		if prefix[k] != word[k] {
			return false
		}
	}
	return true
}

func symbol(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i, value := range value {
		if !(value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || i > 0 && value >= '0' && value <= '9') {
			return false
		}
	}
	return true
}

func sourceAtom(value string) bool {
	if len(value) == 0 || len(value) > 256 {
		return false
	}
	for _, value := range value {
		if !(value == '_' || value == '/' || value == '.' || value == '-' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9') {
			return false
		}
	}
	return true
}

func qualified(value string) bool {
	if len(value) == 0 || s.HasPrefix(value, ".") || s.HasSuffix(value, ".") {
		return false
	}
	for _, part := range s.Split(value, ".") {
		if !symbol(part) {
			return false
		}
	}
	return true
}
