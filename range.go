package compliance

import (
	"fmt"
	"strings"
)

// RangeError 表示区间解析错误，Offset 为出错位置的字符偏移。
type RangeError struct {
	Offset int
	Msg    string
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("range error at offset %d: %s", e.Offset, e.Msg)
}

type comparator struct {
	op  string // "=", "!=", ">", ">=", "<", "<="
	ver Version
}

// VersionRange 是解析后的版本区间：子句间为或，子句内比较符为与。
type VersionRange struct {
	any     bool
	clauses [][]comparator
}

// IsAny 报告区间是否为通配全集（* 或 x）。
func (r VersionRange) IsAny() bool { return r.any }

// ParseRange 解析区间表达式。语法：比较符 = != > >= < <=，空格为与，
// || 为或；通配 1.2.x / 1.2.* / *；连字符区间 "1.2 - 1.4"。
// 非法表达式返回 *RangeError 并带字符偏移。
func ParseRange(s string) (VersionRange, error) {
	if strings.TrimSpace(s) == "" {
		return VersionRange{}, &RangeError{Offset: 0, Msg: "empty range expression"}
	}
	var r VersionRange
	pos := 0
	for pos <= len(s) {
		end := len(s)
		if i := strings.Index(s[pos:], "||"); i >= 0 {
			end = pos + i
		}
		clause, any, err := parseClause(s[pos:end], pos)
		if err != nil {
			return VersionRange{}, err
		}
		if any {
			r.any = true
		} else {
			r.clauses = append(r.clauses, clause)
		}
		if end == len(s) {
			break
		}
		pos = end + 2
	}
	if r.any {
		r.clauses = nil
	}
	if !r.any && len(r.clauses) == 0 {
		return VersionRange{}, &RangeError{Offset: 0, Msg: "empty range expression"}
	}
	return r, nil
}

// token 是带偏移的词法单元。
type token struct {
	text string
	off  int
}

func splitTokens(s string, base int) []token {
	var toks []token
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		j := i
		for j < len(s) && s[j] != ' ' && s[j] != '\t' {
			j++
		}
		toks = append(toks, token{text: s[i:j], off: base + i})
		i = j
	}
	return toks
}

// parseClause 解析 || 之间的一个与子句。any 为 true 表示该子句匹配一切。
func parseClause(s string, base int) ([]comparator, bool, error) {
	toks := splitTokens(s, base)
	if len(toks) == 0 {
		return nil, false, &RangeError{Offset: base, Msg: "empty clause"}
	}
	// 连字符区间：恰好三个词且中间为 "-"
	dash := -1
	for i, t := range toks {
		if t.text == "-" {
			if dash >= 0 {
				return nil, false, &RangeError{Offset: t.off, Msg: "multiple hyphens"}
			}
			dash = i
		}
	}
	if dash >= 0 {
		if len(toks) != 3 || dash != 1 {
			off := toks[dash].off
			return nil, false, &RangeError{Offset: off, Msg: "malformed hyphen range"}
		}
		return parseHyphen(toks[0], toks[2])
	}
	var clause []comparator
	for _, t := range toks {
		if t.text == "*" || t.text == "x" || t.text == "X" {
			continue // 全集比较符，与语义下为空操作
		}
		cs, err := parseComparator(t)
		if err != nil {
			return nil, false, err
		}
		clause = append(clause, cs...)
	}
	if len(clause) == 0 {
		return nil, true, nil
	}
	return clause, false, nil
}

// parseHyphen 解析 "A - B"：等价于 >=A 且 <(B 末段加一)；
// B 带预发布时上界为 <=B。
func parseHyphen(lo, hi token) ([]comparator, bool, error) {
	vlo, err := parseOperand(lo.text, lo.off)
	if err != nil {
		return nil, false, err
	}
	vhi, err := parseOperand(hi.text, hi.off)
	if err != nil {
		return nil, false, err
	}
	out := []comparator{{op: ">=", ver: vlo}}
	if vhi.hasPre {
		out = append(out, comparator{op: "<=", ver: vhi})
	} else {
		inc := make([]string, len(vhi.nums))
		copy(inc, vhi.nums)
		inc[len(inc)-1] = incrementNumStr(inc[len(inc)-1])
		out = append(out, comparator{op: "<", ver: Version{nums: inc}})
	}
	return out, false, nil
}

// incrementNumStr 十进制非负整数字符串加一（任意长度）。
func incrementNumStr(s string) string {
	b := []byte(s)
	carry := byte(1)
	for i := len(b) - 1; i >= 0 && carry > 0; i-- {
		d := b[i] - '0' + carry
		b[i] = '0' + d%10
		carry = d / 10
	}
	if carry > 0 {
		return "1" + string(b)
	}
	return string(b)
}

// parseComparator 解析单个比较符词元，通配符可能展开为多比较符。
func parseComparator(t token) ([]comparator, error) {
	op := ""
	rest := t.text
	hasOp := false
	for _, p := range []string{">=", "<=", "!=", "=", ">", "<"} {
		if strings.HasPrefix(rest, p) {
			op = p
			rest = rest[len(p):]
			hasOp = true
			break
		}
	}
	if op == "" {
		op = "="
	}
	if rest == "" {
		return nil, &RangeError{Offset: t.off + len(t.text), Msg: "missing version after operator"}
	}
	if hasWildcard(rest) {
		if hasOp && op != "=" {
			return nil, &RangeError{Offset: t.off, Msg: "operator not allowed with wildcard"}
		}
		return wildcardComparators(rest, t.off+len(t.text)-len(rest))
	}
	v, err := parseOperand(rest, t.off+len(t.text)-len(rest))
	if err != nil {
		return nil, err
	}
	return []comparator{{op: op, ver: v}}, nil
}

func hasWildcard(s string) bool {
	s = strings.ReplaceAll(s, ",", ".")
	for _, p := range strings.Split(s, ".") {
		if p == "x" || p == "X" || p == "*" {
			return true
		}
	}
	return false
}

// wildcardComparators 把 1.2.x 形式展开为 >=1.2 <1.3。
func wildcardComparators(s string, off int) ([]comparator, error) {
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.Split(s, ".")
	wild := -1
	for i, p := range parts {
		if p == "x" || p == "X" || p == "*" {
			wild = i
			break
		}
	}
	if wild < 0 {
		return nil, &RangeError{Offset: off, Msg: "invalid wildcard"}
	}
	for i := wild + 1; i < len(parts); i++ {
		if parts[i] != "" {
			return nil, &RangeError{Offset: off, Msg: "wildcard must be the last segment"}
		}
	}
	if wild == 0 {
		return nil, nil // 全集
	}
	nums := make([]string, wild)
	for i := 0; i < wild; i++ {
		if !isDigits(parts[i]) {
			return nil, &RangeError{Offset: off, Msg: "invalid numeric segment before wildcard"}
		}
		nums[i] = normalizeNum(parts[i])
	}
	lower := comparator{op: ">=", ver: Version{nums: nums}}
	inc := make([]string, wild)
	copy(inc, nums)
	inc[wild-1] = incrementNumStr(inc[wild-1])
	upper := comparator{op: "<", ver: Version{nums: inc}}
	return []comparator{lower, upper}, nil
}

// parseOperand 解析比较符操作数（不含通配符）。
func parseOperand(s string, off int) (Version, error) {
	v, err := ParseVersion(s)
	if err != nil {
		return Version{}, &RangeError{Offset: off, Msg: "invalid version operand"}
	}
	return v, nil
}

// Contains 判断已解析版本是否在区间内。
func (r VersionRange) Contains(v Version) bool {
	if r.any {
		return true
	}
	for _, clause := range r.clauses {
		ok := true
		for _, c := range clause {
			if !satisfies(c, v) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// ContainsString 解析并判断版本字符串；不可解析的版本只有全集区间放行。
func (r VersionRange) ContainsString(s string) bool {
	v, err := ParseVersion(s)
	if err != nil {
		return r.any
	}
	return r.Contains(v)
}

func satisfies(c comparator, v Version) bool {
	cmp := compareVersion(v, c.ver)
	switch c.op {
	case "=":
		return cmp == 0
	case "!=":
		return cmp != 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<=":
		return cmp <= 0
	case "<":
		if cmp >= 0 {
			return false
		}
		// 严格上界不带预发布时，不放行与上界同号的预发布
		if v.hasPre && !c.ver.hasPre && sameNums(v, c.ver) {
			return false
		}
		return true
	}
	return false
}

// sameNums 判断两版本数字段是否相同（缺失尾段补 0）。
func sameNums(a, b Version) bool {
	n := len(a.nums)
	if len(b.nums) > n {
		n = len(b.nums)
	}
	for i := 0; i < n; i++ {
		av, bv := "0", "0"
		if i < len(a.nums) {
			av = a.nums[i]
		}
		if i < len(b.nums) {
			bv = b.nums[i]
		}
		if av != bv {
			return false
		}
	}
	return true
}
