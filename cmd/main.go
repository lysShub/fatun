package main

import (
	"github.com/shirou/gopsutil/process"
)

func main() {

}

func getProcessName(pid int) string {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		panic(err)
	}

	n, err := p.Name()
	if err != nil {
		panic(err)
	}
	return n
}
