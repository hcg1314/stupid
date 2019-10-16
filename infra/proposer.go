package infra

import (
	"context"
	"fmt"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/peer"
	"io"
)

type Proposers struct {
	workers []*Proposer
}

func CreateProposers(conn, client int, nodes []Node, crypto *Crypto, signed []chan *Elecments) *Proposers {
	ps := make([]*Proposer, conn*len(nodes))
	index := 0
	for _, node := range nodes {
		for i := 0; i < conn; i++ {
			ps[index] = CreateProposer(node, crypto, client, signed[index])
			index += 1
		}
	}

	return &Proposers{workers: ps}
}

func (ps *Proposers) Start(processed chan *Elecments, done <-chan struct{}) {
	fmt.Printf("Start sending proposal ...\n\n")
	for _, p := range ps.workers {
		p.Start(processed, done)
	}
}

func (ps *Proposers) GetStatisticInfo() string {
	info := "Endorser Statistic:\n" +
		"\t     Total(     Speed)   Success(     Speed)      Fail(     Speed)\n"
	for _, p := range ps.workers {
		info += p.GetStatisticInfo()
	}
	return info
}

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

type Proposer struct {
	e         peer.EndorserClient
	node      Node
	clientNum int
	last Statistic
	current Statistic
	signed chan *Elecments
	result chan int
}

func CreateProposer(node Node, crypto *Crypto, clientNum int, signed chan *Elecments) *Proposer {
	endorser, err := CreateEndorserClient(node, crypto.TLSCACerts)
	if err != nil {
		panic(err)
	}

	p := &Proposer{
		e: endorser,
		node: node,
		clientNum: clientNum,
		result: make(chan int),
		signed: signed,
	}
	return p
}

func (p *Proposer) Start(processed chan *Elecments, done <-chan struct{}) {

	go p.startCount(done)

	for seq := 0; seq < p.clientNum; seq++ {
		go p.startProposer(processed, done)
	}
}

func (p *Proposer) startProposer(processed chan *Elecments, done <-chan struct{}) {
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

		case <-done:
			return
		}
	}
}

func (p *Proposer) startCount(done <- chan struct{}) {
	for {
		select {
		case r := <- p.result:
			if r == Total {
				p.current.Total += 1
			}else if r == Succ{
				p.current.Success += 1
			}else {
				p.current.Fail += 1
			}
		case <-done:
			return
		}
	}
}

func (p *Proposer) GetStatisticInfo() string {

	info := fmt.Sprintf("\t%10d(%10d)%10d(%10d)%10d(%10d)\t waited:%10d\tnode:%s-%s\n",
		p.current.Total, p.current.Total-p.last.Total,
		p.current.Success, p.current.Success-p.last.Success,
		p.current.Fail, p.current.Fail-p.last.Fail, len(p.signed),
		p.node.Addr, p.node.OverrideName,
		)

	p.last.Copy(p.current)
	return info
}

type Broadcasters []*Broadcaster

func CreateBroadcasters(conn int, node Node, crypto *Crypto) Broadcasters {
	bs := make(Broadcasters, conn)
	for i := 0; i < conn; i++ {
		bs[i] = CreateBroadcaster(node, crypto)
	}

	return bs
}

func (bs Broadcasters) Start(envs <-chan *Elecments, done <-chan struct{}) {
	for _, b := range bs {
		go b.StartDraining()
		go b.Start(envs, done)
	}
}

type Broadcaster struct {
	c orderer.AtomicBroadcast_BroadcastClient
}

func CreateBroadcaster(node Node, crypto *Crypto) *Broadcaster {
	client, err := CreateBroadcastClient(node, crypto.TLSCACerts)
	if err != nil {
		panic(err)
	}

	return &Broadcaster{c: client}
}

func (b *Broadcaster) Start(envs <-chan *Elecments, done <-chan struct{}) {
	for {
		select {
		case e := <-envs:
			err := b.c.Send(e.Envelope)
			if err != nil {
				fmt.Printf("Failed to broadcast env: %s\n", err)
			}

		case <-done:
			return
		}
	}
}

func (b *Broadcaster) StartDraining() {
	for {
		res, err := b.c.Recv()
		if err != nil {
			if err == io.EOF {
				return
			}

			fmt.Printf("Recv broadcast err: %s, status: %+v\n", err, res)
			panic("bcast recv err")
		}

		if res.Status != common.Status_SUCCESS {
			fmt.Printf("Recv errouneous status: %s\n", res.Status)
			panic("bcast recv err")
		}

	}
}
