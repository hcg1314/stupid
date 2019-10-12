package main

import (
	"fmt"
	"os"
	"time"

	"github.com/hcg1314/stupid/infra"
)

func main() {
/*	if len(os.Args) != 3 {
		fmt.Printf("Usage: stupid config.json 500\n")
		os.Exit(1)
	}*/

	config := infra.LoadConfig(os.Args[1])
	/*N, err := strconv.Atoi(os.Args[2])
	if err != nil {
		panic(err)
	}*/
	N := 100
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

	broadcaster := infra.CreateBroadcasters(config.NumOfConn, config.Orderers, crypto)
	broadcaster.Start(envs, done)

	observer := infra.CreateObserver(config.Peers, config.Channel, crypto)

	start := time.Now()
	go observer.Start(N, start)

	for i := 0; i < N; i++ {

		prop := infra.CreateProposal(
			crypto,
			config.Channel,
			config.Chaincode,
			"addFile",
			fmt.Sprintf("%d", i),
			fmt.Sprintf("%d", i),
			"true",
			"-1",
			"-1",
		)
		raw <- &infra.Elecments{Proposal: prop}
	}

	observer.Wait()
	duration := time.Since(start)
	close(done)

	fmt.Printf("tx: %d, duration: %+v, tps: %f\n", N, duration, float64(N)/duration.Seconds())
	os.Exit(0)
}
