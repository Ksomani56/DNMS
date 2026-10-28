// TODO: better name? network topology?
package graph

type NetworkLink struct {
	SrcName string `json:"src"`
	srcNode *NetworkNode
	DstName string `json:"dst"`
	dstNode *NetworkNode
	SrcPort string `json:"src_port,omitempty"`
	DstPort string `json:"dst_port,omitempty"`
	Speed   string `json:"speed,omitempty"`
	State   string `json:"state,omitempty"` // "UP", "DOWN", "SEVERED", "DEGRADED", "IMPAIRED"

	refCount int
}
