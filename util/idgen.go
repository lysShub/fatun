package util

type IdGen struct {
	id    int32
	idles []int32 // desc
}

func NewIdGen() *IdGen {
	return &IdGen{}
}

func (g *IdGen) Get() int32 {
	if len(g.idles) > 0 {
		id := g.idles[len(g.idles)-1]
		g.idles = g.idles[:len(g.idles)-1]
		return id
	} else {
		g.id++
		return g.id
	}
}

func (g *IdGen) Del(id int32) {
	for i := len(g.idles) - 1; i >= 0; i-- {
		if g.idles[i] > id {
			g.idles = append(g.idles, 0)
			copy(g.idles[i+1:], g.idles[i:])
			g.idles[i] = id
			return
		}
	}

	g.idles = append(g.idles, 0)
	copy(g.idles[1:], g.idles[0:])
	g.idles[0] = id
}
