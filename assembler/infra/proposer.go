package infra

import (
	"context"
	"fmt"
	"github.com/hcg1314/stupid/assembler/basic"
	"github.com/hyperledger/fabric/protos/peer"
)

type proposer struct {
	e         peer.EndorserClient
	clientNum int
	signed    chan *Elements
	result    chan int
}

func CreateProposer(node basic.Node, crypto *basic.Crypto, clientNum int) *proposer {
	endorser, err := CreateEndorserClient(node, crypto.TLSCACerts)
	if err != nil {
		panic(err)
	}

	p := &proposer{
		e:         endorser,
		clientNum: clientNum,
		signed:    make(chan *Elements, 1000),
	}
	return p
}

func (p *proposer) Handle(e *Elements) error {
	p.signed <- e
	return nil
}

func (p *proposer) GetWait() int {
	return len(p.signed)
}

func (p *proposer) Start(processed chan *Elements) {
	for seq := 0; seq < p.clientNum; seq++ {
		go p.startProposer(processed)
	}
}

func (p *proposer) startProposer(processed chan *Elements) {
	for {
		select {
		case s := <-p.signed:
			basic.AddTotal(basic.ItemProposal)
			r, err := p.e.ProcessProposal(context.Background(), s.SignedProp)
			// err不为空时，r会为nil，r.Response会导致panic
			if err != nil {
				basic.AddFail(basic.ItemProposal)
				fmt.Printf("Err processing proposal, err: %v\n", err)
				continue
			}
			if r == nil {
				basic.AddFail(basic.ItemProposal)
				continue
			}
			// 消息投递到peer，背书异常，输出具体原因
			if r.Response.Status < 200 || r.Response.Status >= 400 {
				fmt.Printf("Err processing proposal: %v, status: %d\n", r.Response.Message, r.Response.Status)
				basic.AddFail(basic.ItemProposal)
				continue
			}
			basic.AddSuccess(basic.ItemProposal)

			s.Response = r
			processed <- s
		}
	}
}