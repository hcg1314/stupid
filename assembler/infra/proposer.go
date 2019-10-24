package infra

import (
	"context"
	"fmt"
	"github.com/hcg1314/stupid/assembler/basic"
	"github.com/hyperledger/fabric/protos/peer"
)

const (
	Total = iota
	Succ
	Fail
)

type Statistic struct {
	Total   uint64
	Success uint64
	Fail    uint64
}

func (s *Statistic) Copy(src Statistic) {
	s.Total = src.Total
	s.Success = src.Success
	s.Fail = src.Fail
}

type proposer struct {
	e         peer.EndorserClient
	node      basic.Node
	clientNum int
	last      Statistic
	current   Statistic
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
		node:      node,
		clientNum: clientNum,
		result:    make(chan int, 1000),
		signed:    make(chan *Elements, 1000),
	}
	return p
}

func (p *proposer) Handle(e *Elements) error {
	p.signed <- e
	return nil
}

func (p *proposer) Start(processed chan *Elements) {

	go p.startCount()

	for seq := 0; seq < p.clientNum; seq++ {
		go p.startProposer(processed)
	}
}

func (p *proposer) startProposer(processed chan *Elements) {
	for {
		select {
		case s := <-p.signed:
			p.result <- Total
			r, err := p.e.ProcessProposal(context.Background(), s.SignedProp)
			// err不为空时，r会为nil，r.Response会导致panic
			if err != nil {
				p.result <- Fail
				fmt.Printf("Err processing proposal, err: %v\n", err)
				continue
			}
			if r == nil {
				p.result <- Fail
				continue
			}
			// 消息投递到peer，背书异常，输出具体原因
			if r.Response.Status < 200 || r.Response.Status >= 400 {
				fmt.Printf("Err processing proposal: %v, status: %d\n", r.Response.Message, r.Response.Status)
				p.result <- Fail
				continue
			}
			p.result <- Succ

			s.Response = r
			processed <- s
		}
	}
}

func (p *proposer) startCount() {
	for {
		select {
		case r := <-p.result:
			if r == Total {
				p.current.Total += 1
			} else if r == Succ {
				p.current.Success += 1
			} else {
				p.current.Fail += 1
			}
		}
	}
}

func (p *proposer) GetStatisticInfo() string {

	info := fmt.Sprintf("\t%10d(%10d)%10d(%10d)%10d(%10d)\t waited:%10d\tnode:%s-%s\n",
		p.current.Total, p.current.Total-p.last.Total,
		p.current.Success, p.current.Success-p.last.Success,
		p.current.Fail, p.current.Fail-p.last.Fail, len(p.signed),
		p.node.Addr, p.node.OverrideName,
	)

	p.last.Copy(p.current)
	return info
}
