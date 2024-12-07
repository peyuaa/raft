package cluster

type NodeResponse struct {
	Id         string `json:"id"`
	Role       string `json:"role"`
	Term       int    `json:"term"`
	JournalLen int    `json:"journal_len"`
	Alive      bool   `json:"alive"`
}

type NodesResponse struct {
	Nodes []NodeResponse `json:"nodes"`
}

type JournalResponse struct {
	Id  string   `json:"id"`
	Log []string `json:"log"`
}
