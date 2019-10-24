package assembler

import (
	"fmt"
	"github.com/hcg1314/stupid/assembler/basic"
	"github.com/hcg1314/stupid/assembler/infra"
	"time"
)

const (
	speedSliceNum = 5
)

type Assembler struct {
	raw         chan *infra.Elements
	config      *basic.Config
	signer      *basic.Crypto
	proposer    *infra.Dispatcher
	broadcaster *infra.Dispatcher
	observer    *infra.Observer

	total      uint64
	speedSlice []uint
}

func CreateAssembler(speed uint, total uint64, path string) *Assembler {
	config := basic.LoadConfig(path)
	crypto := config.LoadCrypto()

	proposer := infra.CreateProposalDispatcher(config.NumOfConn, config.ClientPerConn, config.Peers, crypto)
	broadcaster := infra.CreateBroadcastDispatcher(config.NumOfConn, config.Orderer, crypto)
	observer := infra.CreateObserver(config.Peers[0], config.Channel, crypto) // 先从1个peer观察吧

	//go observer.Start(TotalTransaction)

	assembler := &Assembler{
		raw:         make(chan *Elements, 1000),
		config:      config,
		signer:      crypto,
		proposer:    proposer,
		broadcaster: broadcaster,
		observer:    observer,
		total:       total,
		speedSlice:  make([]uint, speedSliceNum),
	}

	remainder := speed % speedSliceNum
	base := speed / speedSliceNum
	if base != 0 {
		for i := 0; i < speedSliceNum; i++ {
			assembler.speedSlice[i] = base
		}
	}
	if remainder != 0 {
		for i := 0; remainder > 0; i++ {
			assembler.speedSlice[i] += 1
			remainder--
		}
	}

	return assembler
}

func (a *Assembler) assemble(e *Elements) *Elements {
	env, err := infra.CreateSignedTx(e.Proposal, a.signer, e.Response)
	if err != nil {
		panic(err)
	}

	e.Envelope = env
	return e
}

func (a *Assembler) sign(e *Elements) *Elements {
	sprop, err := infra.SignProposal(e.Proposal, a.signer)
	if err != nil {
		panic(err)
	}

	e.SignedProp = sprop
	return e
}

func (a *Assembler) Start() {

	go a.observer.Start(a.total)

	speedCtrl := time.NewTicker(200 * time.Millisecond)
	seq := 0
	speedIndex := 0
	remainder := a.total
	for {
		if speedIndex >= speedSliceNum {
			speedIndex = 0
		}

		if remainder == 0 {
			break
		}

		select {
		case <-speedCtrl.C:
			var i, num uint64 = 0, remainder
			if num > uint64(a.speedSlice[speedIndex]) {
				num = uint64(a.speedSlice[speedIndex])
			}
			remainder -= num

			for ; i < num; i++ {
				prop := infra.CreateProposal(
					a.signer,
					a.config.Channel,
					a.config.Chaincode,
					"addFile",
					fmt.Sprintf("%d", seq),
					fmt.Sprintf("%d", seq),
					"true",
					"-1",
					"-1",
				)
				seq += 1
				a.raw <- &Elements{Proposal: prop}
			}
		}
		speedIndex += 1
	}
}

func (a *Assembler) StartSigner() {
	for {
		select {
		case r, ok := <-a.raw:
			if !ok {
				return
			}
			a.proposer.Send(a.sign(r))
		}
	}
}

func (a *Assembler) StartIntegrator() {
	for {
		select {
		case p, ok := <-a.proposer.GetOutput():
			if !ok {
				return
			}
			a.broadcaster.Send(a.assemble(p))
		}
	}
}

func (a *Assembler) Wait() {
	a.observer.Wait()
}