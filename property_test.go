package compliance

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// canonical 返回版本的规范串：数字段去前导零、去尾部零段，保留预发布。
func canonical(v Version) string {
	nums := v.nums
	for len(nums) > 1 && nums[len(nums)-1] == "0" {
		nums = nums[:len(nums)-1]
	}
	s := strings.Join(nums, ".")
	if v.hasPre {
		s += "-" + strings.Join(v.pre, ".")
	}
	return s
}

// randNumSegment 生成随机数字段，可能带前导零或长达 30+ 位。
func randNumSegment(r *rand.Rand) string {
	n := r.Intn(4)
	var l int
	switch n {
	case 0:
		l = 1 + r.Intn(3)
	case 1:
		l = 1 + r.Intn(2) // 可能前导零
	case 2:
		l = 30 + r.Intn(10) // 30+ 位
	default:
		l = 1 + r.Intn(6)
	}
	var b strings.Builder
	for i := 0; i < l; i++ {
		b.WriteByte(byte('0' + r.Intn(10)))
	}
	return b.String()
}

// randBaseVersion 生成随机基础版本（数字段 + 可选预发布）。
func randBaseVersion(r *rand.Rand) string {
	segs := 1 + r.Intn(12) // 可能十段以上
	parts := make([]string, segs)
	for i := range parts {
		parts[i] = randNumSegment(r)
	}
	s := strings.Join(parts, ".")
	if r.Intn(3) == 0 {
		// 预发布
		n := 1 + r.Intn(3)
		ids := make([]string, n)
		for i := range ids {
			switch r.Intn(3) {
			case 0:
				ids[i] = randNumSegment(r)
			case 1:
				ids[i] = []string{"alpha", "beta", "rc", "pre"}[r.Intn(4)]
			default:
				ids[i] = fmt.Sprintf("rc.%d", r.Intn(5))
			}
		}
		s += "-" + strings.Join(ids, ".")
	}
	return s
}

// mutate 生成等价变体：前导 v、空白、逗号、括号尾缀、构建元数据、尾段补零、前导零。
func mutate(r *rand.Rand, base string) string {
	v, err := ParseVersion(base)
	if err != nil {
		return base
	}
	nums := make([]string, len(v.nums))
	copy(nums, v.nums)
	// 随机补前导零
	for i := range nums {
		if r.Intn(3) == 0 {
			nums[i] = strings.Repeat("0", r.Intn(3)) + nums[i]
		}
	}
	// 尾段补零
	for r.Intn(2) == 0 && len(nums) < 14 {
		nums = append(nums, "0")
	}
	sep := "."
	if r.Intn(4) == 0 {
		sep = ","
	}
	s := strings.Join(nums, sep)
	if v.hasPre {
		s += "-" + strings.Join(v.pre, ".")
	}
	if r.Intn(3) == 0 {
		s += "+build." + randNumSegment(r)
	}
	if r.Intn(4) == 0 {
		s += " (WinBuild.160101.0800)"
	}
	if r.Intn(4) == 0 {
		s = "v" + s
	}
	if r.Intn(4) == 0 {
		s = "  " + s + " "
	}
	return s
}

// randRange 生成随机合法区间表达式。
func randRange(r *rand.Rand) string {
	clauses := 1 + r.Intn(3)
	parts := make([]string, clauses)
	for i := range parts {
		parts[i] = randClause(r)
	}
	return strings.Join(parts, " || ")
}

func randClause(r *rand.Rand) string {
	switch r.Intn(6) {
	case 0:
		return "*"
	case 1:
		// 连字符区间
		a := randBaseVersion(r)
		b := randBaseVersion(r)
		return a + " - " + b
	case 2:
		// 通配
		segs := 1 + r.Intn(3)
		parts := make([]string, segs)
		for i := range parts {
			parts[i] = randNumSegment(r)
		}
		wild := "x"
		if r.Intn(2) == 0 {
			wild = "*"
		}
		return strings.Join(append(parts, wild), ".")
	default:
		// 1-3 个比较符
		n := 1 + r.Intn(3)
		cs := make([]string, n)
		for i := range cs {
			op := []string{"=", "!=", ">", ">=", "<", "<="}[r.Intn(6)]
			cs[i] = op + randBaseVersion(r)
		}
		return strings.Join(cs, " ")
	}
}

func TestPropertyCompareTotalOrder(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	var vers []Version
	for i := 0; i < 200; i++ {
		s := randBaseVersion(r)
		v, err := ParseVersion(s)
		if err != nil {
			t.Fatalf("generated version %q not parseable: %v", s, err)
		}
		vers = append(vers, v)
	}
	// 自反与反对称
	for _, a := range vers {
		if compareVersion(a, a) != 0 {
			t.Fatalf("Compare(%s,%s) != 0", canonical(a), canonical(a))
		}
		for _, b := range vers {
			if compareVersion(a, b) != -compareVersion(b, a) {
				t.Fatalf("antisymmetry violated: %s vs %s", canonical(a), canonical(b))
			}
		}
	}
	// 传递性（抽样三元组）
	for i := 0; i < 20000; i++ {
		a := vers[r.Intn(len(vers))]
		b := vers[r.Intn(len(vers))]
		c := vers[r.Intn(len(vers))]
		if compareVersion(a, b) <= 0 && compareVersion(b, c) <= 0 && compareVersion(a, c) > 0 {
			t.Fatalf("transitivity violated: %s <= %s <= %s but %s > %s",
				canonical(a), canonical(b), canonical(c), canonical(a), canonical(c))
		}
	}
}

func TestPropertyCompareZeroIffSameRangeDecisions(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	// 生成基础版本及等价变体
	var versions []string
	for i := 0; i < 60; i++ {
		base := randBaseVersion(r)
		versions = append(versions, base)
		for j := 0; j < 3; j++ {
			versions = append(versions, mutate(r, base))
		}
	}
	// 解析并分组
	type item struct {
		raw   string
		v     Version
		canon string
	}
	var items []item
	for _, s := range versions {
		v, err := ParseVersion(s)
		if err != nil {
			t.Fatalf("version %q unparseable", s)
		}
		items = append(items, item{raw: s, v: v, canon: canonical(v)})
	}
	// 随机合法区间
	var ranges []VersionRange
	for i := 0; i < 150; i++ {
		expr := randRange(r)
		rg, err := ParseRange(expr)
		if err != nil {
			t.Fatalf("generated range %q invalid: %v", expr, err)
		}
		ranges = append(ranges, rg)
	}
	// Compare==0 当且仅当 canon 相同；同组版本对所有区间判定一致
	for i := range items {
		for j := range items {
			eq := compareVersion(items[i].v, items[j].v) == 0
			if eq != (items[i].canon == items[j].canon) {
				t.Fatalf("Compare==0 mismatch canon: %q vs %q", items[i].raw, items[j].raw)
			}
			if !eq {
				continue
			}
			for k, rg := range ranges {
				if rg.Contains(items[i].v) != rg.Contains(items[j].v) {
					t.Fatalf("Compare(%q,%q)==0 but range #%d differs", items[i].raw, items[j].raw, k)
				}
			}
		}
	}
	// Compare!=0 时，=canon(a) 区间可区分二者
	for i := range items {
		for _, j := range r.Perm(len(items))[:20] {
			if compareVersion(items[i].v, items[j].v) == 0 {
				continue
			}
			rg, err := ParseRange("=" + items[i].canon)
			if err != nil {
				t.Fatalf("canonical range parse failed: %v", err)
			}
			if !rg.Contains(items[i].v) || rg.Contains(items[j].v) {
				t.Fatalf("=%s fails to distinguish %q from %q", items[i].canon, items[i].raw, items[j].raw)
			}
		}
	}
}

func TestPropertyUnparseableOnlyStar(t *testing.T) {
	r := rand.New(rand.NewSource(99))
	invalid := []string{"", "unknown", "2019 R2", "1..2", "1.2-", "v", "1.2.3 ", "abc.def"}
	for i := 0; i < 100; i++ {
		expr := randRange(r)
		rg, err := ParseRange(expr)
		if err != nil {
			t.Fatalf("generated range %q invalid: %v", expr, err)
		}
		for _, s := range invalid {
			if _, err := ParseVersion(s); err == nil {
				continue // 实际可解析，跳过
			}
			got := rg.ContainsString(s)
			if got != rg.any {
				t.Fatalf("range %q contains unparseable %q = %v, want %v", expr, s, got, rg.any)
			}
		}
	}
}
