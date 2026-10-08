package service

import (
	"net/netip"
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

type prefixNode struct {
	children [2]*prefixNode
	rule     *store.IpNet
}
type RuleIndex struct {
	exact      map[netip.Addr]store.IpNet
	allow, ban [2]*prefixNode
	revision   uint64
}

func parsePrefix(value string) (netip.Prefix, bool) {
	if addr, err := netip.ParseAddr(value); err == nil {
		addr = addr.Unmap()
		return netip.PrefixFrom(addr, addr.BitLen()), true
	}
	p, err := netip.ParsePrefix(value)
	if err != nil {
		return netip.Prefix{}, false
	}
	if p.Addr().Is4In6() && p.Bits() >= 96 {
		p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
	}
	return p.Masked(), true
}
func addressBits(addr netip.Addr) ([16]byte, int) {
	if addr.Is4() {
		var out [16]byte
		a := addr.As4()
		copy(out[:], a[:])
		return out, 0
	}
	return addr.As16(), 1
}
func NewRuleIndex(rules []store.IpNet, revision uint64) *RuleIndex {
	idx := &RuleIndex{exact: map[netip.Addr]store.IpNet{}, revision: revision}
	for _, r := range rules {
		p, ok := parsePrefix(r.IpNet)
		if !ok {
			continue
		}
		addr := p.Addr()
		bits, family := addressBits(addr)
		if p.Bits() == addr.BitLen() {
			if previous, ok := idx.exact[addr]; !ok || previous.Action != store.ActionAllow || !activeRule(previous, time.Now()) {
				idx.exact[addr] = r
			}
			continue
		}
		roots := &idx.ban
		if r.Action == store.ActionAllow {
			roots = &idx.allow
		}
		if roots[family] == nil {
			roots[family] = &prefixNode{}
		}
		node := roots[family]
		for b := 0; b < p.Bits(); b++ {
			bit := (bits[b/8] >> uint(7-b%8)) & 1
			if node.children[bit] == nil {
				node.children[bit] = &prefixNode{}
			}
			node = node.children[bit]
		}
		cp := r
		node.rule = &cp
	}
	return idx
}
func activeRule(r store.IpNet, now time.Time) bool {
	return r.ExpiresAt == nil || r.ExpiresAt.After(now)
}
func prefixMatch(root *prefixNode, addr netip.Addr, now time.Time) bool {
	bits, _ := addressBits(addr)
	node := root
	for b := 0; node != nil; b++ {
		if node.rule != nil && activeRule(*node.rule, now) {
			return true
		}
		if b >= addr.BitLen() {
			break
		}
		node = node.children[(bits[b/8]>>uint(7-b%8))&1]
	}
	return false
}
func (i *RuleIndex) Allowed(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	now := time.Now()
	if r, ok := i.exact[addr]; ok && r.Action == store.ActionAllow && activeRule(r, now) {
		return true
	}
	_, family := addressBits(addr)
	return prefixMatch(i.allow[family], addr, now)
}
func (i *RuleIndex) Banned(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	now := time.Now()
	_, family := addressBits(addr)
	r, ok := i.exact[addr]
	if ok && r.Action == store.ActionAllow && activeRule(r, now) || prefixMatch(i.allow[family], addr, now) {
		return false
	}
	return ok && r.Action == store.ActionBan && activeRule(r, now) || prefixMatch(i.ban[family], addr, now)
}
func (i *RuleIndex) Exact(ip string) ruleRef {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ruleRef{}
	}
	if r, ok := i.exact[addr.Unmap()]; ok && activeRule(r, time.Now()) {
		return ruleRef{r.ID, r.Action, r.ExpiresAt}
	}
	return ruleRef{}
}
func (s *NetService) rules() (*RuleIndex, error) {
	revision := s.store.IpNetStore.Revision()
	if idx := s.ruleIndex.Load(); idx != nil && idx.revision == revision {
		return idx, nil
	}
	s.ruleMu.Lock()
	defer s.ruleMu.Unlock()
	revision = s.store.IpNetStore.Revision()
	if idx := s.ruleIndex.Load(); idx != nil && idx.revision == revision {
		return idx, nil
	}
	rows, err := s.store.IpNetStore.FindAllActive()
	if err != nil {
		return nil, err
	}
	idx := NewRuleIndex(rows, revision)
	s.ruleIndex.Store(idx)
	return idx, nil
}
