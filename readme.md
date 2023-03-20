代理的是应用层数据包, 是无连接代理, 所以在每个数据包后附加上dstIP.

由于proxy-server的本地端口可能被占用, 所以proxy-server需要一套端口管理的能力。proxy-server通过IPv4+BPF读取对应的IP包。




