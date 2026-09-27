package compliance

import (
	"errors"
	"strings"
)

// Verdict 判定结果。
type Verdict string

const (
	Authorized Verdict = "authorized"
	Low        Verdict = "low"
	Medium     Verdict = "medium"
	High       Verdict = "high"
)

// WhitelistRule 白名单规则：名称、版本区间、可选的允许哈希集合。
type WhitelistRule struct {
	Name   string
	Range  string
	Hashes []string
}

// BlacklistRule 黑名单规则：名称加可选区间（空区间视同 *），或单个哈希。
type BlacklistRule struct {
	Name  string
	Range string
	Hash  string
}

// Entry 终端上报的发现记录。
type Entry struct {
	Name    string
	Version string
	SHA256  string
}

type wlRule struct {
	id     int
	name   string
	rng    VersionRange
	hashes map[string]bool
}

type blRule struct {
	id     int
	name   string
	rng    VersionRange
	hasRng bool
	hash   string
}

// Classifier 合规判定器。规则 ID 按添加顺序从 1 递增分配。
type Classifier struct {
	wl     []wlRule
	bl     []blRule
	nextID int
}

func NewClassifier() *Classifier { return &Classifier{nextID: 1} }

func normalizeName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func normalizeHash(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// AddWhitelist 添加白名单规则，区间非法时返回带字符偏移的错误。
// 返回值为分配的规则 ID。
func (c *Classifier) AddWhitelist(rule WhitelistRule) (int, error) {
	rng, err := parseRuleRange(rule.Range)
	if err != nil {
		return 0, err
	}
	var hashes map[string]bool
	if len(rule.Hashes) > 0 {
		hashes = make(map[string]bool, len(rule.Hashes))
		for _, h := range rule.Hashes {
			hashes[normalizeHash(h)] = true
		}
	}
	id := c.nextID
	c.nextID++
	c.wl = append(c.wl, wlRule{id: id, name: normalizeName(rule.Name), rng: rng, hashes: hashes})
	return id, nil
}

// AddBlacklist 添加黑名单规则，区间非法时返回带字符偏移的错误。
// 返回值为分配的规则 ID。
func (c *Classifier) AddBlacklist(rule BlacklistRule) (int, error) {
	r := blRule{id: c.nextID, hash: normalizeHash(rule.Hash)}
	if r.hash == "" {
		if strings.TrimSpace(rule.Name) == "" {
			return 0, errors.New("blacklist rule requires a name or a hash")
		}
		rng, err := parseRuleRange(rule.Range)
		if err != nil {
			return 0, err
		}
		r.name = normalizeName(rule.Name)
		r.rng = rng
		r.hasRng = true
	}
	c.nextID++
	c.bl = append(c.bl, r)
	return r.id, nil
}

// parseRuleRange 解析规则区间，空串视同 *。
func parseRuleRange(s string) (VersionRange, error) {
	if strings.TrimSpace(s) == "" {
		return ParseRange("*")
	}
	return ParseRange(s)
}

// Classify 判定发现记录，返回结论与命中规则 ID（多条取最小，无命中为 -1）。
func (c *Classifier) Classify(e Entry) (Verdict, int) {
	name := normalizeName(e.Name)
	hash := normalizeHash(e.SHA256)

	// 黑名单优先于一切
	hit := -1
	for _, r := range c.bl {
		if r.hash != "" {
			if hash != "" && hash == r.hash {
				hit = minID(hit, r.id)
			}
			continue
		}
		if r.name == name && r.rng.ContainsString(e.Version) {
			hit = minID(hit, r.id)
		}
	}
	if hit >= 0 {
		return High, hit
	}

	// 白名单
	sameName := -1
	auth := -1
	for _, r := range c.wl {
		if r.name != name {
			continue
		}
		sameName = minID(sameName, r.id)
		if !r.rng.ContainsString(e.Version) {
			continue
		}
		if len(r.hashes) == 0 || r.hashes[hash] {
			auth = minID(auth, r.id)
		}
	}
	switch {
	case auth >= 0:
		return Authorized, auth
	case sameName >= 0:
		return Low, sameName
	default:
		return Medium, -1
	}
}

func minID(cur, id int) int {
	if cur < 0 || id < cur {
		return id
	}
	return cur
}
