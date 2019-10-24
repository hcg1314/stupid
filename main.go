package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/hcg1314/stupid/infra"
)

const (
	SpeedSliceNum = 5
)

var (
	TotalTransaction uint64
	Speed            uint
	SpeedCtrl        []uint
	ConfigFilePath   string
	Help             bool
)

func init() {
	SpeedCtrl = make([]uint, SpeedSliceNum)
	flag.Uint64Var(&TotalTransaction, "total", math.MaxUint64, "the num of transactions generated")
	flag.UintVar(&Speed, "speed", 0, "the num of transactions generated per second")
	flag.StringVar(&ConfigFilePath, "path", "", "the path of config file")
	flag.BoolVar(&Help, "h", false, "help messages")
}

func initSpeedCtrl(speedCtrl []uint, speed uint) {
	remainder := speed % SpeedSliceNum
	base := speed / SpeedSliceNum
	if base != 0 {
		for i := 0; i < SpeedSliceNum; i++ {
			speedCtrl[i] = base
		}
	}
	if remainder != 0 {
		for i := 0; remainder > 0; i++ {
			speedCtrl[i] += 1
			remainder--
		}
	}
}

func outputStatistic(proposers *infra.Proposers) {
	f, err := os.OpenFile("endorser-static.log", os.O_RDWR|os.O_APPEND, os.ModePerm)
	if err != nil {
		f, err = os.OpenFile("endorser-static.log", os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			f = os.Stdout
		}
	}
	log1 := log.New(f,"", log.LstdFlags)
	stat := time.NewTicker(time.Second)
	for {
		select {
		case <- stat.C:
			log1.Println(proposers.GetStatisticInfo())
		}
	}
}

func generateTransaction(crypto *infra.Crypto, config *infra.Config, raw chan *infra.Elecments) {
	speedCtrl := time.NewTicker(200 * time.Millisecond)
	seq := 0
	speedIndex := 0
	remainder := TotalTransaction
	for {
		if speedIndex >= SpeedSliceNum {
			speedIndex = 0
		}

		if remainder == 0 {
			break
		}

		select {
		case <-speedCtrl.C:
			var i,num uint64 = 0, remainder
			if num > uint64(SpeedCtrl[speedIndex]) {
				num = uint64(SpeedCtrl[speedIndex])
			}
			remainder -= num

			for ; i < num; i++ {
				prop := infra.CreateProposal(
					crypto,
					config.Channel,
					config.Chaincode,
					"addFile",
					fmt.Sprintf("%d", seq),
					fmt.Sprintf("%d", seq),
					"true",
					"-1",
					"-1",
				)
				seq += 1
				raw <- &infra.Elecments{Proposal: prop}
			}
		}
		speedIndex += 1
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

	initSpeedCtrl(SpeedCtrl, Speed)

	config := infra.LoadConfig(ConfigFilePath)
	crypto := config.LoadCrypto()

	raw := make(chan *infra.Elecments, 100)
	signed := make(chan *infra.Elecments, 100)
	processed := make(chan *infra.Elecments, 100)
	envs := make(chan *infra.Elecments, 100)
	done := make(chan struct{})

	assembler := &infra.Assembler{Signer: crypto}
	for i := 0; i < 5; i++ {
		go assembler.StartSigner(raw, signed, done)         // sign proposal
		go assembler.StartIntegrator(processed, envs, done) // create signed tx
	}

	proposers := infra.CreateProposers(config.NumOfConn, config.ClientPerConn, config.Peers, crypto, signed)
	proposers.Start(processed, done)

	infra.CreateBroadcasters(config.NumOfConn, config.Orderer, crypto).Start(envs, done)

	observer := infra.CreateObserver(config.Peers[0], config.Channel, crypto) // 先从1个peer观察吧

	start := time.Now()
	go observer.Start(TotalTransaction, start)

	go generateTransaction(crypto, &config, raw)

	go outputStatistic(proposers)

	go func() {
		f, err := os.OpenFile("static.log", os.O_RDWR|os.O_APPEND, os.ModePerm)
		if err != nil {
			f, err = os.OpenFile("static.log", os.O_RDWR|os.O_CREATE, os.ModePerm)
			if err != nil {
				f = os.Stdout
			}
		}
		log1 := log.New(f,"", log.LstdFlags)
		stat := time.NewTicker(time.Second)
		for {
			select {
			case <- stat.C:
				log1.Printf("raw waited: %10d, processed waited: %10d, envelope waited: %10d\n",
					len(raw), len(processed), len(envs))
			}
		}
	}()

	observer.Wait()
	close(done)

	os.Exit(0)
}
