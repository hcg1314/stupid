package infra

import (
	"fmt"
	"github.com/hcg1314/stupid/assembler/basic"
	"time"

	"github.com/hyperledger/fabric/protos/peer"
)

type Observer struct {
	d peer.Deliver_DeliverFilteredClient

	got    uint64
	signal chan error
}

func CreateObserver(node basic.Node, channel string, crypto *basic.Crypto) *Observer {
	deliverer, err := CreateDeliverFilteredClient(node, crypto.TLSCACerts)
	if err != nil {
		panic(err)
	}

	seek, err := CreateSignedDeliverNewestEnv(channel, crypto)
	if err != nil {
		panic(err)
	}

	if err = deliverer.Send(seek); err != nil {
		panic(err)
	}

	// drain first response
	if _, err = deliverer.Recv(); err != nil {
		panic(err)
	}

	return &Observer{
		d:      deliverer,
		got:    0,
		signal: make(chan error, 10),
	}
}

func (o *Observer) Start() {
	defer close(o.signal)

	now := time.Now()

	for  {
		r, err := o.d.Recv()
		if err != nil {
			o.signal <- err
		}

		fb := r.Type.(*peer.DeliverResponse_FilteredBlock)
		o.got += uint64(len(fb.FilteredBlock.FilteredTransactions))
		duration := time.Since(now)
		fmt.Printf("Time %v\tBlock %d\tTx %d\tTotal %d\ttps: %f\n",
			duration, fb.FilteredBlock.Number, len(fb.FilteredBlock.FilteredTransactions),
			o.got, float64(o.got)/duration.Seconds(),
		)
	}
}

func (o *Observer) GetTxNumOfGotFromPeer() uint64 {
	return o.got
}
