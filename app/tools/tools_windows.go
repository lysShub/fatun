//go:build windows
// +build windows

package tools

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	nurl "net/url"
	"os"
	"strconv"
	"time"

	"github.com/lysShub/divert-go"
	"github.com/lysShub/fatun/app/client/session"
	"github.com/lysShub/fatun/sconn"
	"golang.org/x/sync/errgroup"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func CaptureTLSWithGolang(partenCtx context.Context, url string, totalSzie, mssDelta int) (sconn.PrevPackets, error) {
	u, err := nurl.Parse(url)
	if err != nil {
		return nil, err
	} else if u.Scheme != "https" {
		return nil, errors.New("require https url")
	}

	var addr = &net.TCPAddr{}
	if ips, err := net.LookupIP(u.Hostname()); err != nil {
		return nil, err
	} else {
		addr.IP = ips[0]
	}
	if p := u.Port(); p != "" {
		if addr.Port, err = strconv.Atoi(p); err != nil {
			return nil, err
		}
	} else {
		addr.Port = 443
	}

	ctx, cancel := context.WithCancel(partenCtx)
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)

	var pps sconn.PrevPackets
	eg.Go(func() error { // capturer
		defer cancel()

		divert.Load(divert.DLL)
		defer divert.Release()
		var f = fmt.Sprintf("tcp and remoteAddr=%s and remotePort=%d", addr.IP.String(), addr.Port)
		d, err := divert.Open(f, divert.Network, 0, 0)
		if err != nil {
			return err
		}
		defer d.Close()

		var size int
		var addr divert.Address
		for size <= totalSzie {
			var b = make([]byte, 1536)

			n, err := d.RecvCtx(ctx, b, &addr)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil // url source size < sizeLimit
				}
				return err
			} else if n == 0 {
				continue
			}

			size += n
			var tcp header.TCP
			switch header.IPVersion(b) {
			case 4:
				tcp = header.IPv4(b[:n]).Payload()
			case 6:
				tcp = header.IPv6(b[:n]).Payload()
			default:
			}
			if err := session.UpdateMSS(tcp, mssDelta); err != nil {
				return err
			}

			if _, err := d.Send(b[:n], &addr); err != nil {
				return err
			}
			seg := tcp.Payload()
			if len(seg) > 0 {
				size += len(seg)
				pps = append(pps, seg)
			}
		}
		return nil
	})
	eg.Go(func() error { // request
		defer cancel()
		time.Sleep(time.Second) // wait capture started

		conn, err := net.DialTCP("tcp", nil, addr)
		if err != nil {
			return err
		}
		defer conn.Close()
		stop := context.AfterFunc(ctx, func() {
			conn.SetDeadline(time.Now()) // conn with ctx
		})
		defer stop()

		var c = http.Client{
			Transport: &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					return conn, nil
				},
			},
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		resp, err := c.Do(req)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return nil // captured expected sizeLimit segments
			}
			return err
		}
		defer resp.Body.Close()

		return nil
	})

	if err = eg.Wait(); err != nil {
		return nil, err
	}
	return pps, partenCtx.Err()
}

func TLSCapWithBrowser(partenCtx context.Context, url string, sizeLimit int) (sconn.PrevPackets, error) {
	return nil, errors.New("todo")
}
