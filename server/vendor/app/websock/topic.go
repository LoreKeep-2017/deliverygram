package websock

import "time"


import (
	"github.com/tinode/chat/server/store/types"
	"github.com/tinode/chat/server/store"
)

const UA_TIMER_DELAY = time.Second * 5

// Maximum number of SeqIds to pass in a list
const MAX_SEQ_COUNT = 128
// Topic: an isolated communication channel
type Topic struct {
	// Еxpanded/unique name of the topic.
	name string
	// For single-user topics session-specific topic name, such as 'me', otherwise the same as 'name'.
	x_original string

	// Topic category
	cat types.TopicCat

	// TODO(gene): currently unused
	// If isProxy == true, the actual topic is hosted by another cluster member.
	// The topic should:
	// 1. forward all messages to master
	// 2. route replies from the master to sessions.
	// 3. disconnect sessions at master's request.
	// 4. shut down the topic at master's request.
	isProxy bool

	// Time when the topic was first created
	created time.Time
	// Time when the topic was last updated
	updated time.Time

	// Server-side ID of the last data message
	lastId int
	// If messages were hard-deleted, the ID of the last deleted meassage
	clearId int

	// Last published userAgent ('me' topic only)
	userAgent string

	// User ID of the topic owner/creator. Could be zero.
	owner types.Uid

	// Default access mode
	accessAuth types.AccessMode
	accessAnon types.AccessMode

	// Topic's public data
	public interface{}

	// Topic's per-subscriber data
	perUser map[types.Uid]perUserData
	// User's contact list (not nil for 'me' topic only).
	// The map keys are UserIds for P2P topics and grpXXX for group topics.
	perSubs map[string]perSubsData

	// Sessions attached to this topic
	sessions map[*Session]bool

	// Inbound {data} and {pres} messages from sessions or other topics, already converted to SCM. Buffered = 256
	broadcast chan *ServerComMessage

	// Channel for receiving {get}/{set} requests, buffered = 32
	meta chan *metaReq

	// Subscribe requests from sessions, buffered = 32
	reg chan *sessionJoin

	// Unsubscribe requests from sessions, buffered = 32
	unreg chan *sessionLeave

	// Track the most active sessions to report User Agent changes. Buffered = 32
	uaChange chan string

	// Channel to terminate topic  -- either the topic is deleted or system is being shut down. Buffered = 1.
	exit chan *shutDown
	// Flag which tells topic to stop acception requests: hub is in the process of shutting it down
	suspended atomicBool
}


type atomicBool int32

type sessionLeave struct {
	// Session which initiated the request
	sess *Session
	// Leave and unsubscribe
	unsub bool
	// Topic to report success of failure on
	topic string
	// ID of originating request, if any
	reqId string
}


// perUserData holds topic's cache of per-subscriber data
type perUserData struct {
	// Timestamps when the subscription was created and updated
	created time.Time
	updated time.Time

	online int

	// Last t.lastId reported by user through {pres} as received or read
	recvId int
	readId int
	// Greatest ID of a soft-deleted message
	clearId int

	private interface{}

	modeWant  types.AccessMode
	modeGiven types.AccessMode

	// P2P only:
	public interface{}
}

// perSubsData holds user's (on 'me' topic) cache of subscription data
type perSubsData struct {
	online bool
	// Uid of the other user for P2P topics, otherwise 0
	with types.Uid
}


type shutDown struct {
	// Channel to report back completion of topic shutdown. Could be nil
	done chan<- bool
	// Topic is being deleted as opposite to total system shutdown
	del bool
}

// Generate random string as a name of the group topic
func genTopicName() string {
	return "grp" + store.GetUidString()
}

