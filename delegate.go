package main

import (
	"sync/atomic"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/jacksontj/dnms/aggregator"
	"github.com/jacksontj/dnms/mapper"
	"github.com/sirupsen/logrus"
)

type DNMSDelegate struct {
	// for us to map things
	Mapper *mapper.Mapper

	// for aggregation
	AggMap *aggregator.AggGraphMap

	AlertMgr *AlertManager

	localName string
	localAddr string

	OnMessageReceived func(msg *ClusterMessage)

	// memberlist.Create can fire NotifyMsg/NotifyJoin during construction —
	// before the caller has a chance to wire up the *Memberlist value — so we
	// store it atomically and let those callbacks observe nil safely until
	// SetMlist is called.
	mlist atomic.Pointer[memberlist.Memberlist]
}

func NewDNMSDelegate(m *mapper.Mapper, a *aggregator.AggGraphMap, localName, localAddr string) *DNMSDelegate {
	return &DNMSDelegate{
		Mapper:    m,
		AggMap:    a,
		localName: localName,
		localAddr: localAddr,
	}
}

func (d *DNMSDelegate) SetMlist(m *memberlist.Memberlist) {
	d.mlist.Store(m)
}

func (d *DNMSDelegate) Mlist() *memberlist.Memberlist {
	return d.mlist.Load()
}

// NodeMeta is used to retrieve meta-data about the current node
// when broadcasting an alive message. It's length is limited to
// the given byte size. This metadata is available in the Node structure.
func (d *DNMSDelegate) NodeMeta(limit int) []byte {
	return nil
}

// NotifyMsg is called when a user-data message is received via memberlist.
func (d *DNMSDelegate) NotifyMsg(buf []byte) {
	if len(buf) == 0 {
		return
	}
	if buf[0] == clusterMsgPrefix {
		msg, err := decodeClusterMessage(buf)
		if err != nil {
			logrus.Warnf("failed to decode cluster message: %v", err)
			return
		}
		logrus.Infof("[CLUSTER] Received message from %s -> %s: %q", msg.From, msg.To, msg.Content)
		if d.OnMessageReceived != nil {
			d.OnMessageReceived(msg)
		}
		return
	}
	logrus.Debugf("ignoring memberlist user msg type=%d (no handler)", buf[0])
}

// GetBroadcasts is called when user data messages can be broadcast.
// It can return a list of buffers to send. Each buffer should assume an
// overhead as provided with a limit on the total byte size allowed.
// The total byte size of the resulting data to send must not exceed
// the limit. Care should be taken that this method does not block,
// since doing so would block the entire UDP packet receive loop.
func (d *DNMSDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState is used for a TCP Push/Pull. This is sent to
// the remote side in addition to the membership information. Any
// data can be sent here. See MergeRemoteState as well. The `join`
// boolean indicates this is for a join instead of a push/pull.
func (d *DNMSDelegate) LocalState(join bool) []byte {
	return nil
}

// MergeRemoteState is invoked after a TCP Push/Pull. This is the
// state received from the remote side and is the result of the
// remote side's LocalState call. The 'join'
// boolean indicates this is for a join instead of a push/pull.
func (d *DNMSDelegate) MergeRemoteState(buf []byte, join bool) {

}

// Event delegate methods
// NotifyJoin is invoked when a node is detected to have joined.
// The Node argument must not be modified.
func (d *DNMSDelegate) NotifyJoin(n *memberlist.Node) {
	if n.Name == d.localName || (d.localAddr != "" && n.Addr.String() == d.localAddr) {
		return
	}
	logrus.Infof("Node joined %s (%s)", n.Name, n.Addr.String())
	go d.Mapper.AddPeer(mapper.Peer{
		Name: n.Addr.String(),
		Port: int(n.Port),
	})

	// if we are an aggregator
	if d.AggMap != nil {
		d.AggMap.AddPeer(n.Addr.String())
	}

	if d.AlertMgr != nil {
		d.AlertMgr.RecordAlert(&Alert{
			Severity:  SeverityInfo,
			Type:      "peer_join",
			Title:     "Cluster Peer Joined",
			Message:   "Node " + n.Name + " (" + n.Addr.String() + ") joined cluster mesh",
			Entity:    "peer:" + n.Name,
			Timestamp: time.Now().UnixMilli(),
			Resolved:  true,
		})
	}
}

// NotifyLeave is invoked when a node is detected to have left.
// The Node argument must not be modified.
func (d *DNMSDelegate) NotifyLeave(n *memberlist.Node) {
	if n.Name == d.localName || (d.localAddr != "" && n.Addr.String() == d.localAddr) {
		return
	}
	logrus.Infof("Node left %s", n.Addr.String())
	go d.Mapper.RemovePeer(mapper.Peer{
		Name: n.Addr.String(),
		Port: int(n.Port),
	})
	// if we are an aggregator
	if d.AggMap != nil {
		d.AggMap.RemovePeer(n.Addr.String())
	}

	if d.AlertMgr != nil {
		d.AlertMgr.RecordAlert(&Alert{
			Severity:  SeverityWarning,
			Type:      "peer_leave",
			Title:     "Cluster Peer Left",
			Message:   "Node " + n.Name + " (" + n.Addr.String() + ") departed from cluster mesh",
			Entity:    "peer:" + n.Name,
			Timestamp: time.Now().UnixMilli(),
			Resolved:  false,
		})
	}
}

// NotifyUpdate is invoked when a node is detected to have
// updated, usually involving the meta data. The Node argument
// must not be modified.
func (d *DNMSDelegate) NotifyUpdate(*memberlist.Node) {
}
