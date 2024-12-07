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

type RequestResponse struct {
	Id    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type DumpResponse struct {
	Id   string `json:"id"`
	Dump string `json:"dump"`
}

type GetResponse struct {
	Id    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ConnectResponse struct {
	Node   string `json:"node"`
	With   string `json:"with"`
	Status bool   `json:"status"`
}

type DisconnectResponse struct {
	Node   string `json:"node"`
	With   string `json:"with"`
	Status bool   `json:"status"`
}
