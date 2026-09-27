package compliance

import (
	"strings"
	"testing"
)

func sign(a, b string) int {
	c, err := Compare(a, b)
	if err != nil {
		panic(err)
	}
	return c
}

func TestCompareTable(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2", "1.2.0.0", 0},
		{"1.02", "1.2", 0},
		{"1.10", "1.9", 1},
		{"10,0,19041,1", "10.0.19041.1", 0},
		{"10.0.19041.1 (WinBuild.160101.0800)", "10.0.19041.1", 0},
		{"v1.2.3", "1.2.3", 0},
		{"  1.2.3  ", "1.2.3", 0},
		{"1.0.0+build.42", "1.0.0", 0},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
		{"1.0.0-alpha.beta", "1.0.0-beta", -1},
		{"1.0.0-beta", "1.0.0-beta.2", -1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1},
		{"1.0.0-beta.11", "1.0.0-rc.1", -1},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0-rc.1+meta", "1.0.0-rc.1", 0},
		{"2", "10", -1},
		{"123456789012345678901234567890123456", "123456789012345678901234567890123455", 1},
		{"1.2.3.4.5.6.7.8.9.10.11", "1.2.3.4.5.6.7.8.9.10.10", 1},
	}
	for _, c := range cases {
		got, err := Compare(c.a, c.b)
		if err != nil {
			t.Errorf("Compare(%q,%q) error: %v", c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("Compare(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
		// 反对称
		rev, _ := Compare(c.b, c.a)
		if rev != -c.want {
			t.Errorf("Compare(%q,%q) = %d, want %d", c.b, c.a, rev, -c.want)
		}
	}
}

func TestCompareUnparseable(t *testing.T) {
	for _, s := range []string{"", "   ", "unknown", "2019 R2", "1..2", "1.2-", "v"} {
		if _, err := Compare(s, "1.0"); err == nil {
			t.Errorf("Compare(%q, 1.0) expected error", s)
		}
		if _, err := Compare("1.0", s); err == nil {
			t.Errorf("Compare(1.0, %q) expected error", s)
		}
	}
}

func contains(t *testing.T, expr, ver string) bool {
	t.Helper()
	r, err := ParseRange(expr)
	if err != nil {
		t.Fatalf("ParseRange(%q) error: %v", expr, err)
	}
	return r.ContainsString(ver)
}

func TestRangeTable(t *testing.T) {
	cases := []struct {
		expr, ver string
		want      bool
	}{
		{"=1.2", "1.2", true},
		{"=1.2", "1.2.0", true},
		{"=1.2", "1.2.0.5", false},
		{"=1.2", "1.2.0.0", true},
		{"!=1.2", "1.2.0.5", true},
		{"!=1.2", "1.2.0", false},
		{">1.2", "1.2.0.5", true},
		{">1.2", "1.2", false},
		{">=1.2 <2.0", "1.5", true},
		{">=1.2 <2.0", "2.0", false},
		{">=1.2 <2.0", "1.1", false},
		{"<1.2 || >=2.0", "1.0", true},
		{"<1.2 || >=2.0", "1.5", false},
		{"<1.2 || >=2.0", "2.5", true},
		{"1.2 - 1.4", "1.2", true},
		{"1.2 - 1.4", "1.4.9", true},
		{"1.2 - 1.4", "1.5", false},
		{"1.2 - 1.4.0", "1.4.0", true},
		{"1.2 - 1.4.0", "1.4.1", false},
		{"1.2.x", "1.02.7", true},
		{"1.2.x", "1.2.0-beta", false},
		{"1.2.x", "1.3.0", false},
		{"1.2.*", "1.2.99", true},
		{"1.x", "1.999.0", true},
		{"1.x", "2.0.0-rc.1", false},
		{"<2.0", "2.0.0-rc.1", false},
		{"<2.0", "1.5.0-beta", true},
		{">=1.2", "1.5.0-beta", true},
		{">=1.2", "1.2.0-beta", false},
		{"*", "anything unparseable", true},
		{"*", "1.2.3", true},
		{"x", "unknown", true},
		{">=1.2", "unknown", false},
		{"1.2.x", "unknown", false},
		{"<=2.0", "2.0.0-rc.1", true},
		{"=1.2.0-beta", "1.2.0-beta", true},
		{">=1.2.0-alpha <1.2.0", "1.2.0-beta", false},
	}
	for _, c := range cases {
		if got := contains(t, c.expr, c.ver); got != c.want {
			t.Errorf("range %q contains %q = %v, want %v", c.expr, c.ver, got, c.want)
		}
	}
}

func TestRangeInvalid(t *testing.T) {
	cases := []struct {
		expr    string
		wantOff int
	}{
		{">=", 2},
		{"1.2 -", 4},
		{">=1.2 ||", 8},
		{"1.x.3", 0},
		{"", 0},
		{">=1.2 || ", 8},
		{"1.2 - 1.4 - 1.5", 10},
		{">=1.2.x", 0},
		{"=abc", 1},
	}
	for _, c := range cases {
		_, err := ParseRange(c.expr)
		if err == nil {
			t.Errorf("ParseRange(%q) expected error", c.expr)
			continue
		}
		re, ok := err.(*RangeError)
		if !ok {
			t.Errorf("ParseRange(%q) error type %T, want *RangeError", c.expr, err)
			continue
		}
		if re.Offset != c.wantOff {
			t.Errorf("ParseRange(%q) offset = %d, want %d", c.expr, re.Offset, c.wantOff)
		}
	}
}

func newTestClassifier(t *testing.T) *Classifier {
	t.Helper()
	c := NewClassifier()
	// id 1: 白名单 git >=2.30 <2.40，哈希集合为空
	if _, err := c.AddWhitelist(WhitelistRule{Name: " Git ", Range: ">=2.30 <2.40"}); err != nil {
		t.Fatal(err)
	}
	// id 2: 白名单 git 2.40.x，限定哈希
	if _, err := c.AddWhitelist(WhitelistRule{Name: "git", Range: "2.40.x", Hashes: []string{"ABCDEF"}}); err != nil {
		t.Fatal(err)
	}
	// id 3: 黑名单 git <2.30
	if _, err := c.AddBlacklist(BlacklistRule{Name: "GIT", Range: "<2.30"}); err != nil {
		t.Fatal(err)
	}
	// id 4: 黑名单单个哈希
	if _, err := c.AddBlacklist(BlacklistRule{Hash: "DEADBEEF"}); err != nil {
		t.Fatal(err)
	}
	// id 5: 黑名单 openssl 无区间（视同 *）
	if _, err := c.AddBlacklist(BlacklistRule{Name: "openssl"}); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClassify(t *testing.T) {
	c := newTestClassifier(t)
	cases := []struct {
		name    string
		entry   Entry
		verdict Verdict
		ruleID  int
	}{
		{"黑名单区间命中", Entry{Name: "git", Version: "2.29.0"}, High, 3},
		{"黑名单哈希命中且大写", Entry{Name: "anything", Version: "9.9", SHA256: "deadbeef"}, High, 4},
		{"黑名单无区间视同星号", Entry{Name: "OpenSSL", Version: "3.0.1"}, High, 5},
		{"黑名单优先于白名单", Entry{Name: "git", Version: "2.29.0", SHA256: "abcdef"}, High, 3},
		{"白名单区间命中无哈希集", Entry{Name: "git", Version: "2.35.1"}, Authorized, 1},
		{"白名单名称忽略大小写空白", Entry{Name: "  GIT ", Version: "2.35.1"}, Authorized, 1},
		{"白名单哈希集含该哈希", Entry{Name: "git", Version: "2.40.2", SHA256: "abcdef"}, Authorized, 2},
		{"白名单哈希集不含该哈希", Entry{Name: "git", Version: "2.40.2", SHA256: "9999"}, Low, 1},
		{"白名单同名但版本不在区间", Entry{Name: "git", Version: "3.0.0"}, Low, 1},
		{"无同名规则", Entry{Name: "curl", Version: "8.0.0"}, Medium, -1},
		{"不可解析版本白名单非星号", Entry{Name: "git", Version: "unknown"}, Low, 1},
		{"不可解析版本黑名单非星号不命中", Entry{Name: "git", Version: "unknown", SHA256: "00"}, Low, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, id := c.Classify(tc.entry)
			if v != tc.verdict || id != tc.ruleID {
				t.Errorf("Classify(%+v) = (%s,%d), want (%s,%d)", tc.entry, v, id, tc.verdict, tc.ruleID)
			}
		})
	}
}

func TestClassifyMinRuleID(t *testing.T) {
	c := NewClassifier()
	id1, _ := c.AddWhitelist(WhitelistRule{Name: "a", Range: ">=1.0"})
	if _, err := c.AddWhitelist(WhitelistRule{Name: "a", Range: ">=2.0"}); err != nil {
		t.Fatal(err)
	}
	v, id := c.Classify(Entry{Name: "a", Version: "3.0"})
	if v != Authorized || id != id1 {
		t.Errorf("got (%s,%d), want (authorized,%d)", v, id, id1)
	}
	b1, _ := c.AddBlacklist(BlacklistRule{Name: "b", Range: "<5.0"})
	if _, err := c.AddBlacklist(BlacklistRule{Hash: "ff"}); err != nil {
		t.Fatal(err)
	}
	v, id = c.Classify(Entry{Name: "b", Version: "1.0", SHA256: "FF"})
	if v != High || id != b1 {
		t.Errorf("got (%s,%d), want (high,%d)", v, id, b1)
	}
}

func TestAddRuleRangeError(t *testing.T) {
	c := NewClassifier()
	if _, err := c.AddWhitelist(WhitelistRule{Name: "a", Range: ">="}); err == nil {
		t.Error("AddWhitelist with invalid range expected error")
	} else if re, ok := err.(*RangeError); !ok || re.Offset != 2 {
		t.Errorf("want *RangeError offset 2, got %v", err)
	}
	if _, err := c.AddBlacklist(BlacklistRule{Name: "a", Range: "1.x.3"}); err == nil {
		t.Error("AddBlacklist with invalid range expected error")
	}
	if _, err := c.AddBlacklist(BlacklistRule{}); err == nil {
		t.Error("AddBlacklist with neither name nor hash expected error")
	}
	// 哈希规则不解析区间
	if _, err := c.AddBlacklist(BlacklistRule{Hash: "aa", Range: "not a range"}); err != nil {
		t.Errorf("hash-only blacklist rule should ignore range, got %v", err)
	}
}

func TestClassifyArbitraryBytesNoPanic(t *testing.T) {
	c := newTestClassifier(t)
	inputs := []string{
		"", "\x00\x01\x02", "GIT", "git\x00", "\xff\xfe", "1.2.3\x00-beta",
		strings.Repeat("9", 5000), "\t\n ", "((()))", "1.2.3 (unclosed",
	}
	for _, n := range inputs {
		for _, v := range inputs {
			for _, h := range inputs {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("panic on %+v: %v", Entry{Name: n, Version: v, SHA256: h}, r)
						}
					}()
					c.Classify(Entry{Name: n, Version: v, SHA256: h})
				}()
			}
		}
	}
}
