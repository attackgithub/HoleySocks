package main

import (
	"fmt"
	"github.com/audibleblink/HoleySocks/pkg/holeysocks"
)

func main() {
	go holeysocks.DarnSocks()
	fmt.Printf("Serving on remote: %s\n", holeysocks.Config.Socks.Remote)
	holeysocks.ForwardService()
}
