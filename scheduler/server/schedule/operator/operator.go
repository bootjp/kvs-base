// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bootjp/kvs-base/proto/pkg/metapb"
	"github.com/bootjp/kvs-base/scheduler/server/core"
	"github.com/pingcap/log"
	"go.uber.org/zap"
)

const (
	// LeaderOperatorWaitTime is the duration that when a leader operator lives
	// longer than it, the operator will be considered timeout.
	LeaderOperatorWaitTime = 10 * time.Second
	// RegionOperatorWaitTime is the duration that when a region operator lives
	// longer than it, the operator will be considered timeout.
	RegionOperatorWaitTime = 10 * time.Minute
)

// Cluster provides an overview of a cluster's regions distribution.
type Cluster interface {
	GetStore(id uint64) *core.StoreInfo
	AllocPeer(storeID uint64) (*metapb.Peer, error)
}

// OpStep describes the basic scheduling steps that can not be subdivided.
type OpStep interface {
	fmt.Stringer
	ConfVerChanged(region *core.RegionInfo) bool
	IsFinish(region *core.RegionInfo) bool
}

// TransferLeader is an OpStep that transfers a region's leader.
type TransferLeader struct {
	FromStore, ToStore uint64
}

// ConfVerChanged returns true if the conf version has been changed by this step
func (tl TransferLeader) ConfVerChanged(region *core.RegionInfo) bool {
	return false // transfer leader never change the conf version
}

func (tl TransferLeader) String() string {
	return fmt.Sprintf("transfer leader from store %v to store %v", tl.FromStore, tl.ToStore)
}

// IsFinish checks if current step is finished.
func (tl TransferLeader) IsFinish(region *core.RegionInfo) bool {
	return region.GetLeader().GetStoreId() == tl.ToStore
}

// AddPeer is an OpStep that adds a region peer.
type AddPeer struct {
	ToStore, PeerID uint64
}

// ConfVerChanged returns true if the conf version has been changed by this step
func (ap AddPeer) ConfVerChanged(region *core.RegionInfo) bool {
	if p := region.GetStoreVoter(ap.ToStore); p != nil {
		return p.GetId() == ap.PeerID
	}
	return false
}
func (ap AddPeer) String() string {
	return fmt.Sprintf("add peer %v on store %v", ap.PeerID, ap.ToStore)
}

// IsFinish checks if current step is finished.
func (ap AddPeer) IsFinish(region *core.RegionInfo) bool {
	if p := region.GetStoreVoter(ap.ToStore); p != nil {
		if p.GetId() != ap.PeerID {
			log.Warn("obtain unexpected peer", zap.String("expect", ap.String()), zap.Uint64("obtain-voter", p.GetId()))
			return false
		}
		return region.GetPendingVoter(p.GetId()) == nil
	}
	return false
}

// RemovePeer is an OpStep that removes a region peer.
type RemovePeer struct {
	FromStore uint64
}

// ConfVerChanged returns true if the conf version has been changed by this step
func (rp RemovePeer) ConfVerChanged(region *core.RegionInfo) bool {
	return region.GetStorePeer(rp.FromStore) == nil
}

func (rp RemovePeer) String() string {
	return fmt.Sprintf("remove peer on store %v", rp.FromStore)
}

// IsFinish checks if current step is finished.
func (rp RemovePeer) IsFinish(region *core.RegionInfo) bool {
	return region.GetStorePeer(rp.FromStore) == nil
}

// Operator contains execution steps generated by scheduler.
type Operator struct {
	desc        string
	brief       string
	regionID    uint64
	regionEpoch *metapb.RegionEpoch
	kind        OpKind
	steps       []OpStep
	currentStep int32
	createTime  time.Time
	// startTime is used to record the start time of an operator which is added into running operators.
	startTime time.Time
	stepTime  int64
	level     core.PriorityLevel
}

// NewOperator creates a new operator.
func NewOperator(desc, brief string, regionID uint64, regionEpoch *metapb.RegionEpoch, kind OpKind, steps ...OpStep) *Operator {
	level := core.NormalPriority
	if kind&OpAdmin != 0 {
		level = core.HighPriority
	}
	return &Operator{
		desc:        desc,
		brief:       brief,
		regionID:    regionID,
		regionEpoch: regionEpoch,
		kind:        kind,
		steps:       steps,
		createTime:  time.Now(),
		stepTime:    time.Now().UnixNano(),
		level:       level,
	}
}

func (o *Operator) String() string {
	stepStrs := make([]string, len(o.steps))
	for i := range o.steps {
		stepStrs[i] = o.steps[i].String()
	}
	s := fmt.Sprintf("%s {%s} (kind:%s, region:%v(%v,%v), createAt:%s, startAt:%s, currentStep:%v, steps:[%s])", o.desc, o.brief, o.kind, o.regionID, o.regionEpoch.GetVersion(), o.regionEpoch.GetConfVer(), o.createTime, o.startTime, atomic.LoadInt32(&o.currentStep), strings.Join(stepStrs, ", "))
	if o.IsTimeout() {
		s = s + " timeout"
	}
	if o.IsFinish() {
		s = s + " finished"
	}
	return s
}

// MarshalJSON serializes custom types to JSON.
func (o *Operator) MarshalJSON() ([]byte, error) {
	return []byte(`"` + o.String() + `"`), nil
}

// Desc returns the operator's short description.
func (o *Operator) Desc() string {
	return o.desc
}

// SetDesc sets the description for the operator.
func (o *Operator) SetDesc(desc string) {
	o.desc = desc
}

// AttachKind attaches an operator kind for the operator.
func (o *Operator) AttachKind(kind OpKind) {
	o.kind |= kind
}

// RegionID returns the region that operator is targeted.
func (o *Operator) RegionID() uint64 {
	return o.regionID
}

// RegionEpoch returns the region's epoch that is attached to the operator.
func (o *Operator) RegionEpoch() *metapb.RegionEpoch {
	return o.regionEpoch
}

// Kind returns operator's kind.
func (o *Operator) Kind() OpKind {
	return o.kind
}

// ElapsedTime returns duration since it was created.
func (o *Operator) ElapsedTime() time.Duration {
	return time.Since(o.createTime)
}

// RunningTime returns duration since it was promoted.
func (o *Operator) RunningTime() time.Duration {
	return time.Since(o.startTime)
}

// SetStartTime sets the start time for operator.
func (o *Operator) SetStartTime(t time.Time) {
	o.startTime = t
}

// GetStartTime ges the start time for operator.
func (o *Operator) GetStartTime() time.Time {
	return o.startTime
}

// Len returns the operator's steps count.
func (o *Operator) Len() int {
	return len(o.steps)
}

// Step returns the i-th step.
func (o *Operator) Step(i int) OpStep {
	if i >= 0 && i < len(o.steps) {
		return o.steps[i]
	}
	return nil
}

// Check checks if current step is finished, returns next step to take action.
// It's safe to be called by multiple goroutine concurrently.
func (o *Operator) Check(region *core.RegionInfo) OpStep {
	for step := atomic.LoadInt32(&o.currentStep); int(step) < len(o.steps); step++ {
		if o.steps[int(step)].IsFinish(region) {
			atomic.StoreInt32(&o.currentStep, step+1)
			atomic.StoreInt64(&o.stepTime, time.Now().UnixNano())
		} else {
			return o.steps[int(step)]
		}
	}
	return nil
}

// ConfVerChanged returns the number of confver has consumed by steps
func (o *Operator) ConfVerChanged(region *core.RegionInfo) int {
	total := 0
	current := atomic.LoadInt32(&o.currentStep)
	if current == int32(len(o.steps)) {
		current--
	}
	// including current step, it may has taken effects in this heartbeat
	for _, step := range o.steps[0 : current+1] {
		if step.ConfVerChanged(region) {
			total++
		}
	}
	return total
}

// SetPriorityLevel sets the priority level for operator.
func (o *Operator) SetPriorityLevel(level core.PriorityLevel) {
	o.level = level
}

// GetPriorityLevel gets the priority level.
func (o *Operator) GetPriorityLevel() core.PriorityLevel {
	return o.level
}

// IsFinish checks if all steps are finished.
func (o *Operator) IsFinish() bool {
	return atomic.LoadInt32(&o.currentStep) >= int32(len(o.steps))
}

// IsTimeout checks the operator's create time and determines if it is timeout.
func (o *Operator) IsTimeout() bool {
	var timeout bool
	if o.IsFinish() {
		return false
	}
	if o.startTime.IsZero() {
		return false
	}
	if o.kind&OpRegion != 0 {
		timeout = time.Since(o.startTime) > RegionOperatorWaitTime
	} else {
		timeout = time.Since(o.startTime) > LeaderOperatorWaitTime
	}
	if timeout {
		return true
	}
	return false
}

// CreateAddPeerOperator creates an operator that adds a new peer.
func CreateAddPeerOperator(desc string, region *core.RegionInfo, peerID uint64, toStoreID uint64, kind OpKind) *Operator {
	steps := CreateAddPeerSteps(toStoreID, peerID)
	brief := fmt.Sprintf("add peer: store %v", toStoreID)
	return NewOperator(desc, brief, region.GetID(), region.GetRegionEpoch(), kind|OpRegion, steps...)
}

// CreateRemovePeerOperator creates an operator that removes a peer from region.
func CreateRemovePeerOperator(desc string, cluster Cluster, kind OpKind, region *core.RegionInfo, storeID uint64) (*Operator, error) {
	removeKind, steps, err := removePeerSteps(cluster, region, storeID, getRegionFollowerIDs(region))
	if err != nil {
		return nil, err
	}
	brief := fmt.Sprintf("rm peer: store %v", storeID)
	return NewOperator(desc, brief, region.GetID(), region.GetRegionEpoch(), removeKind|kind, steps...), nil
}

// CreateAddPeerSteps creates an OpStep list that add a new peer.
func CreateAddPeerSteps(newStore uint64, peerID uint64) []OpStep {
	st := []OpStep{
		AddPeer{ToStore: newStore, PeerID: peerID},
	}
	return st
}

// CreateTransferLeaderOperator creates an operator that transfers the leader from a source store to a target store.
func CreateTransferLeaderOperator(desc string, region *core.RegionInfo, sourceStoreID uint64, targetStoreID uint64, kind OpKind) *Operator {
	step := TransferLeader{FromStore: sourceStoreID, ToStore: targetStoreID}
	brief := fmt.Sprintf("transfer leader: store %v to %v", sourceStoreID, targetStoreID)
	return NewOperator(desc, brief, region.GetID(), region.GetRegionEpoch(), kind|OpLeader, step)
}

// interleaveStepGroups interleaves two slice of step groups. For example:
//
//  a = [[opA1, opA2], [opA3], [opA4, opA5, opA6]]
//  b = [[opB1], [opB2], [opB3, opB4], [opB5, opB6]]
//  c = interleaveStepGroups(a, b, 0)
//  c == [opA1, opA2, opB1, opA3, opB2, opA4, opA5, opA6, opB3, opB4, opB5, opB6]
//
// sizeHint is a hint for the capacity of returned slice.
func interleaveStepGroups(a, b [][]OpStep, sizeHint int) []OpStep {
	steps := make([]OpStep, 0, sizeHint)
	i, j := 0, 0
	for ; i < len(a) && j < len(b); i, j = i+1, j+1 {
		steps = append(steps, a[i]...)
		steps = append(steps, b[j]...)
	}
	for ; i < len(a); i++ {
		steps = append(steps, a[i]...)
	}
	for ; j < len(b); j++ {
		steps = append(steps, b[j]...)
	}
	return steps
}

// CreateMovePeerOperator creates an operator that replaces an old peer with a new peer.
func CreateMovePeerOperator(desc string, cluster Cluster, region *core.RegionInfo, kind OpKind, oldStore, newStore uint64, peerID uint64) (*Operator, error) {
	removeKind, steps, err := removePeerSteps(cluster, region, oldStore, append(getRegionFollowerIDs(region), newStore))
	if err != nil {
		return nil, err
	}
	st := CreateAddPeerSteps(newStore, peerID)
	steps = append(st, steps...)
	brief := fmt.Sprintf("mv peer: store %v to %v", oldStore, newStore)
	return NewOperator(desc, brief, region.GetID(), region.GetRegionEpoch(), removeKind|kind|OpRegion, steps...), nil
}

// CreateOfflinePeerOperator creates an operator that replaces an old peer with a new peer when offline a store.
func CreateOfflinePeerOperator(desc string, cluster Cluster, region *core.RegionInfo, kind OpKind, oldStore, newStore uint64, peerID uint64) (*Operator, error) {
	k, steps, err := transferLeaderStep(cluster, region, oldStore, append(getRegionFollowerIDs(region)))
	if err != nil {
		return nil, err
	}
	kind |= k
	st := CreateAddPeerSteps(newStore, peerID)
	steps = append(steps, st...)
	steps = append(steps, RemovePeer{FromStore: oldStore})
	brief := fmt.Sprintf("mv peer: store %v to %v", oldStore, newStore)
	return NewOperator(desc, brief, region.GetID(), region.GetRegionEpoch(), kind|OpRegion, steps...), nil
}

func getRegionFollowerIDs(region *core.RegionInfo) []uint64 {
	var ids []uint64
	for id := range region.GetFollowers() {
		ids = append(ids, id)
	}
	return ids
}

// removePeerSteps returns the steps to safely remove a peer. It prevents removing leader by transfer its leadership first.
func removePeerSteps(cluster Cluster, region *core.RegionInfo, storeID uint64, followerIDs []uint64) (kind OpKind, steps []OpStep, err error) {
	kind, steps, err = transferLeaderStep(cluster, region, storeID, followerIDs)
	if err != nil {
		return
	}

	steps = append(steps, RemovePeer{FromStore: storeID})
	kind |= OpRegion
	return
}

func transferLeaderStep(cluster Cluster, region *core.RegionInfo, storeID uint64, followerIDs []uint64) (kind OpKind, steps []OpStep, err error) {
	if region.GetLeader() != nil && region.GetLeader().GetStoreId() == storeID {
		kind, steps, err = transferLeaderToSuitableSteps(cluster, storeID, followerIDs)
		if err != nil {
			log.Debug("failed to create transfer leader step", zap.Uint64("region-id", region.GetID()), zap.Error(err))
			return
		}
	}
	return
}

// findAvailableStore finds the first available store.
func findAvailableStore(cluster Cluster, storeIDs []uint64) (int, uint64) {
	for i, id := range storeIDs {
		store := cluster.GetStore(id)
		if store != nil {
			return i, id
		} else {
			log.Debug("nil store", zap.Uint64("store-id", id))
		}
	}
	return -1, 0
}

// transferLeaderToSuitableSteps returns the first suitable store to become region leader.
// Returns an error if there is no suitable store.
func transferLeaderToSuitableSteps(cluster Cluster, leaderID uint64, storeIDs []uint64) (OpKind, []OpStep, error) {
	_, id := findAvailableStore(cluster, storeIDs)
	if id != 0 {
		return OpLeader, []OpStep{TransferLeader{FromStore: leaderID, ToStore: id}}, nil
	}
	return 0, nil, errors.New("no suitable store to become region leader")
}
