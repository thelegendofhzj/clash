package gvisor

import (
	"encoding/binary"
	"net"
	"net/netip"
	"time"

	"github.com/Dreamacro/clash/adapter/inbound"
	"github.com/Dreamacro/clash/common/pool"
	C "github.com/Dreamacro/clash/constant"
	D "github.com/Dreamacro/clash/listener/tun/ipstack/commons"
	"github.com/Dreamacro/clash/listener/tun/ipstack/gvisor/adapter"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/transport/socks5"
)

var _ adapter.Handler = (*GVHandler)(nil)

type GVHandler struct {
	DNSAdds []netip.AddrPort

	TCPIn chan<- C.ConnContext
	UDPIn chan<- *inbound.PacketAdapter
}

func (gh *GVHandler) HandleTCP(tunConn adapter.TCPConn) {
	id := tunConn.ID()

	rAddr := &net.TCPAddr{
		IP:   net.IP(id.LocalAddress),
		Port: int(id.LocalPort),
		Zone: "",
	}

	addrIp, _ := netip.AddrFromSlice(rAddr.IP)
	addrPort := netip.AddrPortFrom(addrIp, id.LocalPort)

	if D.ShouldHijackDns(gh.DNSAdds, addrPort) {
		go func() {
			log.Debugln("[TUN] hijack dns tcp: %s", addrPort.String())

			defer tunConn.Close()

			buf := pool.Get(pool.UDPBufferSize)
			defer pool.Put(buf)

			for {
				tunConn.SetReadDeadline(time.Now().Add(D.DefaultDnsReadTimeout))

				length := uint16(0)
				if err := binary.Read(tunConn, binary.BigEndian, &length); err != nil {
					break
				}

				if int(length) > len(buf) {
					break
				}

				n, err := tunConn.Read(buf[:length])
				if err != nil {
					break
				}

				msg, err := D.RelayDnsPacket(buf[:n])
				if err != nil {
					break
				}

				_, _ = tunConn.Write(msg)
			}
		}()

		return
	}

	gh.TCPIn <- inbound.NewSocket(socks5.ParseAddrToSocksAddr(rAddr), tunConn, C.TUN)
}

func (gh *GVHandler) HandleUDP(tunConn adapter.UDPConn) {
	id := tunConn.ID()

	rAddr := &net.UDPAddr{
		IP:   net.IP(id.LocalAddress),
		Port: int(id.LocalPort),
		Zone: "",
	}

	addrIp, _ := netip.AddrFromSlice(rAddr.IP)
	addrPort := netip.AddrPortFrom(addrIp, id.LocalPort)
	target := socks5.ParseAddrToSocksAddr(rAddr)

	go func() {
		for {
			buf := pool.Get(pool.UDPBufferSize)

			n, addr, err := tunConn.ReadFrom(buf)
			if err != nil {
				pool.Put(buf)
				break
			}

			payload := buf[:n]

			if D.ShouldHijackDns(gh.DNSAdds, addrPort) {
				go func() {
					defer pool.Put(buf)

					msg, err1 := D.RelayDnsPacket(payload)
					if err1 != nil {
						return
					}

					_, _ = tunConn.WriteTo(msg, addr)

					log.Debugln("[TUN] hijack dns udp: %s", rAddr.String())
				}()

				continue
			}

			gvPacket := &packet{
				pc:      tunConn,
				rAddr:   addr,
				payload: payload,
			}

			select {
			case gh.UDPIn <- inbound.NewPacket(target, gvPacket, C.TUN):
			default:
			}
		}
	}()
}
