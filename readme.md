


```
    0. 只一个tcp连接，通过tcp header中的保留字段分为MgrSeg和PxySeg，MgrSeg
        由Client请求、Server回复, 必须通过tls发送。

    1. 先握手
        a. 首先建立tls连接， tls连接作为MgrConn
        b. 然后完成初始化配置，比如交换SecretKey, 协商MSS等
        c. Client 发送StartCode开始数据代理。



    3. PxySeg 在服务器上加上IP头后就直接发送

```