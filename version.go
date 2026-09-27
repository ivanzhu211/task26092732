package compliance

import (
	"errors"
	"strings"
)

// Version 是解析后的版本号。数字段以去前导零的十进制字符串保存，
// 支持任意长度与任意段数；pre 为预发布标识（空表示正式版）。
type Version struct {
	nums   []string
	pre    []string
	hasPre bool
}

// ErrUnparseable 表示版本字符串不可解析。
var ErrUnparseable = errors.New("unparseable version")

// ParseVersion 解析版本字符串：去首尾空白与前导 v，忽略末尾括号段与
// + 后的构建元数据，逗号视同点号，- 后为预发布标识。
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	if len(s) > 0 && (s[0] == 'v' || s[0] == 'V') {
		s = s[1:]
	}
	// 末尾括号段忽略
	if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(strings.TrimSpace(s), ")") {
		s = strings.TrimSpace(s[:i])
	}
	// + 后构建元数据忽略
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	main := s
	pre := ""
	hasPre := false
	if i := strings.IndexByte(s, '-'); i >= 0 {
		main, pre, hasPre = s[:i], s[i+1:], true
	}
	main = strings.TrimSpace(main)
	main = strings.ReplaceAll(main, ",", ".")
	if main == "" {
		return Version{}, ErrUnparseable
	}
	parts := strings.Split(main, ".")
	nums := make([]string, len(parts))
	for i, p := range parts {
		if p == "" || !isDigits(p) {
			return Version{}, ErrUnparseable
		}
		nums[i] = normalizeNum(p)
	}
	var preIDs []string
	if hasPre {
		if pre == "" {
			return Version{}, ErrUnparseable
		}
		preIDs = strings.Split(pre, ".")
		for _, id := range preIDs {
			if id == "" {
				return Version{}, ErrUnparseable
			}
		}
	}
	return Version{nums: nums, pre: preIDs, hasPre: hasPre}, nil
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}

// normalizeNum 去除前导零，全零归一为 "0"。
func normalizeNum(s string) string {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	return s
}

// compareNumStr 比较两个已归一化的非负整数字符串。
func compareNumStr(a, b string) int {
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

// compareVersion 比较两个已解析版本：先按数字段（缺失尾段补 0），
// 再按 SemVer 2.0 比较预发布标识；预发布版低于同号正式版。
func compareVersion(a, b Version) int {
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
		if c := compareNumStr(av, bv); c != 0 {
			return c
		}
	}
	if a.hasPre != b.hasPre {
		if a.hasPre {
			return -1
		}
		return 1
	}
	if !a.hasPre {
		return 0
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if c := comparePreID(a.pre[i], b.pre[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a.pre) < len(b.pre):
		return -1
	case len(a.pre) > len(b.pre):
		return 1
	}
	return 0
}

// comparePreID 按 SemVer 2.0 比较预发布标识：纯数字按数值且小于字母数字，
// 字母数字按 ASCII 字典序。
func comparePreID(a, b string) int {
	an, bn := isDigits(a), isDigits(b)
	if an && bn {
		return compareNumStr(normalizeNum(a), normalizeNum(b))
	}
	if an {
		return -1
	}
	if bn {
		return 1
	}
	return strings.Compare(a, b)
}

// Compare 比较两个版本字符串，返回 -1/0/1；任一不可解析时报错。
func Compare(a, b string) (int, error) {
	va, err := ParseVersion(a)
	if err != nil {
		return 0, err
	}
	vb, err := ParseVersion(b)
	if err != nil {
		return 0, err
	}
	return compareVersion(va, vb), nil
}
