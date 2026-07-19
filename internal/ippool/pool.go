package ippool

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"
)

type Pool struct {
	mu        sync.Mutex
	addresses []net.IP
	prefixes  []*net.IPNet
	mode      string
	index     uint64
}

func New(addresses, cidrs []string, mode string) (*Pool, error) {
	p := &Pool{mode: mode}
	for _, raw := range addresses {
		ip := net.ParseIP(raw)
		if ip == nil || ip.To4() != nil {
			return nil, fmt.Errorf("source IP must be IPv6: %q", raw)
		}
		p.addresses = append(p.addresses, ip.To16())
	}
	for _, raw := range cidrs {
		ip, network, err := net.ParseCIDR(raw)
		if err != nil || ip.To4() != nil {
			return nil, fmt.Errorf("source CIDR must be IPv6: %q", raw)
		}
		network.IP = network.IP.To16()
		p.prefixes = append(p.prefixes, network)
	}
	if len(p.addresses) == 0 && len(p.prefixes) == 0 {
		return nil, errors.New("source pool is empty")
	}
	return p, nil
}

func (p *Pool) Size() int { return len(p.addresses) + len(p.prefixes) }

func (p *Pool) Next() net.IP {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mode == "round_robin" {
		if len(p.addresses) > 0 {
			ip := append(net.IP(nil), p.addresses[p.index%uint64(len(p.addresses))]...)
			p.index++
			return ip
		}
		idx := p.index % uint64(len(p.prefixes))
		p.index++
		return randomFromPrefix(p.prefixes[idx])
	}

	total := len(p.addresses) + len(p.prefixes)
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(total)))
	idx := int(n.Int64())
	if idx < len(p.addresses) {
		return append(net.IP(nil), p.addresses[idx]...)
	}
	return randomFromPrefix(p.prefixes[idx-len(p.addresses)])
}

func randomFromPrefix(prefix *net.IPNet) net.IP {
	ip := append(net.IP(nil), prefix.IP.To16()...)
	bits := 128
	if ones, _ := prefix.Mask.Size(); ones >= 0 {
		bits -= ones
	}
	if bits == 0 {
		return ip
	}
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return ip
	}
	ones, _ := prefix.Mask.Size()
	for bit := ones; bit < 128; bit++ {
		byteIndex, bitIndex := bit/8, 7-bit%8
		if randomBytes[byteIndex]&(1<<bitIndex) != 0 {
			ip[byteIndex] |= 1 << bitIndex
		} else {
			ip[byteIndex] &^= 1 << bitIndex
		}
	}
	return ip
}
