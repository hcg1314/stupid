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

	total      uint64
	real       uint64
	speedSlice []uint
	stopped    bool
	done       chan struct{}
}

func CreateAssembler(speed uint, total uint64, path string) *Assembler {
	config := basic.LoadConfig(path)
	crypto := config.LoadCrypto()

	proposer := infra.CreateProposalDispatcher(config.NumOfConn, config.ClientPerConn, config.Peers, crypto)
	go proposer.Start()
	broadcaster := infra.CreateBroadcastDispatcher(config.NumOfConn, config.Orderer, crypto)
	go broadcaster.Start()
	infra.CreateObserver(config.Peers[0], config.Channel, crypto) // 先从1个peer观察吧

	assembler := &Assembler{
		raw:         make(chan *infra.Elements, 1000),
		config:      config,
		signer:      crypto,
		proposer:    proposer,
		broadcaster: broadcaster,
		total:       total,
		real:        0,
		stopped:     false,
		speedSlice:  make([]uint, speedSliceNum),
		done:        make(chan struct{}),
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

func (a *Assembler) assemble(e *infra.Elements) *infra.Elements {
	env, err := infra.CreateSignedTx(e.Proposal, a.signer, e.Response)
	if err != nil {
		panic(err)
	}

	e.Envelope = env
	return e
}

func (a *Assembler) sign(e *infra.Elements) *infra.Elements {
	sprop, err := infra.SignProposal(e.Proposal, a.signer)
	if err != nil {
		panic(err)
	}

	e.SignedProp = sprop
	return e
}

func (a *Assembler) Start() {

	speedCtrl := time.NewTicker(200 * time.Millisecond)
	speedIndex := 0
	for {
		if speedIndex >= speedSliceNum {
			speedIndex = 0
		}

		if a.real == a.total || a.stopped {
			close(a.done)
			break
		}

		select {
		case <-speedCtrl.C:
			var i, num uint64 = 0, a.total-a.real
			if num > uint64(a.speedSlice[speedIndex]) {
				num = uint64(a.speedSlice[speedIndex])
			}

			for ; i < num; i++ {
				prop := infra.CreateProposal(
					a.signer,
					a.config.Channel,
					a.config.Chaincode,
					"addFile",
					fmt.Sprintf("%d", a.real),
					fmt.Sprintf("%d", a.real),
				)
				a.real += 1
				a.raw <- &infra.Elements{Proposal: prop}
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

func (a *Assembler) Stop() {
	a.stopped = true
}

func (a *Assembler) Wait() {
	<-a.done

	fmt.Println("waiting for all tx committed to ledger...")

	t := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-t.C:
			if a.real == infra.GlobalObserver.GetTxNumOfObserved() {
				return
			}
		}
	}
}

func (a *Assembler) GetInfo() string {
	return fmt.Sprintf("raw(%10d),signed(%10d),endorsered(%10d)", len(a.raw), a.proposer.GetWaitCount(), a.broadcaster.GetWaitCount())
}
