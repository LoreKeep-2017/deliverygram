package websock

import (
	"encoding/json"
	"time"
	"log"
	"container/list"
	"sync"
	"net/http"
	"github.com/gorilla/websocket"
	"app/store"
)


type SessionStore struct {
	rw sync.RWMutex

	// Support for long polling sessions: a list of sessions sorted by last access time.
	// Needed for cleaning abandoned sessions.
	lru      *list.List
	lifeTime time.Duration

	// All sessions indexed by session ID
	sessCache map[string]*Session
	sessPair map[string]string;
	needpair string;
}

func (ss *SessionStore) Create(conn interface{}, sid string) *Session {
	var s Session

	s.sid = sid

	log.Println("step4");
	switch c := conn.(type) {
	case *websocket.Conn:
		s.proto = WEBSOCK
		s.ws = c
	case http.ResponseWriter:
		s.proto = LPOLL
		// no need to store c for long polling, it changes with every request
	//case *ClusterNode:
	//	s.proto = RPC
	//	s.rpcnode = c
	default:
		s.proto = NONE
	}

	log.Println("step5");
	if s.proto != NONE {
		s.subs = make(map[string]*Subscription)
		s.send = make(chan []byte, 256)  // buffered
		s.stop = make(chan []byte, 1)    // Buffered by 1 just to make it non-blocking
		s.detach = make(chan string, 64) // buffered
	}
	log.Println("step6");
	s.lastTouched = time.Now()
	if s.sid == "" {
		//var seededRand *rand.Rand = rand.New(
			//rand.NewSource(time.Now().UnixNano()))

		//log.Println("step6.1");
		//log.Println(store.GetUidString())
		//log.Println(s)
		s.sid = store.GetUidString()
		log.Println(s.sid);
		//s.sid = strconv.Itoa(seededRand.Int());
	}
	log.Println("step6.2");

	ss.rw.Lock()
	log.Println(s);
	ss.sessCache[s.sid] = &s
	log.Println("step7");
	if s.proto == LPOLL {
		// Only LP sessions need to be sorted by last active
		s.lpTracker = ss.lru.PushFront(&s)

		// Remove expired sessions
		expire := s.lastTouched.Add(-ss.lifeTime)
		for elem := ss.lru.Back(); elem != nil; elem = ss.lru.Back() {
			sess := elem.Value.(*Session)
			if sess.lastTouched.Before(expire) {
				ss.lru.Remove(elem)
				delete(ss.sessCache, sess.sid)
				//globals.cluster.sessionGone(sess)
			} else {
				break // don't need to traverse further
			}
		}
	}
	log.Println("step8");
	ss.rw.Unlock()

	return &s
}

func (ss *SessionStore) Get(sid string) *Session {
	ss.rw.Lock()
	defer ss.rw.Unlock()

	if sess := ss.sessCache[sid]; sess != nil {
		if sess.proto == LPOLL {
			ss.lru.MoveToFront(sess.lpTracker)
			sess.lastTouched = time.Now()
		}

		return sess
	}

	return nil
}

func (ss *SessionStore) GetAll() map[string]*Session{
	return ss.sessCache
}

func (ss *SessionStore) GetNeedPair() string{
	return ss.needpair
}


func (ss *SessionStore) GetAllPair() map[string]string{
	return ss.sessPair
}

func (ss *SessionStore) Delete(s *Session) {
	ss.rw.Lock()
	defer ss.rw.Unlock()

	delete(ss.sessCache, s.sid)

	if s.proto == LPOLL {
		ss.lru.Remove(s.lpTracker)
	}
}

// Shutting down sessionStore. No need to clean up.
func (ss *SessionStore) Shutdown() {
	ss.rw.Lock()
	defer ss.rw.Unlock()

	shutdown, _ := json.Marshal(NoErrShutdown(time.Now().UTC().Round(time.Millisecond)))
	for _, s := range ss.sessCache {
		if s.send != nil {
			s.send <- shutdown
		}
	}

	log.Printf("SessionStore shut down, sessions terminated: %d", len(ss.sessCache))
}

func NewSessionStore(lifetime time.Duration) *SessionStore {
	store := &SessionStore{
		lru:      list.New(),
		lifeTime: lifetime,

		sessCache: make(map[string]*Session),
		sessPair: make(map[string]string),
	}

	return store
}


