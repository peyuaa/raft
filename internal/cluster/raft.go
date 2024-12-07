package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

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
	response := NodesResponse{
		Nodes: make([]NodeResponse, 0, len(h.raft.nodes)),
	}

	for _, n := range h.raft.nodes {
		response.Nodes = append(response.Nodes, NodeResponse{
			Id:         n.id.String(),
			Role:       n.role.String(),
			Term:       n.term,
			JournalLen: n.journal.Len(),
			Alive:      !n.turnOffBool,
		})
	}

	res, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(res)
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

	res := JournalResponse{
		Id:  node.id.String(),
		Log: make([]string, 0, node.journal.Len()),
	}

	for entry := range node.journal.Entries() {
		res.Log = append(res.Log, entry.String())
	}

	body, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
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

	res := RequestResponse{
		Id:    req.ID,
		Key:   req.Msg["key"].(string),
		Value: req.Msg["value"].(string),
	}

	body, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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

	res := DumpResponse{
		Id:   node.id.String(),
		Dump: fmt.Sprint(node.journal.Proc().Dump()),
	}

	body, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
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

	key := r.URL.Query().Get("key")

	v, ok := node.journal.Proc().Get(key)
	if !ok {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}

	res := GetResponse{
		Id:    id,
		Key:   key,
		Value: v.(string),
	}

	body, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
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

	res := TopologyResponse{
		Id:    id,
		Nodes: make([]NodesStatus, 0, len(node.hasConnects)),
	}

	for nd, b := range node.hasConnects {
		res.Nodes = append(res.Nodes, NodesStatus{
			Node:      nd.String(),
			Connected: b,
		})
	}

	body, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
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

	res := DisconnectResponse{
		Node:   id,
		With:   idWith,
		Status: b,
	}

	body, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
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

	status := node.Connect(uidWith)

	res := ConnectResponse{
		Node:   id,
		With:   idWith,
		Status: status,
	}

	body, err := json.Marshal(res)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
