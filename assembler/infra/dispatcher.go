package infra

import (
	"github.com/hcg1314/stupid/assembler/basic"
	"github.com/hyperledger/fabric/protos/common"
	"github.com/hyperledger/fabric/protos/peer"
)

type Elements struct {
	Proposal   *peer.Proposal
	SignedProp *peer.SignedProposal
	Response   *peer.ProposalResponse
	Envelope   *common.Envelope
}

type Handler interface {
	Handle(e *Elements) error
}

type Dispatcher struct {
	input        chan *Elements
	output       chan *Elements
	handlers     []Handler
	handlerCount int
}

func CreateProposalDispatcher(conn, client int, nodes []basic.Node, crypto *basic.Crypto) *Dispatcher {

	count := conn * len(nodes) // peer节点数*每个节点的tcp连接数
	dispatch := &Dispatcher{
		input:        make(chan *Elements, 1000),
		output:       make(chan *Elements, 1000),
		handlerCount: count,
		handlers:     make([]Handler, count),
	}

	index := 0
	for _, node := range nodes {
		for i := 0; i < conn; i++ {
			proposer := CreateProposer(node, crypto, client)
			proposer.Start(dispatch.output)
			dispatch.handlers[index] = proposer
			index += 1
		}
	}

	return dispatch
}

func CreateBroadcastDispatcher(conn int, node basic.Node, crypto *basic.Crypto) *Dispatcher {
	dispatch := &Dispatcher{
		input:        make(chan *Elements, 1000),
		output:       nil,
		handlerCount: conn,
		handlers:     make([]Handler, conn),
	}

	for i := 0; i < conn; i++ {
		broadcaster := CreateBroadcaster(node, crypto)
		go broadcaster.Start()
		dispatch.handlers[i] = broadcaster
	}

	return dispatch
}

func (d *Dispatcher) Start() {
	index := 0
	for {
		select {
		case msg, ok := <-d.input:
			if !ok {
				return
			}
			if index >= d.handlerCount {
				index = 0
			}

			_ = d.handlers[index].Handle(msg)
			index += 1
		}
	}
}

func (d *Dispatcher) Send(e *Elements) {
	d.input <- e
}

func (d *Dispatcher) GetWaitCount() int {
	return len(d.input)
}

func (d *Dispatcher) GetOutput() chan *Elements {
	return d.output
}
