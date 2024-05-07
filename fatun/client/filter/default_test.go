package filter_test

import (
	"os"
	"testing"
	"time"

	"github.com/google/gopacket/pcapgo"
	"github.com/lysShub/fatun/fatun/client/filter"
	"github.com/lysShub/fatun/session"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

func Test_Default_Filter(t *testing.T) {
	fh, err := os.Open("./default_test.data")
	if os.IsNotExist(err) {
		t.Skip("not exist test file")
	}
	require.NoError(t, err)
	defer fh.Close()

	r, err := pcapgo.NewReader(fh)
	require.NoError(t, err)

	var hits = map[uint16]bool{ // local-port:exist
		51076: true,
		51077: true,
		51078: true,
		51079: true,
		51082: true,
		51086: true,
		51087: true,
		51089: true,
		51090: true,
		51091: true,
		51092: true,
		51093: true,
		51096: true,
		51097: true,
	}
	var sess = map[session.Session]int{}

	var limit = 3
	f := filter.NewDefaultFiler(uint8(limit), time.Hour)
	for {
		data, _, err := r.ReadPacketData()
		if err != nil {
			require.Equal(t, "EOF", err.Error())
			return
		}

		ip := data[header.EthernetMinimumSize:]
		if header.IPVersion(ip) == 4 {
			id := session.FromIP(ip)
			sess[id]++

			flags := header.TCP(ip[header.IPv4MinimumSize:]).Flags()
			hit := f.Filter(id.Src, flags)
			if hit {
				has := hits[id.Src.Port()]
				require.True(t, has)
				require.GreaterOrEqual(t, sess[id], limit)
			} else {
				has := hits[id.Src.Port()]
				if has {
					require.LessOrEqual(t, sess[id], limit)
				} else {
					require.False(t, has)
				}
			}
		}
	}
}
