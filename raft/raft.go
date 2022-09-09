// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	"errors"
	log "github.com/sirupsen/logrus"

	pb "github.com/bootjp/kvs-base/proto/pkg/eraftpb"
)

// None is a placeholder node ID used when there is no leader.
const None uint64 = 0

// StateType represents the role of a node in a cluster.
type StateType uint64

const (
	StateFollower StateType = iota
	StateCandidate
	StateLeader
)

var stmap = [...]string{
	"StateFollower",
	"StateCandidate",
	"StateLeader",
}

func (st StateType) String() string {
	return stmap[uint64(st)]
}

// ErrProposalDropped is returned when the proposal is ignored by some cases,
// so that the proposer can be notified and fail fast.
var ErrProposalDropped = errors.New("raft proposal dropped")

// Config contains the parameters to start a raft.
type Config struct {
	// ID is the identity of the local raft. ID cannot be 0.
	ID uint64

	// peers contains the IDs of all nodes (including self) in the raft cluster. It
	// should only be set when starting a new raft cluster. Restarting raft from
	// previous configuration will panic if peers is set. peer is private and only
	// used for testing right now.
	peers []uint64

	// ElectionTick is the number of Node.Tick invocations that must pass between
	// elections. That is, if a follower does not receive any message from the
	// leader of current term before ElectionTick has elapsed, it will become
	// candidate and start an election. ElectionTick must be greater than
	// HeartbeatTick. We suggest ElectionTick = 10 * HeartbeatTick to avoid
	// unnecessary leader switching.
	// ElectionTickはノードの数です。選挙と選挙の間に通過しなければならないティック呼び出し。
	// つまり、ElectionTickが経過する前にフォロワーが現在の用語のリーダーからメッセージを受信しなかった場合、そのフォロワーは候補になり、選挙を開始します。
	// ElectionTickはより大きくなければなりません
	// HeartbeatTick。不要なリーダーの切り替えを避けるために、ElectionTick=10*HeartbeatTickをお勧めします。
	ElectionTick int
	// HeartbeatTick is the number of Node.Tick invocations that must pass between
	// heartbeats. That is, a leader sends heartbeat messages to maintain its
	// leadership every HeartbeatTick ticks.
	// HeartbeatTickはノードの数です。心拍と心拍の間を通過する必要があるティック呼び出し。
	// つまり、リーダーはハートビートメッセージを送信して、HeartbeatTickのティックごとにリーダーシップを維持します。
	HeartbeatTick int

	// Storage is the storage for raft. raft generates entries and states to be
	// stored in storage. raft reads the persisted entries and states out of
	// Storage when it needs. raft reads out the previous state and configuration
	// out of storage when restarting.
	Storage Storage
	// Applied is the last applied index. It should only be set when restarting
	// raft. raft will not return entries to the application smaller or equal to
	// Applied. If Applied is unset when restarting, raft might return previous
	// applied entries. This is a very application dependent configuration.
	// 「適用済」 は、最後に適用された索引です。ラフトを再起動するときのみ設定してください。ラフトは、以下のサイズ以下のエントリをアプリケーションに返しません。
	// 適用済み。再起動時にAppliedが設定されていない場合、raftは以前に適用されたエントリを返すことがあります。これは、アプリケーションに大きく依存する構成です。
	Applied uint64
}

func (c *Config) validate() error {
	if c.ID == None {
		return errors.New("cannot use none as id")
	}

	if c.HeartbeatTick <= 0 {
		return errors.New("heartbeat tick must be greater than 0")
	}

	if c.ElectionTick <= c.HeartbeatTick {
		return errors.New("election tick must be greater than heartbeat tick")
	}

	if c.Storage == nil {
		return errors.New("storage cannot be nil")
	}

	return nil
}

// Progress represents a follower’s progress in the view of the leader. Leader maintains
// progresses of all followers, and sends entries to the follower based on its progress.
// Progressは、リーダーから見たフォロワーの進捗を表す。リーダーは フォロワーの進捗を管理し、その進捗に応じたエントリーをフォロワーに送信する。
type Progress struct {
	Match, Next uint64
}

type Raft struct {
	id uint64

	Term uint64
	Vote uint64

	// the log
	RaftLog *RaftLog

	// log replication progress of each peers
	Prs map[uint64]*Progress

	// this peer's role
	State StateType

	// votes records
	votes map[uint64]bool

	// msgs need to send
	msgs []pb.Message

	// the leader id
	Lead uint64

	// heartbeat interval, should send
	// ハートビート間隔、送信する
	heartbeatTimeout int
	// baseline of election interval
	// 選挙区間ベースライン
	electionTimeout int
	// number of ticks since it reached last heartbeatTimeout.
	// only leader keeps heartbeatElapsed.
	// 最後のheartbeatTimeoutに到達してからのtick数。leaderのみがheartbeatElapsedを保持する。
	heartbeatElapsed int
	// Ticks since it reached last electionTimeout when it is leader or candidate.
	// Number of ticks since it reached last electionTimeout or received a
	// valid message from current leader when it is a follower.
	// リーダーまたは候補である場合，それが最後のelectionTimeoutに到達してからのティック数．
	// フォロワーである場合、最後のelectionTimeoutに達してからのティック数、または現在のリーダーからの有効なメッセージを受信した数。
	electionElapsed int

	// leadTransferee is id of the leader transfer target when its value is not zero.
	// Follow the procedure defined in section 3.10 of Raft phd thesis.
	// (https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf)
	// (Used in 3A leader transfer)
	leadTransferee uint64

	// Only one conf change may be pending (in the log, but not yet
	// applied) at a time. This is enforced via PendingConfIndex, which
	// is set to a value >= the log index of the latest pending
	// configuration change (if any). Config changes are only allowed to
	// be proposed if the leader's applied index is greater than this
	// value.
	// (Used in 3A conf change)
	PendingConfIndex uint64
}

// newRaft return a raft peer with the given config
func newRaft(c *Config) *Raft {
	if err := c.validate(); err != nil {
		panic(err.Error())
	}

	prs := map[uint64]*Progress{}
	for _, peer := range c.peers {
		prs[peer] = &Progress{}
	}

	return &Raft{
		id:               c.ID,
		Term:             0,
		Vote:             0,
		State:            StateFollower,
		RaftLog:          newLog(c.Storage),
		Prs:              prs,
		votes:            map[uint64]bool{},
		msgs:             []pb.Message{},
		heartbeatTimeout: 0,
		// baseline of election interval
		electionTimeout: c.ElectionTick,

		// number of ticks since it reached last heartbeatTimeout.
		// only leader keeps heartbeatElapsed.
		// 最後のheartbeatTimeoutに到達してからのtick数。leaderのみがheartbeatElapsedを保持する。
		heartbeatElapsed: 0,
		// Ticks since it reached last electionTimeout when it is leader or candidate.
		// Number of ticks since it reached last electionTimeout or received a
		// valid message from current leader when it is a follower.
		// リーダーまたは候補である場合，それが最後のelectionTimeoutに到達してからのティック数．
		// フォロワーである場合、最後のelectionTimeoutに達してからのティック数、または現在のリーダーからの有効なメッセージを受信した数。
		electionElapsed:  0,
		leadTransferee:   0,
		PendingConfIndex: 0,
	}
	//electionElapsed: c.ElectionTick,
}

// sendAppend sends an append RPC with new entries (if any) and the
// current commit index to the given peer. Returns true if a message was sent.
// sendAppend は、指定されたピアに新しいエントリ (もしあれば) と現在のコミットインデックスを含む append RPC を送信します。
// メッセージが送信された場合は true を返します。
func (r *Raft) sendAppend(to uint64) bool {
	for i := range r.msgs {
		if r.msgs[i].To == to {
			//append(r.msgs)
		}
	}
	//r.msgs[0].To = to
	//ifur Code Here (2A).
	return false
}

// sendHeartbeat sends a heartbeat RPC to the given peer.
func (r *Raft) sendHeartbeat(to uint64) {
	r.msgs = append(r.msgs, pb.Message{From: r.id, To: to, Term: r.Term, MsgType: pb.MessageType_MsgHeartbeat})
	r.heartbeatElapsed++
	//r.electionElapsed++

	if r.heartbeatElapsed >= r.heartbeatTimeout {
		r.heartbeatElapsed = 0
		if err := r.Step(pb.Message{From: r.id, MsgType: pb.MessageType_MsgBeat}); err != nil {
			log.Printf("error occurred during checking sending heartbeat: %v", err)
		}
	}
}

// tick advances the internal logical clock by a single tick.
func (r *Raft) tick() {
	switch r.State {
	case StateLeader:
		for u := range r.Prs {
			if r.id == u {
				continue
			}
			r.sendHeartbeat(u)
		}

	case StateFollower:
		r.electionElapsed++
		if r.electionElapsed >= r.electionTimeout {
			r.electionElapsed = 0
			r.becomeCandidate()
			for u := range r.Prs {
				if r.id == u {
					continue
				}
				r.msgs = append(r.msgs, pb.Message{From: r.id, To: u, MsgType: pb.MessageType_MsgRequestVote, Term: r.Term})
			}
		}

	case StateCandidate:
		r.electionElapsed++
		if r.electionElapsed >= r.electionTimeout {
			r.electionElapsed = 0
			r.Term++
			for u := range r.Prs {
				if r.id == u {
					continue
				}
				r.msgs = append(r.msgs, pb.Message{From: r.id, To: u, MsgType: pb.MessageType_MsgRequestVote, Term: r.Term})
			}
		}
	}
}

// becomeFollower transform this peer's state to Follower
func (r *Raft) becomeFollower(term uint64, lead uint64) {
	r.State = StateFollower
	r.Lead = lead
	r.Term = term
}

// becomeCandidate transform this peer's state to candidate
func (r *Raft) becomeCandidate() {
	r.State = StateCandidate
	r.Term++
	r.votes = map[uint64]bool{}
	r.votes[r.id] = true
	r.Vote = r.id
}

// becomeLeader transform this peer's state to leader
func (r *Raft) becomeLeader() {
	r.Lead = r.id
	r.State = StateLeader
	r.msgs = append(r.msgs, pb.Message{MsgType: pb.MessageType_MsgPropose, From: r.id, To: r.id})
	// becomeLeader
	//r.Term = r.Term
	// Your Code Here (2A).
	// NOTE: Leader should propose a noop entry on its term
}

// Step the entrance of handle message, see `MessageType`
// on `eraftpb.proto` for what msgs should be handled
func (r *Raft) Step(m pb.Message) error {
	// Your Code Here (2A).
	switch r.State {
	case StateFollower:
		r.Term = m.Term
		if m.MsgType == pb.MessageType_MsgAppend {
			for _, entry := range m.Entries {
				r.RaftLog.entries = append(r.RaftLog.entries, *entry)
			}
		}
	case StateCandidate:
		if r.Term < m.Term {
			r.becomeFollower(m.Term, m.From)
		}
		r.sendAppend(m.From)
	case StateLeader:
		if r.Term < m.Term {
			r.becomeFollower(m.Term, m.From)
		}
		r.sendAppend(m.From)

	}
	return nil
}

// handleAppendEntries handle AppendEntries RPC request
func (r *Raft) handleAppendEntries(m pb.Message) {

	// Your Code Here (2A).
}

// handleHeartbeat handle Heartbeat RPC request
func (r *Raft) handleHeartbeat(m pb.Message) {
	r.msgs = append(r.msgs, pb.Message{MsgType: pb.MessageType_MsgHeartbeatResponse, From: r.id, To: m.From})
	//r
	// Your Code Here (2A).
}

// handleSnapshot handle Snapshot RPC request
func (r *Raft) handleSnapshot(m pb.Message) {
	// Your Code Here (2C).
}

// addNode add a new node to raft group
func (r *Raft) addNode(id uint64) {
	r.Prs[id] = &Progress{}
	// Your Code Here (3A).
}

// removeNode remove a node from raft group
func (r *Raft) removeNode(id uint64) {
	delete(r.Prs, id)
	// Your Code Here (3A).
}
