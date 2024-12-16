package journal

import (
	"errors"
	"fmt"
	"iter"
	"strconv"

	"github.com/charmbracelet/log"
)

type Message struct {
	Term  int
	Index int
	Data  any
}

func (m Message) String() string {
	return fmt.Sprintf("%d:{TERM:%d, DATA:%s}", m.Index, m.Term, strconv.Quote(fmt.Sprint(m.Data)))
}

type Processor[K comparable, V any] interface {
	Process(any) (any, error)
	Dump() map[K]V
	Get(K) (V, bool)
}

type Journal struct {
	storage     []Message
	commitIndex int
	processor   Processor[any, any]
}

func NewJournal(processor Processor[any, any]) *Journal {
	return &Journal{commitIndex: -1, processor: processor, storage: []Message{}}
}

func (j *Journal) Put(m Message) error {
	if m.Index != j.Len() {
		return errors.New("messages must be added sequentially")
	}

	if j.Len() > 0 && j.storage[j.PrevIndex()].Term > m.Term {
		return errors.New("term of the new message must be greater than or equal to the last term")
	}

	j.storage = append(j.storage, m)
	return nil
}

func (j *Journal) Commit() bool {
	if j.commitIndex+1 >= len(j.storage) {
		return false
	}
	j.commitIndex++

	_, err := j.processor.Process(j.storage[j.commitIndex].Data)
	if err != nil {
		log.Errorf("journal commit err: %v", err)
		return false
	}

	return true
}

func (j *Journal) CommitIndex() int {
	return j.commitIndex
}

func (j *Journal) Len() int {
	return len(j.storage)
}

func (j *Journal) PrevIndex() int {
	return j.Len() - 1
}

func (j *Journal) PrevTerm() int {
	return j.storage[j.PrevIndex()].Term
}

func (j *Journal) Get(i int) Message {
	if i < 0 || i >= j.Len() {
		return Message{}
	}
	return j.storage[i]
}

func (j *Journal) Last() Message {
	return j.storage[len(j.storage)-1]
}

func (j *Journal) Entries() iter.Seq[Message] {
	return func(yield func(Message) bool) {
		for _, entry := range j.storage {
			if !yield(entry) {
				return
			}
		}
	}
}

func (j *Journal) Proc() Processor[any, any] {
	return j.processor
}
