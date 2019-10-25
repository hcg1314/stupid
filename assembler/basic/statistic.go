package basic

import (
	"fmt"
	"log"
	"os"
	"time"
)

const (
	total = iota
	succ
	fail
)

const (
	ItemProposal  = 0
	ItemBroadcast = 1
	ItemButt      = 2
)

var itemDesc = []string{"proposal", "broadcast"}

type sig struct {
	Item int
	Sig  int
}

var globalStat *statHandler

func init() {
	globalStat = &statHandler{
		signal: make(chan *sig, 1000),
	}

	go globalStat.Start()
}

type statHandler struct {
	signal  chan *sig
	last    [ItemButt]statItem
	current [ItemButt]statItem
}

func AddTotal(item int) {
	if item >= ItemButt {
		return
	}
	globalStat.signal <- &sig{item, total}
}

func AddSuccess(item int) {
	if item >= ItemButt {
		return
	}
	globalStat.signal <- &sig{item, succ}
}

func AddFail(item int) {
	if item >= ItemButt {
		return
	}
	globalStat.signal <- &sig{item, fail}
}

func (sh *statHandler) Start() {
	f, err := os.OpenFile("static.log", os.O_RDWR|os.O_APPEND, os.ModePerm)
	if err != nil {
		f, err = os.OpenFile("static.log", os.O_RDWR|os.O_CREATE, os.ModePerm)
		if err != nil {
			f = os.Stdout
		}
	}
	log1 := log.New(f, "", log.LstdFlags)
	stat := time.NewTicker(time.Second)

	for {
		select {
		case r := <-sh.signal:
			if r.Sig == total {
				sh.current[r.Item].Total += 1
			} else if r.Sig == succ {
				sh.current[r.Item].Success += 1
			} else {
				sh.current[r.Item].Fail += 1
			}
		case <-stat.C:
			info := "Endorser Statistic:\n" +
				"                    Total(     Speed)   Success(     Speed)      Fail(     Speed)\n"
			for i, curr := range sh.current{
				last := sh.last[i]
				info += fmt.Sprintf("%-15s%10d(%10d)%10d(%10d)%10d(%10d)\n",
					itemDesc[i],
					curr.Total, curr.Total-last.Total,
					curr.Success, curr.Success-last.Success,
					curr.Fail, curr.Fail-last.Fail,
				)
			}
			log1.Println(info)
		}
	}
}

type statItem struct {
	Total   uint64
	Success uint64
	Fail    uint64
}

func (s *statItem) Copy(src statItem) {
	s.Total = src.Total
	s.Success = src.Success
	s.Fail = src.Fail
}
