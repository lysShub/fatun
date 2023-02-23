package itun

/*
  网络代理

  代理流程：
  当收到一个socks请求时,
  首先检查请求地址是否命中手动规则, 如果命中则直接使用代理。
  否之检查请求地址是否存在历史记录, 如果存在则使用代理; 如果此历史记录超过D, 则需要重新验证
  否则尝试直连, 如果直连失败则使用代理并写入历史记录。
*/

import (
	"net"
	"path/filepath"
)

type Itun struct {
	ListenAddr net.Addr
	listener   net.Listener

	proxy func(laddr net.Addr, raddr string) net.Conn

	db *db
}

const defaultListenAddr = "127.0.0.1:19986" // tcp
var dbPath, _ = filepath.Abs("./config.db")

func (i *Itun) init() error {
	var err error
	if i.ListenAddr == nil {
		i.ListenAddr, _ = net.ResolveTCPAddr("tcp", defaultListenAddr)
	}

	switch i.ListenAddr.Network() {
	case "tcp":
		i.listener, err = net.ListenTCP("tcp", i.ListenAddr.(*net.TCPAddr))
		if err != nil {
			return err
		}
	case "udp":

	}

	i.db, err = openDb(dbPath)
	if err != nil {
		return err
	}

	return nil
}

// LoadConfig 从指定路径加载配置, 会和当前配置合并
func (i *Itun) LoadConfig(db string) error {

	return nil
}

// ExportConfig 导出配置到指定路径
//  parts可以指定导出部分, 如果为空则导出全部
func (i *Itun) ExportConfig(path string, parts ...string) error {

	return nil
}

func (i *Itun) GetServer(raddr string) (serveId string) {

	return ""
}
