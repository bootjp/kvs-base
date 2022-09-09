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

/*
Package raft sends and receives messages in the Protocol Buffer format
defined in the eraftpb package.

Raft is a protocol with which a cluster of nodes can maintain a replicated state machine.
The state machine is kept in sync through the use of a replicated log.
For more details on Raft, see "In Search of an Understandable Consensus Algorithm"

Raftは、クラスタのノードが複製されたステートマシンを維持するためのプロトコルである。
ステートマシンは複製されたログを利用して同期される。
Raftの詳細については、"理解しやすいコンセンサス・アルゴリズムを求めて "を参照してください。

(https://ramcloud.stanford.edu/raft.pdf) by Diego Ongaro and John Ousterhout.

Usage

The primary object in raft is a Node. You either start a Node from scratch
using raft.StartNode or start a Node from some initial state using raft.RestartNode.
raftの主要なオブジェクトはNodeです。
Nodeをゼロから開始するには を使用してゼロからNodeを開始するか、 raft.RestartNodeを使用していくつかの初期状態からNodeを開始します。

To start a node from scratch:

  storage := raft.NewMemoryStorage()
  c := &Config{
    ID:              0x01,
    ElectionTick:    10,
    HeartbeatTick:   1,
    Storage:         storage,
  }
  n := raft.StartNode(c, []raft.Peer{{ID: 0x02}, {ID: 0x03}})

To restart a node from previous state:

  storage := raft.NewMemoryStorage()

  // recover the in-memory storage from persistent
  // snapshot, state and entries.
  storage.ApplySnapshot(snapshot)
  storage.SetHardState(state)
  storage.Append(entries)

  c := &Config{
    ID:              0x01,
    ElectionTick:    10,
    HeartbeatTick:   1,
    Storage:         storage,
    MaxInflightMsgs: 256,
  }

  // restart raft without peer information.
  // peer information is already included in the storage.
  n := raft.RestartNode(c)

Now that you are holding onto a Node you have a few responsibilities:

First, you must read from the Node.Ready() channel and process the updates
it contains. These steps may be performed in parallel, except as noted in step
2.

まず、Node.Ready() チャネルから読み込み、それが含む更新を処理する必要があります。
これらのステップは、step2 の記述を除き、並行して実行することができます。


1. Write HardState, Entries, and Snapshot to persistent storage if they are
not empty. Note that when writing an Entry with Index i, any
previously-persisted entries with Index >= i must be discarded.

1. HardState、Entries、Snapshotが空でなければ、永続的なストレージに書き込む。
インデックスiを持つEntryを書き込むとき、インデックス >= iを持つ以前から存在するEntryはすべて捨てなければならないことに注意。



2. Send all Messages to the nodes named in the To field. It is important that
no messages be sent until the latest HardState has been persisted to disk,
and all Entries written by any previous Ready batch (Messages may be sent while
entries from the same batch are being persisted).

2. Toフィールドに指定されたノードにすべてのメッセージを送信します。
最新のHardStateがディスクに保持され、以前のReadyバッチで書き込まれたすべてのエントリが書き込まれるまで、メッセージを送信しないことが重要です（同じバッチのエントリが保持されている間は、メッセージを送信することができます）。


Note: Marshalling messages is not thread-safe; it is important that you
make sure that no new entries are persisted while marshalling.
The easiest way to achieve this is to serialize the messages directly inside
your main raft loop.
注：メッセージのマーシャリングはスレッドセーフではありません。
マーシャリング中に新しいエントリが永続化されないようにすることが重要です。

3. Apply Snapshot (if any) and CommittedEntries to the state machine.
If any committed Entry has Type EntryType_EntryConfChange, call Node.ApplyConfChange()
to apply it to the node. The configuration change may be cancelled at this point
by setting the NodeId field to zero before calling ApplyConfChange
(but ApplyConfChange must be called one way or the other, and the decision to cancel
must be based solely on the state machine and not external information such as
the observed health of the node).

3. Snapshot（ある場合）とCommittedEntriesをステートマシンに適用します。
コミットされたEntryがType EntryType_EntryConfChangeを持っている場合、Node.ApplyConfChange()を呼び出してノードに適用します。
 この時点で、ApplyConfChange を呼び出す前に NodeId フィールドをゼロに設定することで、構成変更をキャンセルできます（ただし、ApplyConfChange はいずれにせよ呼び出す必要があり、キャンセルする決定は、ノードの状態観察などの外部情報ではなくステートマシンだけに基づいて行われる必要があります）。


4. Call Node.Advance() to signal readiness for the next batch of updates.
This may be done at any time after step 1, although all updates must be processed
in the order they were returned by Ready.

4. 4. Node.Advance() を呼び出して、次の更新バッチの準備完了を通知します。
これはステップ 1 の後いつでも行うことができますが、すべての更新は Ready によって返された順序で処理されなければなりません。



Second, all persisted log entries must be made available via an
implementation of the Storage interface. The provided MemoryStorage
type can be used for this (if you repopulate its state upon a
restart), or you can supply your own disk-backed implementation.

次に、ストレージインターフェイスの実装を介して、すべての永続化されたログエントリを使用できるようにする必要があります。

提供されている MemoryStorage 型はこのために使うことができます (再起動時にその状態を再投入するのであれば)。
または、独自のディスクバックアップされた実装を提供することができます。

Third, when you receive a message from another node, pass it to Node.Step:

	func recvRaftRPC(ctx context.Context, m eraftpb.Message) {
		n.Step(ctx, m)
	}

Finally, you need to call Node.Tick() at regular intervals (probably
via a time.Ticker). Raft has two important timeouts: heartbeat and the
election timeout. However, internally to the raft package time is
represented by an abstract "tick".

最後に、一定時間ごとに Node.Tick() を呼び出す必要があります (おそらく time.Ticker を使って)。
Raft には 2 つの重要なタイムアウトがあります: heartbeat と election timeout です。
しかし、raft パッケージの内部では、時間は抽象的な "tick" で表現されます。


The total state machine handling loop will look something like this:

  for {
    select {
    case <-s.Ticker:
      n.Tick()
    case rd := <-s.Node.Ready():
      saveToStorage(rd.State, rd.Entries, rd.Snapshot)
      send(rd.Messages)
      if !raft.IsEmptySnap(rd.Snapshot) {
        processSnapshot(rd.Snapshot)
      }
      for _, entry := range rd.CommittedEntries {
        process(entry)
        if entry.Type == eraftpb.EntryType_EntryConfChange {
          var cc eraftpb.ConfChange
          cc.Unmarshal(entry.Data)
          s.Node.ApplyConfChange(cc)
        }
      }
      s.Node.Advance()
    case <-s.done:
      return
    }
  }

To propose changes to the state machine from your node take your application
data, serialize it into a byte slice and call:
ノードからステートマシンの変更を提案するには、アプリケーションのデータ データを取得し、バイトスライスにシリアライズして呼び出します。

	n.Propose(data)

If the proposal is committed, data will appear in committed entries with type
eraftpb.EntryType_EntryNormal. There is no guarantee that a proposed command will be
committed; you may have to re-propose after a timeout.

提案がコミットされた場合、データはコミットされたエントリに "eraftpb.EntryType_EntryNormal" というタイプで表示されます。
提案されたコマンドがコミットされる保証はありません。タイムアウト後に再提案する必要があるかもしれません。

To add or remove a node in a cluster, build ConfChange struct 'cc' and call:

	n.ProposeConfChange(cc)

After config change is committed, some committed entry with type
eraftpb.EntryType_EntryConfChange will be returned. You must apply it to node through:
コンフィグレーションの変更がコミットされると、コミットされたエントリーのうち "eraftpb.EntryType_EntryConfChange" を持つコミットされたエントリーが返されます。
それをノードに適用する必要があります。

	var cc eraftpb.ConfChange
	cc.Unmarshal(data)
	n.ApplyConfChange(cc)

Note: An ID represents a unique node in a cluster for all time. A
given ID MUST be used only once even if the old node has been removed.
This means that for example IP addresses make poor node IDs since they
may be reused. Node IDs must be non-zero.

注：IDは、クラスタ内で常に一意のノードを表します。
A 与えられたIDは、古いノードが削除されても一度だけ使用しなければなりません（MUST）。
これは、例えばIPアドレスは再利用される可能性があるため、ノードIDとしては不適当であることを意味します。は再利用される可能性があります。
ノードIDはゼロであってはならない。


Implementation notes

This implementation is up to date with the final Raft thesis
(https://ramcloud.stanford.edu/~ongaro/thesis.pdf), although our
implementation of the membership change protocol differs somewhat from
that described in chapter 4. The key invariant that membership changes
happen one node at a time is preserved, but in our implementation the
membership change takes effect when its entry is applied, not when it
is added to the log (so the entry is committed under the old
membership instead of the new). This is equivalent in terms of safety,
since the old and new configurations are guaranteed to overlap.

この実装は、最終的なRaftの論文 (https://ramcloud.stanford.edu/~ongaro/thesis.pdf) に対応しています。
メンバーチェンジのプロトコルの実装は4章で説明したものとは多少異なりますが とは多少異なります。
メンバーチェンジは一度に1つのノードで起こるという重要な不変性 しかし、我々の実装では、メンバーシップの変更は、そのエントリーが を適用したときではなく、そのエントリーがログに追加されたときに、メンバーシップの変更が有効になります。
ログに追加されたときではなく、そのエントリーが適用されたときに有効となります（したがって、エントリーは新しいメンバーシップではなく古いメンバーシップの下でコミットされます）。の下にコミットされる)。
これは安全性の面では同等である。というのも、古い構成と新しい構成は重なることが保証されているからです。

To ensure that we do not attempt to commit two membership changes at
once by matching log positions (which would be unsafe since they
should have different quorum requirements), we simply disallow any
proposed membership change while any uncommitted change appears in
the leader's log.

ログ位置を一致させることによって2つのメンバーシップの変更を一度にコミットしようとしないようにするには (クォーラム要件が異なるため安全ではありません) 、リーダーのログにコミットされていない変更が表示されている間は、提案されたメンバーシップの変更を許可しないようにします。


This approach introduces a problem when you try to remove a member
from a two-member cluster: If one of the members dies before the
other one receives the commit of the confchange entry, then the member
cannot be removed any more since the cluster cannot make progress.
For this reason it is highly recommended to use three or more nodes in
every cluster.

この方法では、2メンバーのクラスタからメンバーを削除しようとすると問題が発生します。メンバーの1つがconfchangeエントリのコミットを受信する前に死んでしまうと、クラスタが進行できなくなるため、メンバーを削除できなくなります。
このため、すべてのクラスターで3つ以上のノードを使用することを強くお勧めします。

MessageType

Package raft sends and receives message in Protocol Buffer format (defined
in eraftpb package). Each state (follower, candidate, leader) implements its
own 'step' method ('stepFollower', 'stepCandidate', 'stepLeader') when
advancing with the given eraftpb.Message. Each step is determined by its
eraftpb.MessageType. Note that every step is checked by one common method
'Step' that safety-checks the terms of node and incoming message to prevent
stale log entries:

パッケージ・ラフトは、プロトコル・バッファ形式 (eraftpbパッケージで定義) でメッセージを送受信します。各状態(追従者、候補者、リーダー)は、指定されたeraftpbで進むときに、独自の'step'メソッド(「stepFollower」 、 「stepCandidate」 、 「stepLeader」)を実装します。メッセージ。
各ステップは、そのeraftpbによって決定される。MessageType。すべてのステップは、ノードと受信メッセージの条件を安全性チェックして古いログエントリを防止する1つの共通メソッド 「Step」 によってチェックされることに注意してください。

	'MessageType_MsgHup' is used for election. If a node is a follower or candidate, the
	'tick' function in 'raft' struct is set as 'tickElection'. If a follower or
	candidate has not received any heartbeat before the election timeout, it
	passes 'MessageType_MsgHup' to its Step method and becomes (or remains) a candidate to
	start a new election.

	'MessageType_MsgHup' は選挙に使用される。
	ノードがフォロワーまたは候補者である場合、raft 構造体の raft構造体のtick関数にtickElectionが設定される。
	フォロワーまたはキャンディデイトが フォロワーまたはキャンディデイトが選挙タイムアウトまでにハートビートを受信しなかった場合、そのノードは MessageType_MsgHup を Step メソッドに渡し、候補となる(あるいは候補のまま)。になり、新しい選挙を開始する。

	'MessageType_MsgBeat' is an internal type that signals the leader to send a heartbeat of
	the 'MessageType_MsgHeartbeat' type. If a node is a leader, the 'tick' function in
	the 'raft' struct is set as 'tickHeartbeat', and triggers the leader to
	send periodic 'MessageType_MsgHeartbeat' messages to its followers.

	'MessageType_MsgBeat' は 'MessageType_MsgHeartbeat' タイプのハートビートを送信するようにリーダーにシグナルを送る内部タイプである。
	ノードがリーダーの場合、'raft' 構造体の 'tick' 関数は 'tickHeartbeat' として設定され、リーダーが定期的に 'MessageType_MsgHeartbeat' メッセージをフォロワーに送信するトリガーとなる。

	'MessageType_MsgPropose' proposes to append data to its log entries. This is a special
	type to redirect proposals to the leader. Therefore, send method overwrites
	eraftpb.Message's term with its HardState's term to avoid attaching its
	local term to 'MessageType_MsgPropose'. When 'MessageType_MsgPropose' is passed to the leader's 'Step'
	method, the leader first calls the 'appendEntry' method to append entries
	to its log, and then calls 'bcastAppend' method to send those entries to
	its peers. When passed to candidate, 'MessageType_MsgPropose' is dropped. When passed to
	follower, 'MessageType_MsgPropose' is stored in follower's mailbox(msgs) by the send
	method. It is stored with sender's ID and later forwarded to the leader by
	rafthttp package.

	MessageType_MsgPropose' は、そのログエントリにデータを追加することを提案する。これは、プロポーザルをリーダーにリダイレクトするための特殊なタイプです。
	そのため、sendメソッドは eraftpb.Message の項を HardState の項で上書きして 'MessageType_MsgPropose' にローカル項を付けないようにしている。
	MessageType_MsgPropose' がリーダーの 'Step' メソッドに渡されると、リーダーはまず 'appendEntry' メソッドを呼んでログにエントリを追加し、次に 'bcastAppend' メソッドを呼んでそのエントリをピアに送信する。
	候補に渡されると、'MessageType_MsgPropose' は削除される。followerに渡されると、sendメソッドによって'MessageType_MsgPropose'がfollowerのメールボックス(msgs)に格納される。これは送信者のIDとともに保存され、後にrafthttpパッケージによってリーダーに転送される。

	'MessageType_MsgAppend' contains log entries to replicate. A leader calls bcastAppend,
	which calls sendAppend, which sends soon-to-be-replicated logs in 'MessageType_MsgAppend'
	type. When 'MessageType_MsgAppend' is passed to candidate's Step method, candidate reverts
	back to follower, because it indicates that there is a valid leader sending
	'MessageType_MsgAppend' messages. Candidate and follower respond to this message in
	'MessageType_MsgAppendResponse' type.

	MessageType_MsgAppend'には、複製するログ・エントリが格納されている。リーダーはbcastAppendを呼び出し、sendAppendを呼び出して、もうすぐ複製されるログを'MessageType_MsgAppend'型で送信する。
	MessageType_MsgAppend'が候補のStepメソッドに渡されると、候補はフォロワーに戻る。これは、'MessageType_MsgAppend'メッセージを送る有効なリーダーが存在することを示すからである。候補とフォロワーはこのメッセージに対して 'MessageType_MsgAppendResponse' 型で応答する。

	'MessageType_MsgAppendResponse' is response to log replication request('MessageType_MsgAppend'). When
	'MessageType_MsgAppend' is passed to candidate or follower's Step method, it responds by
	calling 'handleAppendEntries' method, which sends 'MessageType_MsgAppendResponse' to raft
	mailbox.

	MessageType_MsgAppendResponse'はログレプリケーション要求('MessageType_MsgAppend')に対する応答である。
	MessageType_MsgAppend'が候補やフォロワーのStepメソッドに渡されると、'handleAppendEntries'メソッドを呼んで応答し、'MessageType_MsgAppendResponse'をラフトメールボックスに送っています。

	'MessageType_MsgRequestVote' requests votes for election. When a node is a follower or
	candidate and 'MessageType_MsgHup' is passed to its Step method, then the node calls
	'campaign' method to campaign itself to become a leader. Once 'campaign'
	method is called, the node becomes candidate and sends 'MessageType_MsgRequestVote' to peers
	in cluster to request votes. When passed to the leader or candidate's Step
	method and the message's Term is lower than leader's or candidate's,
	'MessageType_MsgRequestVote' will be rejected ('MessageType_MsgRequestVoteResponse' is returned with Reject true).
	If leader or candidate receives 'MessageType_MsgRequestVote' with higher term, it will revert
	back to follower. When 'MessageType_MsgRequestVote' is passed to follower, it votes for the
	sender only when sender's last term is greater than MessageType_MsgRequestVote's term or
	sender's last term is equal to MessageType_MsgRequestVote's term but sender's last committed
	index is greater than or equal to follower's.

	MessageType_MsgRequestVote' は選挙のための投票を要求します。
	Stepメソッドに 'MessageType_MsgHup' が渡されると、ノードは 'campaign' メソッドを呼び出してリーダーになるための選挙運動を行います。
	campaign' メソッドが呼ばれると、ノードは候補となり、クラスタ内のピアに 'MessageType_MsgRequestVote' を送信して投票を要求する。
	リーダーや候補の Step メソッドに渡されたメッセージの Term がリーダーや候補の Term よりも小さい場合、'MessageType_MsgRequestVote' は拒否されます (Reject true で 'MessageType_MsgRequestVoteResponse' が返される)。
	リーダーや候補がより高い項の 'MessageType_MsgRequestVote' を受け取った場合、フォロワーに戻ることになります。
	MessageType_MsgRequestVote' が follower に渡された場合、送信者の最終タームが MessageType_MsgRequestVote のタームより大きいか、送信者の最終タームが MessageType_MsgRequestVote のタームと同じだが送信者の最終コミットインデックスが follower と同じかそれ以上の時のみ、送信者に投票します。

	'MessageType_MsgRequestVoteResponse' contains responses from voting request. When 'MessageType_MsgRequestVoteResponse' is
	passed to candidate, the candidate calculates how many votes it has won. If
	it's more than majority (quorum), it becomes leader and calls 'bcastAppend'.
	If candidate receives majority of votes of denials, it reverts back to
	follower.

	MessageType_MsgRequestVoteResponse' は投票要求に対する応答である。
	MessageType_MsgRequestVoteResponse'が候補者に渡されると、候補者は自分が何票獲得したかを計算する。それが過半数（quorum）以上であれば、リーダーとなって 'bcastAppend' を呼び出す。
	候補が過半数の拒否票を獲得した場合、候補はフォロワーに戻る。


	'MessageType_MsgSnapshot' requests to install a snapshot message. When a node has just
	become a leader or the leader receives 'MessageType_MsgPropose' message, it calls
	'bcastAppend' method, which then calls 'sendAppend' method to each
	follower. In 'sendAppend', if a leader fails to get term or entries,
	the leader requests snapshot by sending 'MessageType_MsgSnapshot' type message.

	MessageType_MsgSnapshot' は、スナップショットメッセージのインストールを要求します。
	リーダーになったばかりのノードやリーダーが「MessageType_MsgPropose」メッセージを受信すると、「bcastAppend」メソッドを呼び出し、「sendAppend」メソッドを各フォロワーに呼び出す。
	sendAppend」メソッドでは、リーダーがタームやエントリーを取得できなかった場合、リーダーは「MessageType_MsgSnapshot」タイプのメッセージを送信してスナップショットを要求する。

	'MessageType_MsgHeartbeat' sends heartbeat from leader. When 'MessageType_MsgHeartbeat' is passed
	to candidate and message's term is higher than candidate's, the candidate
	reverts back to follower and updates its committed index from the one in
	this heartbeat. And it sends the message to its mailbox. When
	'MessageType_MsgHeartbeat' is passed to follower's Step method and message's term is
	higher than follower's, the follower updates its leaderID with the ID
	from the message.

	MessageType_MsgHeartbeat' はリーダーからのハートビートを送信する。
	MessageType_MsgHeartbeat' が候補に渡され、メッセージの term が候補より大きい場合、候補は follower に戻り、このハートビート中のものからコミットしたインデックスを更新する。
	そして、そのメッセージを自分のメールボックスに送ります。MessageType_MsgHeartbeat' が follower の Step メソッドに渡され、メッセージの term が follower の term よりも大きい場合、follower は自分の leaderID をメッセージの ID で更新する。

	'MessageType_MsgHeartbeatResponse' is a response to 'MessageType_MsgHeartbeat'. When 'MessageType_MsgHeartbeatResponse'
	is passed to the leader's Step method, the leader knows which follower
	responded.

	MessageType_MsgHeartbeatResponse' は 'MessageType_MsgHeartbeat' に対する応答である。
	MessageType_MsgHeartbeatResponse'がリーダーのStepメソッドに渡されると、リーダーはどのフォロワーから応答があったかを知ることができる。

*/
package raft
