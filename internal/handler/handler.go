package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/peyuaa/raft/internal/cluster"
)

type Handler struct {
	raft *cluster.Cluster
}

func New(raft *cluster.Cluster) *Handler {
	return &Handler{raft}
}

func (h *Handler) Nodes(w http.ResponseWriter, _ *http.Request) {
	response := NodesResponse{
		Nodes: make([]NodeResponse, 0, len(h.raft.Nodes)),
	}

	for _, n := range h.raft.Nodes {
		response.Nodes = append(response.Nodes, NodeResponse{
			Id:         n.Id.String(),
			Role:       n.Role.String(),
			Term:       n.Term,
			JournalLen: n.Journal.Len(),
			Alive:      !n.TurnOffBool,
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

	node := h.raft.Node(cluster.ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	res := JournalResponse{
		Id:  node.Id.String(),
		Log: make([]string, 0, node.Journal.Len()),
	}

	for entry := range node.Journal.Entries() {
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

	node := h.raft.Node(cluster.ID(uid))
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

	node := h.raft.Node(cluster.ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	node.TurnOff <- struct{}{}
	node.TurnOffBool = true
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

	node := h.raft.Node(cluster.ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	<-node.TurnOff
	node.TurnOffBool = false
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

	node := h.raft.Node(cluster.ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	res := DumpResponse{
		Id:   node.Id.String(),
		Dump: fmt.Sprint(node.Journal.Proc().Dump()),
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

	node := h.raft.Node(cluster.ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	key := r.URL.Query().Get("key")

	v, ok := node.Journal.Proc().Get(key)
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

	node := h.raft.Node(cluster.ID(uid))
	if node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	res := TopologyResponse{
		Id:    id,
		Nodes: make([]NodesStatus, 0, len(node.HasConnects)),
	}

	for nd, b := range node.HasConnects {
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

	node := h.raft.Node(cluster.ID(uid))
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

	node := h.raft.Node(cluster.ID(uid))
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
