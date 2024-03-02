package client

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/itun"
	"github.com/lysShub/itun/cctx"
	"github.com/shirou/gopsutil/v3/process"
)

type filter struct {
	ctx     cctx.CancelCtx
	proxyCh chan itun.Session

	processName       string
	processNameProtos []itun.Proto
}

func NewFilter(ctx cctx.CancelCtx) *filter {
	f := &filter{
		ctx:     ctx,
		proxyCh: make(chan itun.Session, 8),
	}
	return f
}

func (f *filter) ProxyCh() <-chan itun.Session {
	return f.proxyCh
}

func (f *filter) EnableDefaultRule() error {
	return nil // todo:
}

func (f *filter) DisableDefaultRule() error {
	return nil
}

func (f *filter) AddRule(proto itun.Proto, pname string) error {
	if f.processName == "" {
		go f.proxyByNameService()
	}

	if slices.Contains(f.processNameProtos, proto) {
		return nil
	}
	f.processNameProtos = append(f.processNameProtos, proto)
	return nil
}

func (f *filter) proxyByNameService() {
	var s = "!ipv6 and event=CONNECT"
	d, err := divert.Open(s, divert.SOCKET, filterPriority, divert.READ_ONLY|divert.SNIFF)
	if err != nil {
		f.ctx.Cancel(err)
		return
	}

	var addr divert.Address
	for {
		_, err := d.Recv(nil, &addr)
		if err != nil {
			f.ctx.Cancel(err)
			return
		}

		s := addr.Socket()

		if p, err := process.NewProcess(int32(s.ProcessId)); err == nil {
			if name, err := p.Name(); err == nil {
				fmt.Println(name)
				if f.processName == name {
					session := itun.Session{
						SrcAddr: netip.AddrPortFrom(s.LocalAddr(), s.LocalPort),
						Proto:   itun.Proto(s.Protocol),
						DstAddr: netip.AddrPortFrom(s.RemoteAddr(), s.RemotePort),
					}

					if !slices.Contains(f.processNameProtos, session.Proto) {
						fmt.Println("还不支持的proto ", session.Proto.String())
					} else if !session.IsValid() {
						fmt.Println("不合法的session", itun.ErrInvalidSession(session).Error())
					}

					select {
					case f.proxyCh <- session:
					default:
						fmt.Println("block")
					}
				}
			}
		}
	}
}

func (f *filter) Close() error {
	return nil
}
