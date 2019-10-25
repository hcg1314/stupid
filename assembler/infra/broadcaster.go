package infra

import (
	"fmt"
	"github.com/hcg1314/stupid/assembler/basic"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/orderer"
	"io"
)

type broadcaster struct {
	c    orderer.AtomicBroadcast_BroadcastClient
	envs chan *Elements
}

func CreateBroadcaster(node basic.Node, crypto *basic.Crypto) *broadcaster {
	client, err := CreateBroadcastClient(node, crypto.TLSCACerts)
	if err != nil {
		panic(err)
	}

	return &broadcaster{
		c:    client,
		envs: make(chan *Elements, 1000),
	}
}

func (b *broadcaster) Handle(e *Elements) error {
	b.envs <- e
	return nil
}

func (b *broadcaster) GetWait() int {
	return len(b.envs)
}

func (b *broadcaster) Start() {
	go b.startDraining()
	for {
		select {
		case e,ok := <-b.envs:
			if !ok {
				return
			}
			basic.AddTotal(basic.ItemBroadcast)
			err := b.c.Send(e.Envelope)
			if err != nil {
				basic.AddFail(basic.ItemBroadcast)
				fmt.Printf("Failed to broadcast env: %s\n", err)
			}
		}
	}
}

func (b *broadcaster) startDraining() {
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
			basic.AddFail(basic.ItemBroadcast)
			fmt.Printf("Recv errouneous status: %s\n", res.Status)
			continue
		}
		basic.AddSuccess(basic.ItemBroadcast)
	}
}
