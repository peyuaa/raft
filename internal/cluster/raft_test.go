package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/peyuaa/raft/internal/node"
)

func TestRaft(t *testing.T) {
	raft, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	done := make(chan struct{}, 1)
	go func() {
		defer func() { done <- struct{}{} }()
		_ = raft.Run(ctx)
	}()
	time.Sleep(3 * time.Second)

	var leader *node.Node
	require.Eventually(t, func() bool {
		leader = findLeader(raft)
		return leader != nil
	}, 5*time.Second, 100*time.Millisecond)

	firstLeader := leader
	t.Log(firstLeader.Id)

	// turn off first leader
	firstLeader.TurnOff <- struct{}{}
	time.Sleep(15 * time.Second)
	require.Eventually(t, func() bool {
		leader = findLeader(raft)
		return leader != nil
	}, 5*time.Second, 100*time.Millisecond)

	secondLeader := leader
	t.Log(secondLeader.Id)

	require.NotEqual(t, firstLeader.Id, secondLeader.Id)
	require.NotEqual(t, firstLeader.Term, secondLeader.Term)

	// turn on first leader
	<-firstLeader.TurnOff

	require.Eventually(t, func() bool {
		return firstLeader.Term == secondLeader.Term
	}, 2*time.Second, 100*time.Millisecond)

	cancel()
	<-done
}

func TestLog(t *testing.T) {
	raft, err := New(5)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	done := make(chan struct{}, 1)
	go func() {
		defer func() { done <- struct{}{} }()
		_ = raft.Run(ctx)
	}()
	time.Sleep(3 * time.Second)

	var leader *node.Node
	require.Eventually(t, func() bool {
		leader = findLeader(raft)
		return leader != nil
	}, 5*time.Second, 100*time.Millisecond)
	t.Log("put aboba")
	leader.Request("aboba")
	first := leader
	time.Sleep(3 * time.Second)
	leader.TurnOff <- struct{}{}
	time.Sleep(7 * time.Second)
	require.Eventually(t, func() bool {
		leader = findLeader(raft)
		return leader != nil
	}, 5*time.Second, 100*time.Millisecond)
	leader.Request("aboba2")
	time.Sleep(3 * time.Second)
	<-first.TurnOff
	time.Sleep(3 * time.Second)
	for _, raftNode := range raft.Nodes {
		t.Log(raftNode.Journal.Get(1))
		t.Log(raftNode.Journal.Len())
	}
	cancel()
	<-done
}

func findLeader(raft *Cluster) (n *node.Node) {
	maxTerm := -2
	for _, raftNode := range raft.Nodes {
		if raftNode.Role == node.Leader && raftNode.Term > maxTerm {
			n = raftNode
			maxTerm = raftNode.Term
		}
	}
	return
}
