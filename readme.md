
tansaport proxy


TODO: 
    1. use github.com/pkg/errors
    2. multiplex gvisor stack: 
        a. PrevPackets-SwapKey阶段需要，CtrSession需要
        b. 需要吧gonet拷贝过来, 然后增加AcceptBy
        c. 需要连接管理





    4. crypto.TCP 不支持并发
    5. Byte tree for PrevPackets 





cliet:
```go
go func(){ // uplink
    ip := stack.Outbound()
    if inited {
        b = setSID(ip.tcp, 0xffff)
        b = fakeTCP(b)
        b = encrypt(b)
        raw.Write(b)
    }else{
        raw.Write(ip)
    }
}
go func(){ // downlink
    ip := raw.Read()
    if isfake(ip) {
        b := decrypt(ip.tcp)
        id = getSID(b)
        if id == 0xffff {
            stack.Inbound(b)
        }else{
            addr = sessionMgr.Get(id)
            inject(addr, b)
        }
    }else{
        stack.Inbound(ip)
    }
}

tcp := Connect(stack, raddr)
PrevPacket(tcp)
key := SwapKey(tcp)

seq,ack := stack.link.SeqAck()
fakeTCP = NewFakeTCP(seq,ack)
crypt = NewCrypt(key)
inited = true

ctr := NewCtr(tcp)

ctr.Xxx()
ctr.EndConfig()
```

server:
```go
// in proxyer

go func(){ // downlink
    ip := stack.OutboundBy(addr)
    if inited {
        b := setSID(ip.tcp, 0xffff)
        b = fakeTCP(b)
        b = encrypt(b)
        raw.Write(b)
    }else{
        raw.Write(ip)
    }
}
go func(){ // uplink
    ip := raw.Read()
    if isfake(ip) {
        b := decrypt(ip.tcp)
        id := getSID(b)
        if id==0xffff {
            stack.Inbound(b)
        }else{
            s := sessionMgr.Get(id)
            s.Send(b)
        }
    }else{
        seq,ack := get(ip)
        stack.Inbound(ip)
    }
}


tcp := stack.AccetpBy(addr)
PrevPacket(tcp)
key := SwapKey(tcp)

fakeTCP = NewFakeTCP(seq,ack)
crypt = NewCrypt(key)
inited = true

ctr =  NewCtr(tcp)
ctr.Xxxx()
```
