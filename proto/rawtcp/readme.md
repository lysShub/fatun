/*
	planA: pxyseg 直接将本地捕获到的数据包修改address后发送, ctrseg 通过特殊标识区分单独进
			入user-tcp-stack。对udp编一个tcp头。
			优点：不会有嵌套头的浪费
			确定：对外暴露的tcp行为怪异，seq/ack 是乱的，容易被针对。




	planB: 嵌套，tcp的seq/ack可以是正常的，tcp的payload是完整的udp/tcp数据包。
			优点：不太好被区分
			缺点：浪费严重，不容易0拷贝。

	当前计划：基本实现planA， 然后转向planB
*/
