package cluster

import (
	"time"

	"github.com/peyuaa/raft/internal/cluster/message"
	"github.com/peyuaa/raft/internal/journal"
)

type entry = message.Entry[any]

func (n *Node) requestVoteHandle(msg message.RequestVote, timeNow time.Time) {
	to := n.nodes[msg.GetFrom()]

	if msg.GetTerm() <= n.term { // if we don't need to update term
		to.Send(message.Vote{
			From:        n.id.String(),
			To:          to.id.String(),
			Term:        n.term,
			VoteGranted: false,
		})
		return
	}

	granted := true

	if n.voted {
		granted = false
	}
	n.voted = true

	n.updateTerm(msg.GetTerm(), timeNow)

	vote := message.Vote{
		From:        n.id.String(),
		To:          to.id.String(),
		Term:        n.term,
		VoteGranted: granted,
	}

	to.Send(vote)
}

func (n *Node) voteHandler(msg message.Vote) {
	if n.role == Leader {
		return
	}
	if n.votePool[msg.GetFrom()] {
		return
	}
	n.votePool[msg.GetFrom()] = true
	if msg.Term != n.term {
		return
	}

	if msg.VoteGranted {
		n.currentVotes++
	}
	n.logger.Infof("%v: got `%d`", n.id, n.currentVotes)
	if n.currentVotes >= (len(n.votePool)+1)/2 {
		n.logger.Infof("a leader is %v", n.id)
		n.SetRole(Leader)
		n.leaderHeartDeadline = time.Time{}
		for _, node := range n.nodes {
			node.Send(message.AppendEntries{
				From:        n.id.String(),
				To:          node.id.String(),
				Term:        n.term,
				PrevIndex:   n.journal.PrevIndex(),
				PrevTerm:    n.journal.Get(n.journal.PrevIndex()).Term,
				CommitIndex: n.journal.CommitIndex(),
				Entries:     nil,
			})
		}
	}
}

func (n *Node) appendEntriesHandler(msg message.AppendEntries, timeNow time.Time) {
	n.updateTerm(msg.GetTerm(), timeNow)
	n.voted = false

	if n.term < msg.Term {
		n.term = msg.Term
	}
	if msg.CommitIndex > n.journal.CommitIndex() {
		if len(msg.Entries) != 0 {
			_ = n.journal.Put(journal.Message{
				Term:  msg.Term,
				Index: msg.PrevIndex,
				Data:  msg.Entries[0].Data,
			})
		}
		if n.journal.PrevIndex() > n.journal.CommitIndex() {
			if n.journal.Commit() {
				n.nodes[msg.GetFrom()].Send(message.AppendEntriesResponse{
					From:       n.id.String(),
					To:         msg.From,
					Term:       n.term,
					Success:    true,
					MatchIndex: n.journal.CommitIndex(),
				})
				return
			}

		}
	}
	if msg.CommitIndex == n.journal.CommitIndex() && n.journal.Get(n.journal.CommitIndex()).Term == msg.PrevTerm {
		if len(msg.Entries) > 0 {
			_ = n.journal.Put(journal.Message{
				Term:  msg.Term,
				Index: n.journal.Len(),
				Data:  msg.Entries[0].Data,
			})
		}
		n.nodes[msg.GetFrom()].Send(message.AppendEntriesResponse{
			From:       n.id.String(),
			To:         msg.From,
			Term:       n.term,
			Success:    true,
			MatchIndex: n.journal.PrevIndex(),
		})
		return
	}
	n.nodes[msg.GetFrom()].Send(message.AppendEntriesResponse{
		From:       n.id.String(),
		To:         msg.From,
		Term:       n.term,
		Success:    false,
		MatchIndex: msg.PrevIndex,
	})
}

func (n *Node) appendEntriesResponseHandler(msg message.AppendEntriesResponse) {
	if msg.Success {
		if msg.MatchIndex < n.journal.CommitIndex() {
			n.nodes[msg.GetFrom()].Send(message.AppendEntries{
				From:        n.id.String(),
				To:          msg.From,
				Term:        n.term,
				PrevIndex:   msg.MatchIndex + 1,
				PrevTerm:    n.journal.Get(msg.MatchIndex + 1).Term,
				CommitIndex: n.journal.CommitIndex(),
				Entries: []entry{
					{
						Term: n.journal.Get(msg.MatchIndex + 1).Term,
						Data: n.journal.Get(msg.MatchIndex + 1).Data,
					},
				},
			})
			return
		}
		if msg.MatchIndex == n.journal.CommitIndex() {
			var entries []entry
			if n.voteUpdate.Done {
				select {
				case v := <-n.updaters:
					entries = append(entries, entry{
						Term: n.term,
						Data: v,
					})

					err := n.journal.Put(journal.Message{
						Term:  n.term,
						Index: n.journal.Len(),
						Data:  v,
					})
					if err != nil {
						n.logger.Error("unable to put message in the journal: %v", err)
					}

					n.voteUpdate = NewVoteUpdate(entries)
				default:
				}
			} else {
				if !n.voteUpdate.Nodes[msg.GetFrom()] {
					entries = n.voteUpdate.Entry
				}
			}
			n.nodes[msg.GetFrom()].Send(message.AppendEntries{
				From:        n.id.String(),
				To:          msg.From,
				Term:        n.term,
				PrevIndex:   msg.MatchIndex,
				PrevTerm:    n.journal.Get(msg.MatchIndex).Term,
				CommitIndex: n.journal.CommitIndex(),
				Entries:     entries,
			})

			return
		}
		if !n.voteUpdate.Nodes[msg.GetFrom()] {
			n.voteUpdate.Nodes[msg.GetFrom()] = true
			n.voteUpdate.Count++
			if n.voteUpdate.Count >= (len(n.nodes)+1)/2 {
				n.voteUpdate.Done = true
				n.journal.Commit()
			}
		}
		n.nodes[msg.GetFrom()].Send(message.AppendEntries{
			From:        n.id.String(),
			To:          msg.From,
			Term:        n.term,
			PrevIndex:   msg.MatchIndex,
			PrevTerm:    n.journal.Get(msg.MatchIndex).Term,
			CommitIndex: n.journal.CommitIndex(),
			Entries:     nil,
		})
		return
	}
	n.nodes[msg.GetFrom()].Send(message.AppendEntries{
		From:        n.id.String(),
		To:          msg.From,
		Term:        n.term,
		PrevIndex:   msg.MatchIndex - 1,
		PrevTerm:    n.journal.Get(msg.MatchIndex - 1).Term,
		CommitIndex: n.journal.CommitIndex(),
		Entries: []entry{
			{
				Term: n.journal.Get(msg.MatchIndex - 1).Term,
				Data: n.journal.Get(msg.MatchIndex - 1).Data,
			},
		},
	})
}
