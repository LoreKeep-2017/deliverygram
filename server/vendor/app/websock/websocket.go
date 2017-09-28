package websock


import (
	"net/http"
	"log"
	"github.com/gorilla/websocket"
	"time"
)


const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = time.Second * 55

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 1 << 18 // 256K
)

//var globals struct {
//	hub           *Hub
//	sessionStore  *SessionStore
//	//cluster       *Cluster
//	apiKeySalt    []byte
//	indexableTags []string
//	// Add Strict-Transport-Security to headers, the value signifies age.
//	// Empty string "" turns it off
//	tlsStrictMaxAge string
//}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow connections from any Origin
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Session) closeWS() {
	if s.proto == WEBSOCK {
		s.ws.Close()
	}
}

func (sess *Session) readLoop() {
	defer func() {
		log.Println("serveWebsocket - stop")
		sess.closeWS()
		globals.sessionStore.Delete(sess)
		//globals.cluster.sessionGone(sess)
		for _, sub := range sess.subs {
			// sub.done is the same as topic.unreg
			sub.done <- &sessionLeave{sess: sess, unsub: false}
		}
	}()

	sess.ws.SetReadLimit(maxMessageSize)
	sess.ws.SetReadDeadline(time.Now().Add(pongWait))
	sess.ws.SetPongHandler(func(string) error {
		sess.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	sess.remoteAddr = sess.ws.RemoteAddr().String()

	for {
		// Read a ClientComMessage
		if _, raw, err := sess.ws.ReadMessage(); err != nil {
			log.Println("sess.readLoop: " + err.Error())
			return
		} else {
			sess.dispatchRaw(raw)
		}
	}
}

func (sess *Session) writeLoop() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		sess.closeWS() // break readLoop
	}()

	//log.Println("{session: \""+sess.sid + "\"}")
	//msgsess := "{sid: \""+sess.sid + "\"}"

	sessions := globals.sessionStore.GetAll();

	if(globals.sessionStore.needpair != ""){
		globals.sessionStore.sessPair[globals.sessionStore.needpair] = sess.sid;
		globals.sessionStore.sessPair[sess.sid] = globals.sessionStore.needpair;

		wsess := sessions[globals.sessionStore.needpair]
		log.Println(wsess)

		if err := ws_write(wsess.ws, websocket.TextMessage, []byte(sess.sid)); err != nil {
			log.Println("sess.writeLoop: " + err.Error())
			return
		}

		if err := ws_write(sess.ws, websocket.TextMessage, []byte(wsess.sid)); err != nil {
			log.Println("sess.writeLoop: " + err.Error())
			return
		}
		globals.sessionStore.needpair = ""

	}else{
		globals.sessionStore.needpair = sess.sid;
	}


	keys := make([]string, 0, len(sessions))
	for k := range sessions {
		keys = append(keys, k)
	}

	for {
		select {
		case msg, ok := <-sess.send:
			log.Println("first")
			if !ok {
				// channel closed
				return
			}
			if err := ws_write(sess.ws, websocket.TextMessage, msg); err != nil {
				log.Println("sess.writeLoop: " + err.Error())
				return
			}
		case msg := <-sess.stop:
			log.Println("second")
			// Shutdown requested, don't care if the message is delivered
			if msg != nil {
				ws_write(sess.ws, websocket.TextMessage, msg)
			}
			return

		case topic := <-sess.detach:
			log.Println("third")
			delete(sess.subs, topic)

		case <-ticker.C:
			log.Println("fourth")
			if err := ws_write(sess.ws, websocket.PingMessage, []byte{}); err != nil {
				log.Println("sess.writeLoop: ping/" + err.Error())
				return
			}
		}
	}
}

// Writes a message with the given message type (mt) and payload.
func ws_write(ws *websocket.Conn, mt int, payload []byte) error {
	ws.SetWriteDeadline(time.Now().Add(writeWait))
	return ws.WriteMessage(mt, payload)
}


func ServerWebSocket(wrt http.ResponseWriter, req *http.Request){

	log.Println("Entered");

	if req.Method != "GET" {
		http.Error(wrt, "Method not allowed", http.StatusMethodNotAllowed)
		log.Println("ws: Invalid HTTP method")
		return
	}
	ws, err := upgrader.Upgrade(wrt, req, nil)
	if _, ok := err.(websocket.HandshakeError); ok {
		log.Println("ws: Not a websocket handshake")
		return
	} else if err != nil {
		log.Println("ws: failed to Upgrade ", err.Error())
		return
	}

	sess := globals.sessionStore.Create(ws, "")
	log.Println("step_ex");
	go sess.writeLoop()
	sess.readLoop()
}
