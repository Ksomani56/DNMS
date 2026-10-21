// TODO: better name? network topology?
package graph

type NetworkLink struct {
	SrcName string `json:"src"`
	srcNode *NetworkNode
	DstName string `json:"dst"`
	dstNode *NetworkNode
	State   string `json:"state,omitempty"` // "UP", "SEVERED", "DEGRADED"

	refCount int
}
