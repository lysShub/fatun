// Code generated by "stringer -output gob_gen.go -trimprefix=CtrType -type=CtrType"; DO NOT EDIT.

package internal

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[start-0]
	_ = x[IPv6-1]
	_ = x[EndConfig-2]
	_ = x[AddTCP-3]
	_ = x[DelTCP-4]
	_ = x[AddUDP-5]
	_ = x[DelUDP-6]
	_ = x[PackLoss-7]
	_ = x[Ping-8]
	_ = x[end-9]
}

const _CtrType_name = "startIPv6EndConfigAddTCPDelTCPAddUDPDelUDPPackLossPingend"

var _CtrType_index = [...]uint8{0, 5, 9, 18, 24, 30, 36, 42, 50, 54, 57}

func (i CtrType) String() string {
	if i >= CtrType(len(_CtrType_index)-1) {
		return "CtrType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _CtrType_name[_CtrType_index[i]:_CtrType_index[i+1]]
}
