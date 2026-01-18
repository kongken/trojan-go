package router

import (
	"context"
	"log/slog"
	"net"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	v2router "github.com/v2fly/v2ray-core/v4/app/router"

	"github.com/p4gefau1t/trojan-go/common"
	"github.com/p4gefau1t/trojan-go/common/geodata"
	"github.com/p4gefau1t/trojan-go/config"
	"github.com/p4gefau1t/trojan-go/tunnel"
	"github.com/p4gefau1t/trojan-go/tunnel/freedom"
	"github.com/p4gefau1t/trojan-go/tunnel/transport"
)

const (
	Block  = 0
	Bypass = 1
	Proxy  = 2
)

const (
	AsIs         = 0
	IPIfNonMatch = 1
	IPOnDemand   = 2
)

const MaxPacketSize = 1024 * 8

func matchDomain(list []*v2router.Domain, target string) bool {
	for _, d := range list {
		switch d.GetType() {
		case v2router.Domain_Full:
			domain := d.GetValue()
			if domain == target {
				slog.Debug("domain rule matched", "target", target, "rule", domain, "type", "full")
				return true
			}
		case v2router.Domain_Domain:
			domain := d.GetValue()
			if strings.HasSuffix(target, domain) {
				idx := strings.Index(target, domain)
				if idx == 0 || target[idx-1] == '.' {
					slog.Debug("domain rule matched", "target", target, "rule", domain, "type", "domain")
					return true
				}
			}
		case v2router.Domain_Plain:
			// keyword
			if strings.Contains(target, d.GetValue()) {
				slog.Debug("domain rule matched", "target", target, "rule", d.GetValue(), "type", "keyword")
				return true
			}
		case v2router.Domain_Regex:
			matched, err := regexp.Match(d.GetValue(), []byte(target))
			if err != nil {
				slog.Error("invalid regex", "pattern", d.GetValue())
				return false
			}
			if matched {
				slog.Debug("domain rule matched", "target", target, "rule", d.GetValue(), "type", "regex")
				return true
			}
		default:
			slog.Debug("unknown rule type", "type", d.GetType().String())
		}
	}
	return false
}

func matchIP(list []*v2router.CIDR, target net.IP) bool {
	isIPv6 := true
	len := net.IPv6len
	if target.To4() != nil {
		len = net.IPv4len
		isIPv6 = false
	}
	for _, c := range list {
		n := int(c.GetPrefix())
		mask := net.CIDRMask(n, 8*len)
		cidrIP := net.IP(c.GetIp())
		if cidrIP.To4() != nil { // IPv4 CIDR
			if isIPv6 {
				continue
			}
		} else { // IPv6 CIDR
			if !isIPv6 {
				continue
			}
		}
		subnet := &net.IPNet{IP: cidrIP.Mask(mask), Mask: mask}
		if subnet.Contains(target) {
			return true
		}
	}
	return false
}

func newIPAddress(address *tunnel.Address) (*tunnel.Address, error) {
	ip, err := address.ResolveIP()
	if err != nil {
		return nil, common.NewError("router failed to resolve ip").Base(err)
	}
	newAddress := &tunnel.Address{
		IP:   ip,
		Port: address.Port,
	}
	if ip.To4() != nil {
		newAddress.AddressType = tunnel.IPv4
	} else {
		newAddress.AddressType = tunnel.IPv6
	}
	return newAddress, nil
}

type Client struct {
	domains        [3][]*v2router.Domain
	cidrs          [3][]*v2router.CIDR
	defaultPolicy  int
	domainStrategy int
	underlay       tunnel.Client
	direct         *freedom.Client
	ctx            context.Context
	cancel         context.CancelFunc
}

func (c *Client) Route(address *tunnel.Address) int {
	if address.AddressType == tunnel.DomainName {
		if c.domainStrategy == IPOnDemand {
			resolvedIP, err := newIPAddress(address)
			if err == nil {
				for i := Block; i <= Proxy; i++ {
					if matchIP(c.cidrs[i], resolvedIP.IP) {
						return i
					}
				}
			}
		}
		for i := Block; i <= Proxy; i++ {
			if matchDomain(c.domains[i], address.DomainName) {
				return i
			}
		}
		if c.domainStrategy == IPIfNonMatch {
			resolvedIP, err := newIPAddress(address)
			if err == nil {
				for i := Block; i <= Proxy; i++ {
					if matchIP(c.cidrs[i], resolvedIP.IP) {
						return i
					}
				}
			}
		}
	} else {
		for i := Block; i <= Proxy; i++ {
			if matchIP(c.cidrs[i], address.IP) {
				return i
			}
		}
	}
	return c.defaultPolicy
}

func (c *Client) DialConn(address *tunnel.Address, overlay tunnel.Tunnel) (tunnel.Conn, error) {
	policy := c.Route(address)
	switch policy {
	case Proxy:
		return c.underlay.DialConn(address, overlay)
	case Block:
		return nil, common.NewError("router blocked address: " + address.String())
	case Bypass:
		conn, err := c.direct.DialConn(address, &Tunnel{})
		if err != nil {
			return nil, common.NewError("router dial error").Base(err)
		}
		return &transport.Conn{
			Conn: conn,
		}, nil
	}
	panic("unknown policy")
}

func (c *Client) DialPacket(overlay tunnel.Tunnel) (tunnel.PacketConn, error) {
	directConn, err := net.ListenPacket("udp", "")
	if err != nil {
		return nil, common.NewError("router failed to dial udp (direct)").Base(err)
	}
	proxy, err := c.underlay.DialPacket(overlay)
	if err != nil {
		return nil, common.NewError("router failed to dial udp (proxy)").Base(err)
	}
	ctx, cancel := context.WithCancel(c.ctx)
	conn := &PacketConn{
		Client:     c,
		PacketConn: directConn,
		proxy:      proxy,
		cancel:     cancel,
		ctx:        ctx,
		packetChan: make(chan *packetInfo, 16),
	}
	go conn.packetLoop()
	return conn, nil
}

func (c *Client) Close() error {
	c.cancel()
	return c.underlay.Close()
}

type codeInfo struct {
	code     string
	strategy int
}

func loadCode(cfg *Config, prefix string) []codeInfo {
	codes := []codeInfo{}
	for _, s := range cfg.Router.Proxy {
		if strings.HasPrefix(s, prefix) {
			if left := s[len(prefix):]; len(left) > 0 {
				codes = append(codes, codeInfo{
					code:     left,
					strategy: Proxy,
				})
			} else {
				slog.Warn("invalid empty rule", "rule", s)
			}
		}
	}
	for _, s := range cfg.Router.Bypass {
		if strings.HasPrefix(s, prefix) {
			if left := s[len(prefix):]; len(left) > 0 {
				codes = append(codes, codeInfo{
					code:     left,
					strategy: Bypass,
				})
			} else {
				slog.Warn("invalid empty rule", "rule", s)
			}
		}
	}
	for _, s := range cfg.Router.Block {
		if strings.HasPrefix(s, prefix) {
			if left := s[len(prefix):]; len(left) > 0 {
				codes = append(codes, codeInfo{
					code:     left,
					strategy: Block,
				})
			} else {
				slog.Warn("invalid empty rule", "rule", s)
			}
		}
	}
	return codes
}

func NewClient(ctx context.Context, underlay tunnel.Client) (*Client, error) {
	m1 := runtime.MemStats{}
	m2 := runtime.MemStats{}
	m3 := runtime.MemStats{}
	m4 := runtime.MemStats{}

	cfg := config.FromContext(ctx, Name).(*Config)
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	direct, err := freedom.NewClient(ctx, nil)
	if err != nil {
		cancel()
		return nil, common.NewError("router failed to initialize raw client").Base(err)
	}

	client := &Client{
		domains:  [3][]*v2router.Domain{},
		cidrs:    [3][]*v2router.CIDR{},
		underlay: underlay,
		direct:   direct,
		ctx:      ctx,
		cancel:   cancel,
	}
	switch strings.ToLower(cfg.Router.DomainStrategy) {
	case "as_is", "as-is", "asis":
		client.domainStrategy = AsIs
	case "ip_if_non_match", "ip-if-non-match", "ipifnonmatch":
		client.domainStrategy = IPIfNonMatch
	case "ip_on_demand", "ip-on-demand", "ipondemand":
		client.domainStrategy = IPOnDemand
	default:
		return nil, common.NewError("unknown strategy: " + cfg.Router.DomainStrategy)
	}

	switch strings.ToLower(cfg.Router.DefaultPolicy) {
	case "proxy":
		client.defaultPolicy = Proxy
	case "bypass":
		client.defaultPolicy = Bypass
	case "block":
		client.defaultPolicy = Block
	default:
		return nil, common.NewError("unknown strategy: " + cfg.Router.DomainStrategy)
	}

	runtime.ReadMemStats(&m1)

	geodataLoader := geodata.NewGeodataLoader()

	ipCode := loadCode(cfg, "geoip:")
	for _, c := range ipCode {
		code := c.code
		cidrs, err := geodataLoader.LoadIP(cfg.Router.GeoIPFilename, code)
		if err != nil {
			slog.Error("failed to load geoip", "code", code, "error", err)
		} else {
			slog.Info("geoip loaded", "code", code)
			client.cidrs[c.strategy] = append(client.cidrs[c.strategy], cidrs...)
		}
	}

	runtime.ReadMemStats(&m2)

	siteCode := loadCode(cfg, "geosite:")
	for _, c := range siteCode {
		code := c.code
		attrWanted := ""
		// Test if user wants domains that have an attribute
		if attrIdx := strings.Index(code, "@"); attrIdx > 0 {
			if !strings.HasSuffix(code, "@") {
				code = c.code[:attrIdx]
				attrWanted = c.code[attrIdx+1:]
			} else { // "geosite:google@" is invalid
				slog.Warn("geosite invalid", "code", code)
				continue
			}
		} else if attrIdx == 0 { // "geosite:@cn" is invalid
			slog.Warn("geosite invalid", "code", code)
			continue
		}

		domainList, err := geodataLoader.LoadSite(cfg.Router.GeoSiteFilename, code)
		if err != nil {
			slog.Error("failed to load geosite", "code", code, "error", err)
		} else {
			found := false
			if attrWanted != "" {
				for _, domain := range domainList {
					for _, attr := range domain.GetAttribute() {
						if strings.EqualFold(attrWanted, attr.GetKey()) {
							client.domains[c.strategy] = append(client.domains[c.strategy], domain)
							found = true
						}
					}
				}
			} else {
				client.domains[c.strategy] = append(client.domains[c.strategy], domainList...)
				found = true
			}
			if found {
				slog.Info("geosite loaded", "code", c.code)
			} else {
				slog.Error("geosite not found", "code", c.code)
			}
		}
	}

	runtime.ReadMemStats(&m3)

	domainInfo := loadCode(cfg, "domain:")
	for _, info := range domainInfo {
		client.domains[info.strategy] = append(client.domains[info.strategy], &v2router.Domain{
			Type:      v2router.Domain_Domain,
			Value:     strings.ToLower(info.code),
			Attribute: nil,
		})
	}

	keywordInfo := loadCode(cfg, "keyword:")
	for _, info := range keywordInfo {
		client.domains[info.strategy] = append(client.domains[info.strategy], &v2router.Domain{
			Type:      v2router.Domain_Plain,
			Value:     strings.ToLower(info.code),
			Attribute: nil,
		})
	}

	regexInfo := loadCode(cfg, "regex:")
	for _, info := range regexInfo {
		if _, err := regexp.Compile(info.code); err != nil {
			return nil, common.NewError("invalid regular expression: " + info.code).Base(err)
		}
		client.domains[info.strategy] = append(client.domains[info.strategy], &v2router.Domain{
			Type:      v2router.Domain_Regex,
			Value:     info.code,
			Attribute: nil,
		})
	}

	// Just for compatibility with V2Ray rule type `regexp`
	regexpInfo := loadCode(cfg, "regexp:")
	for _, info := range regexpInfo {
		if _, err := regexp.Compile(info.code); err != nil {
			return nil, common.NewError("invalid regular expression: " + info.code).Base(err)
		}
		client.domains[info.strategy] = append(client.domains[info.strategy], &v2router.Domain{
			Type:      v2router.Domain_Regex,
			Value:     info.code,
			Attribute: nil,
		})
	}

	fullInfo := loadCode(cfg, "full:")
	for _, info := range fullInfo {
		client.domains[info.strategy] = append(client.domains[info.strategy], &v2router.Domain{
			Type:      v2router.Domain_Full,
			Value:     strings.ToLower(info.code),
			Attribute: nil,
		})
	}

	cidrInfo := loadCode(cfg, "cidr:")
	for _, info := range cidrInfo {
		tmp := strings.Split(info.code, "/")
		if len(tmp) != 2 {
			return nil, common.NewError("invalid cidr: " + info.code)
		}
		ip := net.ParseIP(tmp[0])
		if ip == nil {
			return nil, common.NewError("invalid cidr ip: " + info.code)
		}
		prefix, err := strconv.ParseInt(tmp[1], 10, 32)
		if err != nil {
			return nil, common.NewError("invalid prefix").Base(err)
		}
		client.cidrs[info.strategy] = append(client.cidrs[info.strategy], &v2router.CIDR{
			Ip:     ip,
			Prefix: uint32(prefix),
		})
	}

	slog.Info("router client created")

	runtime.ReadMemStats(&m4)

	slog.Debug("geoip rules memory",
		"alloc", common.HumanFriendlyTraffic(m2.Alloc-m1.Alloc),
		"total_alloc", common.HumanFriendlyTraffic(m2.TotalAlloc-m1.TotalAlloc),
	)
	slog.Debug("geosite rules memory",
		"alloc", common.HumanFriendlyTraffic(m3.Alloc-m2.Alloc),
		"total_alloc", common.HumanFriendlyTraffic(m3.TotalAlloc-m2.TotalAlloc),
	)
	slog.Debug("plaintext rules memory",
		"alloc", common.HumanFriendlyTraffic(m4.Alloc-m3.Alloc),
		"total_alloc", common.HumanFriendlyTraffic(m4.TotalAlloc-m3.TotalAlloc),
	)
	slog.Debug("router memory total",
		"alloc", common.HumanFriendlyTraffic(m4.Alloc-m1.Alloc),
		"total_alloc", common.HumanFriendlyTraffic(m4.TotalAlloc-m1.TotalAlloc),
	)

	return client, nil
}
