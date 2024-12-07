package raft

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"github.com/peyuaa/raft/internal/journal"
	raftmap "github.com/peyuaa/raft/internal/map"
	"github.com/peyuaa/raft/internal/raft/sms"

	"github.com/google/uuid"
)

type (
	ID         fmt.Stringer
	SMS        = sms.Message
	VoteUpdate struct {
		Entry []sms.Entry[any]
		Count int
		Nodes map[ID]bool
		Done  bool
	}
)

func NewVoteUpdate(entry []sms.Entry[any]) VoteUpdate {
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
	id                  ID
	term                int
	role                Role
	nodes               map[ID]*Node
	voted               bool
	currentVotes        int
	votePool            map[ID]bool
	maxDelta            time.Duration
	leaderHeartDeadline time.Time
	messages            chan SMS
	updaters            chan any
	indexPool           map[ID]*time.Ticker
	nodePoolWait        map[ID]chan struct{}
	voteUpdate          VoteUpdate
	waitRequest         chan any
	hasConnects         map[ID]bool

	journal *journal.Journal

	logger *log.Logger

	// debug only
	turnOff     chan struct{}
	turnOffBool bool
}

const _messageBufferSise = 1000
const _factor = 16

func NewNode(nodes iter.Seq[*Node]) *Node {
	n := &Node{
		id:                  uuid.New(),
		journal:             journal.NewJournal(raftmap.New[any, any]()),
		term:                -1,
		role:                Follower,
		nodes:               make(map[ID]*Node),
		votePool:            make(map[ID]bool),
		messages:            make(chan SMS, _messageBufferSise),
		updaters:            make(chan any, _messageBufferSise),
		logger:              log.New(os.Stdout),
		maxDelta:            randDelta(),
		leaderHeartDeadline: time.Now().Add(time.Second + rand.N(5*time.Second)),
		turnOff:             make(chan struct{}, 1),
		nodePoolWait:        make(map[ID]chan struct{}, 1),
		indexPool:           make(map[ID]*time.Ticker),
		voteUpdate:          VoteUpdate{Done: true},
		waitRequest:         make(chan any, _messageBufferSise),
		hasConnects:         map[ID]bool{},
	}
	for node := range nodes {
		n.nodes[node.id] = node
		n.nodes[node.id] = node
		n.hasConnects[node.id] = true
		node.hasConnects[n.id] = true
		n.indexPool[node.id] = time.NewTicker(time.Second / _factor / 2)
		n.votePool[node.id] = false
	}
	return n
}

func (n *Node) ID() ID {
	return n.id
}

func (n *Node) Term() int {
	return n.term
}

func (n *Node) JournalLen() int {
	return n.journal.Len()
}

func (n *Node) Role() Role {
	return n.role
}

func (n *Node) Run(ctx context.Context) error {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Sprintf("id:%v, panic: %v", n.id, r))
		}
	}()
	ticker := time.NewTicker(time.Second / _factor)

loop:
	for {
		n.turnOff <- struct{}{}
		<-n.turnOff

		select {
		case <-ctx.Done():
			break loop
		case msg := <-n.messages:
			now := time.Now()
			n.logger.Infof("%v: got sms with message `%s`", n.ID(), msg)
			if msg.GetTo() != n.ID() {
				break
			}
			if !n.hasConnects[msg.GetFrom()] {
				break
			}
			if msg.GetTerm() < n.term {
				break
			}
			switch v := msg.(type) {
			case sms.RequestVote:
				n.requestVoteHandle(v, now)
			case sms.Vote:
				n.voteHandler(v)
			case sms.AppendEntries:
				n.appendEntriesHandler(v, now)
			case sms.AppendEntriesResponse:
				if n.role != Leader {
					continue
				}
				<-n.indexPool[msg.GetFrom()].C
				n.appendEntriesResponseHandler(v)
			}
		case <-ticker.C:
			if n.role == Leader {
				for _, node := range n.nodes {
					select {
					case v := <-node.waitRequest:
						n.updaters <- v
					default:
					}
				}
			}
			now := time.Now()

			if n.role == Candidate {
				n.retryRequestVotes()
				break
			}

			if n.IsLeaderDead(now) {
				n.SetRole(Candidate)
				go n.Election(now)
				break
			}
		}
	}
	return nil
}

func (n *Node) IsLeaderDead(timeNow time.Time) bool {
	return !n.leaderHeartDeadline.IsZero() && n.leaderHeartDeadline.Before(timeNow)
}

func (n *Node) Send(sms SMS) error {
	n.messages <- sms
	return nil
}

func (n *Node) Election(timeNow time.Time) {
	n.logger.Infof("%v: election", n.ID())
	n.currentVotes = 0
	n.clearVotePool()
	n.updateTerm(n.term+1, timeNow)
	go func() {
		for _, node := range n.nodes {
			_ = node.Send(sms.RequestVote{
				From: n.ID().String(),
				To:   node.ID().String(),
				Term: n.term,
			})
		}
	}()
}

func (n *Node) SetRole(role Role) {
	n.role = role
}

func (n *Node) Add(node *Node) error {
	if _, ok := n.nodes[node.id]; ok {
		return fmt.Errorf("node `%v` already exists", node.id)
	}
	n.nodes[node.id] = node
	n.votePool[node.ID()] = false
	n.indexPool[node.ID()] = time.NewTicker(time.Second / _factor)
	n.hasConnects[node.id] = true
	node.hasConnects[n.id] = true

	return nil
}

func (n *Node) clearVotePool() {
	for id := range n.votePool {
		n.votePool[id] = false
	}
}

func (n *Node) retryRequestVotes() {
	for id := range n.votePool {
		if n.voted {
			continue
		}
		_ = n.nodes[id].Send(sms.RequestVote{
			From: n.ID().String(),
			To:   id.String(),
			Term: n.term,
		})
	}
}

func (n *Node) addDeadline2(timeNow time.Time) {
	delta := n.leaderHeartDeadline.Sub(timeNow)
	if (n.maxDelta-delta)/4 == 0 {
		return
	}
	r := rand.N(2*time.Second) / _factor * 4
	n.leaderHeartDeadline = n.leaderHeartDeadline.Add(r)
}

func randDelta() time.Duration {
	return 1*time.Second + rand.N(7*time.Second)
}

func (n *Node) updateTerm(term int, timeNow time.Time) {
	if n.term > term {
		return
	}
	if n.term == term {
		n.addDeadline2(timeNow)
	}
	n.term = term
	n.voted = false
	n.SetRole(Follower)
	n.maxDelta = randDelta()
	n.leaderHeartDeadline = timeNow.Add(n.maxDelta)
}

func (n *Node) Request(s any) {
	if n.role == Leader {
		n.updaters <- s
		return
	}
	n.waitRequest <- s
}

func (n *Node) Topology() map[ID]bool {
	return n.hasConnects
}

func (n *Node) Disconnect(id ID) bool {
	if _, ok := n.hasConnects[id]; !ok {
		return false
	}
	if _, ok := n.nodes[id].hasConnects[n.id]; !ok {
		return false
	}
	n.hasConnects[id] = false
	n.nodes[id].hasConnects[n.id] = false

	return true
}

func (n *Node) Connect(id ID) bool {
	if _, ok := n.hasConnects[id]; !ok {
		return false
	}
	if _, ok := n.nodes[id].hasConnects[n.id]; !ok {
		return false
	}
	n.hasConnects[id] = true
	n.nodes[id].hasConnects[n.id] = true

	return true
}
