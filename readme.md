
tansaport proxy


TODO: 
    1. use github.com/pkg/errors
    2. multiplex gvisor stack: 
        a. PrevPackets-SwapKey阶段需要，CtrSession需要
        b. 需要吧gonet拷贝过来, 然后增加AcceptBy
        c. 需要连接管理
    4. crypto.TCP 不支持并发
    5. Byte tree for PrevPackets 


    