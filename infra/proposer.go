package infra

import (
	"context"
	"fmt"
	"io"

	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"github.com/hyperledger/fabric/protos/peer"
)

type Proposers struct {
	workers []*Proposer

	client int
	index  uint64
}

func CreateProposers(conn, client int, node Node, crypto *Crypto) *Proposers {
	ps := make([]*Proposer, conn)
	for i := 0; i < conn; i++ {
		ps[i] = CreateProposer(node, crypto)
	}

	return &Proposers{workers: ps, client: client}
}

func (ps *Proposers) Start(signed, processed chan *Elecments, done <-chan struct{}) {
	fmt.Printf("Start sending transactions...\n\n")
	for _, p := range ps.workers {
		for i := 0; i < ps.client; i++ {
			go p.Start(signed, processed, done)
		}
	}
}

type Proposer struct {
	e peer.EndorserClient
}

func CreateProposer(node Node, crypto *Crypto) *Proposer {
	endorser, err := CreateEndorserClient(node, crypto.TLSCACerts)
	if err != nil {
		panic(err)
	}

	return &Proposer{e: endorser}
}

func (p *Proposer) Start(signed, processed chan *Elecments, done <-chan struct{}) {
	for {
		select {
		case s := <-signed:
			r, err := p.e.ProcessProposal(context.Background(), s.SignedProp)
			// err不为空时，r会为nil，r.Response会导致panic
			if err != nil {
				fmt.Printf("Err processing proposal, err: %v\n", err)
				continue
			}
			if r == nil {
				continue
			}
			// 消息投递到peer，背书异常，输出具体原因
			if r.Response.Status < 200 || r.Response.Status >= 400 {
				fmt.Printf("Err processing proposal: %v, status: %d\n", r.Response.Message, r.Response.Status)
				continue
			}

			s.Response = r
			processed <- s

		case <-done:
			return
		}
	}
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
