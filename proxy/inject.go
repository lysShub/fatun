package proxy

import (
	"itun/divert"
	"itun/pack"
	"math/rand"
)

/*
	1. 传输层数据包代理
	2. 需附加ifiID和dstIP
	3. server收到数据包后, 先解包, 通过dstIP构建IP包, 可能需要更改LocalPort(因为可能已被占用), 然后
		发送IP包。如果是第一个ifiID:localPort的数据包, 则在listen中增加读取规则。
	4. 读取时, 通过bpf, 每个ifiID对象所对应的listen只会读取属于自己的数据包; 读取到数据包后, 可能需要
		更改localPort(如果第三步更改了localPort), 最后给数据打包, 最后剔除IP头, 发送至客户端。
*/

func (p *Proxy) inject() {
	// TODO: 不知道具体实现
	var ipId uint16 = uint16(rand.Intn(0xffff))

	var srcIP [4]byte

	var addr = divert.Address{}
	addr.Header.SetOutbound(false)

	var b = make([]byte, 1536)
	var n int
	var err error
	for {
		if n, err = p.proxyConn.Read(b); p.shutdown(err) {
			return
		} else {
			if n > 40+pack.W {
				srcIP, localPort = pack.Parse(b[:n])

				ipHdrLen := int((b[0] >> 4) * 5)

				p.ipHdr.SetTotalLen(uint16(n - ipHdrLen + 20 - pack.W))
				p.ipHdr.SetID(ipId)
				ipId++
				p.ipHdr.SetSrcIP(srcIP)
				p.ipHdr.SetChecksum(0)
				p.ipHdr.SetChecksum(pack.Checksum(*p.ipHdr))

				copy(b[ipHdrLen-20:], (*p.ipHdr)[:])

				if _, err = p.listenHdl.Send(b[ipHdrLen-20:n-pack.W], &addr); p.shutdown(err) {
					return
				}
			}
		}
	}
}
