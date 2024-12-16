package cluster

import (
	"time"

	"github.com/peyuaa/raft/internal/cluster/message"
	"github.com/peyuaa/raft/internal/journal"
)

type entry = message.Entry[any]

func (n *Node) requestVoteHandle(msg message.RequestVote, timeNow time.Time) {
	to := n.Nodes[msg.GetFrom()]

	if msg.GetTerm() <= n.Term { // if we don't need to update Term
		to.Send(message.Vote{
			From:        n.Id.String(),
			To:          to.Id.String(),
			Term:        n.Term,
			VoteGranted: false,
		})
		return
	}

	granted := true

	if n.Voted {
		granted = false
	}
	n.Voted = true

	n.updateTerm(msg.GetTerm(), timeNow)

	vote := message.Vote{
		From:        n.Id.String(),
		To:          to.Id.String(),
		Term:        n.Term,
		VoteGranted: granted,
	}

	to.Send(vote)
}

func (n *Node) voteHandler(msg message.Vote) {
	if n.Role == Leader {
		return
	}
	if n.VotePool[msg.GetFrom()] {
		return
	}
	n.VotePool[msg.GetFrom()] = true
	if msg.Term != n.Term {
		return
	}

	if msg.VoteGranted {
		n.CurrentVotes++
	}
	n.Logger.Infof("%v: got `%d`", n.Id, n.CurrentVotes)
	if n.CurrentVotes >= (len(n.VotePool)+1)/2 {
		n.Logger.Infof("a leader is %v", n.Id)
		n.SetRole(Leader)
		n.LeaderHeartDeadline = time.Time{}
		for _, node := range n.Nodes {
			node.Send(message.AppendEntries{
				From:        n.Id.String(),
				To:          node.Id.String(),
				Term:        n.Term,
				PrevIndex:   n.Journal.PrevIndex(),
				PrevTerm:    n.Journal.Get(n.Journal.PrevIndex()).Term,
				CommitIndex: n.Journal.CommitIndex(),
				Entries:     nil,
			})
		}
	}
}

func (n *Node) appendEntriesHandler(msg message.AppendEntries, timeNow time.Time) {
	n.updateTerm(msg.GetTerm(), timeNow)
	n.Voted = false

	if n.Term < msg.Term {
		n.Term = msg.Term
	}
	if msg.CommitIndex > n.Journal.CommitIndex() {
		if len(msg.Entries) != 0 {
			_ = n.Journal.Put(journal.Message{
				Term:  msg.Term,
				Index: msg.PrevIndex,
				Data:  msg.Entries[0].Data,
			})
		}
		if n.Journal.PrevIndex() > n.Journal.CommitIndex() {
			if n.Journal.Commit() {
				n.Nodes[msg.GetFrom()].Send(message.AppendEntriesResponse{
					From:       n.Id.String(),
					To:         msg.From,
					Term:       n.Term,
					Success:    true,
					MatchIndex: n.Journal.CommitIndex(),
				})
				return
			}

		}
	}
	if msg.CommitIndex == n.Journal.CommitIndex() && n.Journal.Get(n.Journal.CommitIndex()).Term == msg.PrevTerm {
		if len(msg.Entries) > 0 {
			_ = n.Journal.Put(journal.Message{
				Term:  msg.Term,
				Index: n.Journal.Len(),
				Data:  msg.Entries[0].Data,
			})
		}
		n.Nodes[msg.GetFrom()].Send(message.AppendEntriesResponse{
			From:       n.Id.String(),
			To:         msg.From,
			Term:       n.Term,
			Success:    true,
			MatchIndex: n.Journal.PrevIndex(),
		})
		return
	}
	n.Nodes[msg.GetFrom()].Send(message.AppendEntriesResponse{
		From:       n.Id.String(),
		To:         msg.From,
		Term:       n.Term,
		Success:    false,
		MatchIndex: msg.PrevIndex,
	})
}

func (n *Node) appendEntriesResponseHandler(msg message.AppendEntriesResponse) {
	if msg.Success {
		if msg.MatchIndex < n.Journal.CommitIndex() {
			n.Nodes[msg.GetFrom()].Send(message.AppendEntries{
				From:        n.Id.String(),
				To:          msg.From,
				Term:        n.Term,
				PrevIndex:   msg.MatchIndex + 1,
				PrevTerm:    n.Journal.Get(msg.MatchIndex + 1).Term,
				CommitIndex: n.Journal.CommitIndex(),
				Entries: []entry{
					{
						Term: n.Journal.Get(msg.MatchIndex + 1).Term,
						Data: n.Journal.Get(msg.MatchIndex + 1).Data,
					},
				},
			})
			return
		}
		if msg.MatchIndex == n.Journal.CommitIndex() {
			var entries []entry
			if n.VoteUpdate.Done {
				select {
				case v := <-n.Updaters:
					entries = append(entries, entry{
						Term: n.Term,
						Data: v,
					})

					err := n.Journal.Put(journal.Message{
						Term:  n.Term,
						Index: n.Journal.Len(),
						Data:  v,
					})
					if err != nil {
						n.Logger.Error("unable to put message in the Journal: %v", err)
					}

					n.VoteUpdate = NewVoteUpdate(entries)
				default:
				}
			} else {
				if !n.VoteUpdate.Nodes[msg.GetFrom()] {
					entries = n.VoteUpdate.Entry
				}
			}
			n.Nodes[msg.GetFrom()].Send(message.AppendEntries{
				From:        n.Id.String(),
				To:          msg.From,
				Term:        n.Term,
				PrevIndex:   msg.MatchIndex,
				PrevTerm:    n.Journal.Get(msg.MatchIndex).Term,
				CommitIndex: n.Journal.CommitIndex(),
				Entries:     entries,
			})

			return
		}
		if !n.VoteUpdate.Nodes[msg.GetFrom()] {
			n.VoteUpdate.Nodes[msg.GetFrom()] = true
			n.VoteUpdate.Count++
			if n.VoteUpdate.Count >= (len(n.Nodes)+1)/2 {
				n.VoteUpdate.Done = true
				n.Journal.Commit()
			}
		}
		n.Nodes[msg.GetFrom()].Send(message.AppendEntries{
			From:        n.Id.String(),
			To:          msg.From,
			Term:        n.Term,
			PrevIndex:   msg.MatchIndex,
			PrevTerm:    n.Journal.Get(msg.MatchIndex).Term,
			CommitIndex: n.Journal.CommitIndex(),
			Entries:     nil,
		})
		return
	}
	n.Nodes[msg.GetFrom()].Send(message.AppendEntries{
		From:        n.Id.String(),
		To:          msg.From,
		Term:        n.Term,
		PrevIndex:   msg.MatchIndex - 1,
		PrevTerm:    n.Journal.Get(msg.MatchIndex - 1).Term,
		CommitIndex: n.Journal.CommitIndex(),
		Entries: []entry{
			{
				Term: n.Journal.Get(msg.MatchIndex - 1).Term,
				Data: n.Journal.Get(msg.MatchIndex - 1).Data,
			},
		},
	})
}
