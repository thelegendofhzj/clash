package common

import (
	"fmt"
	"github.com/Dreamacro/clash/component/geodata"
	"github.com/Dreamacro/clash/component/geodata/router"
	"strings"

	"github.com/Dreamacro/clash/component/mmdb"
	"github.com/Dreamacro/clash/component/resolver"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"
)

type GEOIP struct {
	*Base
	country      string
	adapter      string
	noResolveIP  bool
	geoIPMatcher *router.GeoIPMatcher
}

func (g *GEOIP) RuleType() C.RuleType {
	return C.GEOIP
}

func (g *GEOIP) Match(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if ip == nil {
		return false
	}

	if strings.EqualFold(g.country, "LAN") {
		return ip.IsPrivate() ||
			ip.IsUnspecified() ||
			ip.IsLoopback() ||
			ip.IsMulticast() ||
			ip.IsLinkLocalUnicast() ||
			resolver.IsFakeBroadcastIP(ip)
	}
	if !C.GeodataMode {
		record, _ := mmdb.Instance().Country(ip)
		return strings.EqualFold(record.Country.IsoCode, g.country)
	}
	return g.geoIPMatcher.Match(ip)
}

func (g *GEOIP) Adapter() string {
	return g.adapter
}

func (g *GEOIP) Payload() string {
	return g.country
}

func (g *GEOIP) ShouldResolveIP() bool {
	return !g.noResolveIP
}

func (g *GEOIP) GetCountry() string {
	return g.country
}

func (g *GEOIP) GetIPMatcher() *router.GeoIPMatcher {
	return g.geoIPMatcher
}

func NewGEOIP(country string, adapter string, noResolveIP bool) (*GEOIP, error) {
	if !C.GeodataMode {
		geoip := &GEOIP{
			Base:        &Base{},
			country:     country,
			adapter:     adapter,
			noResolveIP: noResolveIP,
		}
		return geoip, nil
	}

	geoIPMatcher, recordsCount, err := geodata.LoadGeoIPMatcher(country)
	if err != nil {
		return nil, fmt.Errorf("[GeoIP] %s", err.Error())
	}

	log.Infoln("Start initial GeoIP rule %s => %s, records: %d", country, adapter, recordsCount)
	geoip := &GEOIP{
		Base:         &Base{},
		country:      country,
		adapter:      adapter,
		noResolveIP:  noResolveIP,
		geoIPMatcher: geoIPMatcher,
	}
	return geoip, nil
}

var _ C.Rule = (*GEOIP)(nil)
