// socks Server
// https://zh.m.wikipedia.org/zh-hans/SOCKS
package itun

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

// 返回conn是agent到server之间的连接, 如果返回nil表示拒绝此次代理请求
//  参数other为账号密码, 仅socks5支持
type doFunc func(raddr string) net.Conn

// Socks
//  conn  socksClient与socksServer之间的连接
//  sconn socksServer与destServer之间的连接
func Socks(ctx context.Context, conn net.Conn, do doFunc) (sconn net.Conn, err error) {
	if deadline, ok := ctx.Deadline(); ok {
		return nil, errors.New("timeout")
	} else {
		conn.SetReadDeadline(deadline)
	}
	var done = make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.SetReadDeadline(time.Now())
		case <-done:
		default:
		}
	}()
	defer func() {
		close(done)
		conn.SetReadDeadline(time.Time{})
	}()

	const S = 512
	var r = make([]byte, S)
	var n int
	if n, err = conn.Read(r); err != nil {
		return nil, err
	} else if n >= 8 {
		if r[0] == 0x4 { // socks4
			var raddr net.TCPAddr

			raddr.Port = int(r[2])<<8 + int(r[3])
			raddr.IP = net.IPv4(r[4], r[5], r[6], r[7])

			var v4a = r[4]+r[5]+r[6] == 0 && r[7] != 0

			var rcode byte = 90
			defer func() {
				_, err = conn.Write([]byte{
					0,
					rcode,
					byte(raddr.Port >> 8), byte(raddr.Port),
					raddr.IP[12], raddr.IP[13], raddr.IP[14], raddr.IP[15],
				})
			}()
			if !v4a {
				if sconn = do(raddr.String()); sconn == nil {
					rcode = 91
				}
			} else {
				var domain string

				var s, e int
				for r[s] != 0 && s < S {
					s++
				}
				e = s
				for r[e] != 0 && e < S {
					e++
				}
				domain = string(r[s:e])
				if sconn = do(domain); sconn == nil {
					rcode = 91
				}
			}
		} else if r[0] == 0x5 { // socks5
			if r[1] < 2 {
				return nil, errors.New("xxx")
			}
			var respCode byte = 0x00

			// auth
			methods := r[2:r[1]]
			auth := AcceptAuths.Include(methods...)
			if _, err = conn.Write([]byte{0x05, byte(auth)}); err != nil {
				return nil, err
			}
			if auth == NULL {
				return nil, errors.New("vvvvv")
			} else {
				if err = auth.Auth(conn); err != nil {
					return nil, err
				}
			}

			if n, err = conn.Read(r); err != nil {
				return nil, err
			} else if n < 7 {
				return nil, errors.New("wwwww")
			} else if r[0] != 0x05 {
				return nil, errors.New("yyyyyy")
			}
			r = r[:n:cap(r)]

			var raddr string
			switch r[1] {
			case 0x01:
				switch r[3] {
				case 0x01: // ip4
					if n < 10 {
						return nil, errors.New("zzzzz")
					}
					p := net.IPv4(r[4], r[5], r[6], r[7])
					raddr = p.String() + ":" + strconv.Itoa(int(r[8])<<8+int(r[9]))
				case 0x03: // domain
					var s, e int = int(r[5]), int(r[5]) + int(r[4])
					if s >= e {
						return nil, errors.New("zzzzz")
					} else if e+1 >= n {
						return nil, errors.New("aaaaa")
					}
					raddr = string(r[s:e]) + ":" + strconv.Itoa(int(r[e])<<8+int(r[e+1]))
				case 0x04: // ip6
					if n < 22 {
						return nil, errors.New("bbbbb")
					}
					p := net.IP{r[4], r[5], r[6], r[7], r[8], r[9], r[10], r[11], r[12], r[13], r[14], r[15], r[16], r[17], r[18], r[19]}
					raddr = p.String() + ":" + strconv.Itoa(int(r[20])<<8+int(r[21]))
				default:
					return nil, errors.New("kkkkkk")
				}
			case 0x02:
				respCode = 0x07
			case 0x03:
				respCode = 0x07
			default:
				respCode = 0x07
			}

			defer func() {
				_, err = conn.Write(append(
					[]byte{
						0x05,
						respCode,
						0x00,
					},
					r[4:]..., // 似乎是代理服务器的地址, 及最终请求服务器收到请求时所知道的client的地址
				))
			}()

			if respCode == 0 {
				if sconn = do(raddr); sconn == nil {
					respCode = 0x02
				}
			}
		} else {
			return nil, fmt.Errorf("unknown version: %d", r[0])
		}
	}

	return sconn, nil
}
