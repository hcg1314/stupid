package main

import (
	"flag"
	"fmt"
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
	signed := make(chan *infra.Elecments, 10)
	processed := make(chan *infra.Elecments, 10)
	envs := make(chan *infra.Elecments, 10)
	done := make(chan struct{})

	assember := &infra.Assembler{Signer: crypto}
	for i := 0; i < 5; i++ {
		go assember.StartSigner(raw, signed, done)
		go assember.StartIntegrator(processed, envs, done)
	}

	proposor := infra.CreateProposers(config.NumOfConn, config.ClientPerConn, config.Peers, crypto)
	proposor.Start(signed, processed, done)

	broadcaster := infra.CreateBroadcasters(config.NumOfConn, config.Orderer, crypto)
	broadcaster.Start(envs, done)

	observer := infra.CreateObserver(config.Peers[0], config.Channel, crypto) // 先从1个peer观察吧

	start := time.Now()
	go observer.Start(TotalTransaction, start)

	go func() {
		speedCtrl := time.NewTicker(200 * time.Millisecond)
		seq := 0
		speedIndex := 0
		remainder := TotalTransaction
		for {
			if speedIndex >= 5 {
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
	}()

	observer.Wait()
	duration := time.Since(start)
	close(done)

	fmt.Printf("tx: %d, duration: %+v, tps: %f\n", TotalTransaction, duration, float64(TotalTransaction)/duration.Seconds())
	os.Exit(0)
}
