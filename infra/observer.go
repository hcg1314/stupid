package infra

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric/protos/peer"
)

type Observer struct {
	d peer.Deliver_DeliverFilteredClient

	signal chan error
}

func CreateObserver(node Node, channel string, crypto *Crypto) *Observer {
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

	return &Observer{d: deliverer, signal: make(chan error, 10)}
}

func (o *Observer) Start(N uint64, now time.Time) {
	defer close(o.signal)

	var n uint64 = 0
	for n < N {
		r, err := o.d.Recv()
		if err != nil {
			o.signal <- err
		}

		fb := r.Type.(*peer.DeliverResponse_FilteredBlock)
		n = n + uint64(len(fb.FilteredBlock.FilteredTransactions))
		duration := time.Since(now)
		fmt.Printf("Time %v\tBlock %d\tTx %d\tTotal %d\ttps: %f\n",
			duration, fb.FilteredBlock.Number, len(fb.FilteredBlock.FilteredTransactions),
			n, float64(n)/duration.Seconds(),
			)
	}
}

func (o *Observer) Wait() {
	for err := range o.signal {
		if err != nil {
			fmt.Printf("Observed error: %s\n", err)
		}
	}
}
