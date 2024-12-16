package node

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/peyuaa/raft/internal/journal"
	raftmap "github.com/peyuaa/raft/internal/map"
)

type VoteUpdate struct {
	Entry []Entry[any]
	Count int
	Nodes map[ID]bool
	Done  bool
}

type ID fmt.Stringer

func NewVoteUpdate(entry []Entry[any]) VoteUpdate {
	return VoteUpdate{
		Entry: entry,
		Count: 0,
		Nodes: make(map[ID]bool),
	}
}

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=Role
type Role int

const (
	Follower Role = iota
	Candidate
	Leader
)

type Node struct {
	Id                      ID
	Term                    int
	Role                    Role
	Nodes                   map[ID]*Node
	Voted                   bool
	CurrentVotes            int
	VotePool                map[ID]bool
	MaxDelta                time.Duration
	LeaderHeartBeatDeadline time.Time
	Messages                chan Message
	Updaters                chan any
	IndexPool               map[ID]*time.Ticker
	NodePoolWait            map[ID]chan struct{}
	VoteUpdate              VoteUpdate
	WaitRequest             chan any
	HasConnects             map[ID]bool

	Journal *journal.Journal

	Logger *log.Logger

	// debug only
	TurnOff     chan struct{}
	TurnOffBool bool
}

const messageBufferSise = 1000
const factor = 16

func NewNode(nodes iter.Seq[*Node]) *Node {
	n := &Node{
		Id:                      uuid.New(),
		Journal:                 journal.NewJournal(raftmap.New[any, any]()),
		Term:                    -1,
		Role:                    Follower,
		Nodes:                   make(map[ID]*Node),
		VotePool:                make(map[ID]bool),
		Messages:                make(chan Message, messageBufferSise),
		Updaters:                make(chan any, messageBufferSise),
		Logger:                  log.New(os.Stdout),
		MaxDelta:                randDelta(),
		LeaderHeartBeatDeadline: time.Now().Add(time.Second + rand.N(5*time.Second)),
		TurnOff:                 make(chan struct{}, 1),
		NodePoolWait:            make(map[ID]chan struct{}, 1),
		IndexPool:               make(map[ID]*time.Ticker),
		VoteUpdate:              VoteUpdate{Done: true},
		WaitRequest:             make(chan any, messageBufferSise),
		HasConnects:             map[ID]bool{},
	}
	for node := range nodes {
		n.Nodes[node.Id] = node
		n.Nodes[node.Id] = node
		n.HasConnects[node.Id] = true
		node.HasConnects[n.Id] = true
		n.IndexPool[node.Id] = time.NewTicker(time.Second / factor / 2)
		n.VotePool[node.Id] = false
	}
	return n
}

func (n *Node) Run(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Sprintf("Id: %v, panic: %v", n.Id, r))
		}
	}()
	ticker := time.NewTicker(time.Second / factor)

loop:
	for {
		n.TurnOff <- struct{}{}
		<-n.TurnOff

		select {
		case <-ctx.Done():
			break loop
		case msg := <-n.Messages:
			timestamp := time.Now()
			n.Logger.Infof("%v: got message `%s`", n.Id, msg)

			if n.messageInvalid(msg) {
				break
			}

			n.handleMessage(msg, timestamp)
		case <-ticker.C:
			if n.Role == Leader {
				n.processUpdates()
			}
			now := time.Now()

			if n.Role == Candidate {
				n.retryRequestVotes()
				break
			}

			if n.LeaderDead(now) {
				n.SetRole(Candidate)
				go n.Election(now)
				break
			}
		}
	}
	return nil
}

func (n *Node) processUpdates() {
	for _, node := range n.Nodes {
		select {
		case v := <-node.WaitRequest:
			n.Updaters <- v
		default:
		}
	}
}

func (n *Node) messageInvalid(msg Message) bool {
	if msg.GetTo() != n.Id {
		return true
	}
	if !n.HasConnects[msg.GetFrom()] {
		return true
	}
	if msg.GetTerm() < n.Term {
		return true
	}

	return false
}

func (n *Node) handleMessage(msg Message, time time.Time) {
	switch v := msg.(type) {
	case RequestVote:
		n.requestVoteHandle(v, time)
	case Vote:
		n.voteHandler(v)
	case AppendEntries:
		n.appendEntriesHandler(v, time)
	case AppendEntriesResponse:
		if n.Role != Leader {
			return
		}
		<-n.IndexPool[msg.GetFrom()].C
		n.appendEntriesResponseHandler(v)
	}
}

func (n *Node) LeaderDead(timeNow time.Time) bool {
	return !n.LeaderHeartBeatDeadline.IsZero() && n.LeaderHeartBeatDeadline.Before(timeNow)
}

func (n *Node) Send(sms Message) {
	n.Messages <- sms
}

func (n *Node) Election(timeNow time.Time) {
	n.Logger.Infof("%v: election", n.Id)
	n.CurrentVotes = 0
	n.clearVotePool()
	n.updateTerm(n.Term+1, timeNow)
	go func() {
		for _, node := range n.Nodes {
			node.Send(RequestVote{
				From: n.Id.String(),
				To:   node.Id.String(),
				Term: n.Term,
			})
		}
	}()
}

func (n *Node) SetRole(role Role) {
	n.Role = role
}

func (n *Node) Add(node *Node) error {
	if _, ok := n.Nodes[node.Id]; ok {
		return fmt.Errorf("node `%v` already exists", node.Id)
	}
	n.Nodes[node.Id] = node
	n.VotePool[node.Id] = false
	n.IndexPool[node.Id] = time.NewTicker(time.Second / factor)
	n.HasConnects[node.Id] = true
	node.HasConnects[n.Id] = true

	return nil
}

func (n *Node) clearVotePool() {
	for id := range n.VotePool {
		n.VotePool[id] = false
	}
}

func (n *Node) retryRequestVotes() {
	for id := range n.VotePool {
		if n.Voted {
			continue
		}
		n.Nodes[id].Send(RequestVote{
			From: n.Id.String(),
			To:   id.String(),
			Term: n.Term,
		})
	}
}

func (n *Node) addDeadline2(timeNow time.Time) {
	delta := n.LeaderHeartBeatDeadline.Sub(timeNow)
	if (n.MaxDelta-delta)/4 == 0 {
		return
	}
	r := rand.N(2*time.Second) / factor * 4
	n.LeaderHeartBeatDeadline = n.LeaderHeartBeatDeadline.Add(r)
}

func randDelta() time.Duration {
	return 1*time.Second + rand.N(7*time.Second)
}

func (n *Node) updateTerm(term int, timeNow time.Time) {
	if n.Term > term {
		return
	}
	if n.Term == term {
		n.addDeadline2(timeNow)
	}
	n.Term = term
	n.Voted = false
	n.SetRole(Follower)
	n.MaxDelta = randDelta()
	n.LeaderHeartBeatDeadline = timeNow.Add(n.MaxDelta)
}

func (n *Node) Request(s any) {
	if n.Role == Leader {
		n.Updaters <- s
		return
	}
	n.WaitRequest <- s
}

func (n *Node) Disconnect(id ID) bool {
	if _, ok := n.HasConnects[id]; !ok {
		return false
	}
	if _, ok := n.Nodes[id].HasConnects[n.Id]; !ok {
		return false
	}
	n.HasConnects[id] = false
	n.Nodes[id].HasConnects[n.Id] = false

	return true
}

func (n *Node) Connect(id ID) bool {
	if _, ok := n.HasConnects[id]; !ok {
		return false
	}
	if _, ok := n.Nodes[id].HasConnects[n.Id]; !ok {
		return false
	}
	n.HasConnects[id] = true
	n.Nodes[id].HasConnects[n.Id] = true

	return true
}
