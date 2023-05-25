语法：
rule: 
    add/del/list rule 'filter language' as ruleName

proxy:
    




内置计划：代理重发了SYN数据包的TCP链接。
TODO: 这只代理了本机的, 需要增加FORWARD层代理
内置代理的语法是：add rule default [on ifIdx]
通过判断ifIdx确定是哪一层的代理。



