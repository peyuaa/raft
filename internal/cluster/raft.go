package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type Cluster struct {
	nodes []*Node
}

func New(n int) (*Cluster, error) {
	nodes := make([]*Node, n)
	for i := range n {
		nodes[i] = NewNode(slices.Values(nodes[:i]))
		for _, nd := range nodes[:i] {
			if err := nd.Add(nodes[i]); err != nil {
				return nil, err
			}
		}
	}
	return &Cluster{nodes}, nil
}

func (c *Cluster) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, n := range c.nodes {
		g.Go(func() error {
			return n.Run(ctx)
		})
	}
	return g.Wait()
}

func (c *Cluster) Node(id ID) *Node {
	for _, n := range c.nodes {
		if n.id == id {
			return n
		}
	}
	return nil
}

type Handler struct {
	raft *Cluster
}

func NewHandler(raft *Cluster) *Handler {
	return &Handler{raft}
}

func (h *Handler) Nodes(w http.ResponseWriter, _ *http.Request) {
	buf := &bytes.Buffer{}
	for i, n := range h.raft.nodes {
		buf.WriteString(strconv.Itoa(i+1) + ": " + n.id.String() + " | Role:" + n.role.String() + " | Term:" + strconv.Itoa(n.term) + " | JournalLen: " + strconv.Itoa(n.journal.Len()) + " | alive=" + fmt.Sprint(!n.turnOffBool) + "\n")
	}

	_, err := io.Copy(w, buf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Journal(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	buf := &bytes.Buffer{}
	buf.WriteString("Journal of " + node.id.String() + ":\n")
	for entry := range node.journal.Entries() {
		buf.WriteString(entry.String() + "\n")
	}

	_, err = io.Copy(w, buf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type request struct {
	Msg map[string]any `json:"msg"`
	ID  string         `json:"id"`
}

func (h *Handler) Request(w http.ResponseWriter, r *http.Request) {
	var req request
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(req.ID)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	node.Request(req.Msg)

	_, _ = w.Write([]byte(fmt.Sprint(req)))
}

func (h *Handler) Kill(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	node.turnOff <- struct{}{}
	node.turnOffBool = true
}

func (h *Handler) Recover(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	<-node.turnOff
	node.turnOffBool = false
}

func (h *Handler) DumpMap(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	buf := &bytes.Buffer{}
	buf.WriteString("Map of " + node.id.String() + ":\n")
	buf.WriteString(fmt.Sprint(node.journal.Proc().Dump()))

	_, err = io.Copy(w, buf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	v, ok := node.journal.Proc().Get(r.URL.Query().Get("key"))
	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	_, err = io.Copy(w, strings.NewReader(fmt.Sprint(v)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Topology(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	buf := &bytes.Buffer{}
	buf.WriteString("TOPOLOGY FOR " + node.id.String() + "\n")
	for nd, b := range node.hasConnects {
		buf.WriteString(fmt.Sprintf("%s --> %t\n", nd, b))
	}

	_, err = io.Copy(w, buf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Disconnect(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	idWith := r.URL.Query().Get("with")
	if idWith == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uidWith, err := uuid.Parse(idWith)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	b := node.Disconnect(uidWith)
	_, err = w.Write([]byte(fmt.Sprint(b)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("node")
	if id == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	node := h.raft.Node(ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	idWith := r.URL.Query().Get("with")
	if idWith == "" {
		http.Error(w, "node id is required", http.StatusBadRequest)
		return
	}

	uidWith, err := uuid.Parse(idWith)
	if err != nil {
		http.Error(w, "invalid node id", http.StatusBadRequest)
		return
	}

	b := node.Connect(uidWith)
	_, err = w.Write([]byte(fmt.Sprint(b)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
