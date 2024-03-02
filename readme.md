
tansaport proxy


TODO: 
    1. use github.com/pkg/errors
    2. multiplex gvisor stack



先是正常的tcp 完成prev和swap key

var rawl

stack(rawl) ==> ep1=>tcp

key = Swap(tcp)

delete ep1

secl = wrapCrypto(rawl, key) // 会将原始IP数据包加密， 同时更新sum和seq/ack
fakel = wrapFake(secl) // 会附加假的tcp头





然后转为fake tcp， 且fake tcp 是安全的

session      
segement     附加 session id
packet          增加 fake-tcp
raw          对 tcp packet 加密        