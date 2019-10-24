package main

import (
	"flag"
	"math"
	"os"

	"github.com/hcg1314/stupid/assembler"
)

var (
	TotalTransaction uint64
	Speed            uint
	ConfigFilePath   string
	Help             bool
)

func init() {
	flag.Uint64Var(&TotalTransaction, "total", math.MaxUint64, "the num of transactions generated")
	flag.UintVar(&Speed, "speed", 0, "the num of transactions generated per second")
	flag.StringVar(&ConfigFilePath, "path", "", "the path of config file")
	flag.BoolVar(&Help, "h", false, "help messages")
}

/*func outputStatistic(proposers *assembler.Proposers) {
	f, err := os.OpenFile("endorser-static.log", os.O_RDWR|os.O_APPEND, os.ModePerm)
	if err != nil {
		f, err = os.OpenFile("endorser-static.log", os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			f = os.Stdout
		}
	}
	log1 := log.New(f, "", log.LstdFlags)
	stat := time.NewTicker(time.Second)
	for {
		select {
		case <-stat.C:
			log1.Println(proposers.GetStatisticInfo())
		}
	}
}*/

func main() {
	flag.Parse()
	if Help {
		flag.Usage()
		return
	}
	if TotalTransaction == 0 || Speed == 0 {
		flag.Usage()
		return
	}

	assembler := assembler.CreateAssembler(Speed, TotalTransaction, ConfigFilePath)

	for i := 0; i < 5; i++ {
		go assembler.StartSigner()     // sign proposal
		go assembler.StartIntegrator() // create signed tx
	}

	go assembler.Start()



	os.Exit(0)
}
