package main

import (
	"flag"
	"fmt"
	"github.com/hcg1314/stupid/assembler/basic"
	"log"
	"math"
	"os"
	"time"

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

func outputInfo(as *assembler.Assembler) {
	f, err := os.OpenFile("static.log", os.O_RDWR|os.O_APPEND, os.ModePerm)
	if err != nil {
		f, err = os.OpenFile("static.log", os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			f = os.Stdout
		}
	}
	log1 := log.New(f, "", log.LstdFlags)
	stat := time.NewTicker(time.Second)

	for {
		select {
		case <-stat.C:
			info := basic.GetInfo() + fmt.Sprintf("Assembler: %s\n",as.GetInfo())
			log1.Println(info)
		}
	}
}

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

	go outputInfo(assembler)

	assembler.Wait()

	os.Exit(0)
}
