package voice

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/gateway"
	"github.com/ayn2op/arikawa/v3/utils/handler"
	"github.com/ayn2op/arikawa/v3/utils/ws"
	"github.com/ayn2op/arikawa/v3/voice/udp"
	"github.com/ayn2op/arikawa/v3/voice/voicegateway"
	"github.com/gorilla/websocket"
)

const (
	fakeGuildID discord.GuildID = 1
	fakeUserID  discord.UserID  = 2
)

// fakeDiscord is a MainSession whose voice state updates Discord answers with
// the voice server, as it does when a voice channel is joined.
type fakeDiscord struct {
	events *handler.Handler
	server *fakeVoiceServer
}

func (d *fakeDiscord) Me() (*discord.User, error) {
	return &discord.User{ID: fakeUserID}, nil
}

func (d *fakeDiscord) Channel(id discord.ChannelID) (*discord.Channel, error) {
	return &discord.Channel{ID: id, GuildID: fakeGuildID}, nil
}

func (d *fakeDiscord) SendGateway(_ context.Context, event ws.Event) error {
	update, ok := event.(*gateway.UpdateVoiceStateCommand)
	if !ok || !update.ChannelID.IsValid() {
		return nil
	}

	// Discord answers after the command is sent, which JoinChannel waits for.
	go func() {
		d.events.Call(&gateway.VoiceStateUpdateEvent{VoiceState: discord.VoiceState{
			GuildID:   update.GuildID,
			ChannelID: update.ChannelID,
			UserID:    fakeUserID,
			SessionID: "session",
		}})
		d.moveVoiceServer()
	}()
	return nil
}

// moveVoiceServer tells the session that its voice server has changed, as
// Discord does when the voice channel is moved to another one.
func (d *fakeDiscord) moveVoiceServer() {
	d.events.Call(&gateway.VoiceServerUpdateEvent{
		GuildID:  fakeGuildID,
		Token:    "token",
		Endpoint: d.server.endpoint(),
	})
}

// fakeVoiceServer is a voice server that connects every client that
// identifies, each with a new SSRC, without encrypting end to end.
type fakeVoiceServer struct {
	gateway *httptest.Server
	udp     net.PacketConn
	// ssrc is the SSRC of the newest connection.
	ssrc atomic.Uint32

	speaking chan voicegateway.SpeakingEvent
	errs     chan error
	closed   chan struct{}
}

// newFakeVoiceServer starts a fakeVoiceServer that is closed when the test
// ends. It is only trusted on Linux, where the test is skipped otherwise.
func newFakeVoiceServer(t *testing.T) *fakeVoiceServer {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("the voice gateway's certificate can only be trusted through SSL_CERT_FILE on Linux")
	}

	packets, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	server := &fakeVoiceServer{
		udp:      packets,
		speaking: make(chan voicegateway.SpeakingEvent),
		errs:     make(chan error, 1),
		closed:   make(chan struct{}),
	}
	server.gateway = httptest.NewTLSServer(http.HandlerFunc(server.serveGateway))
	go server.serveUDP()

	t.Cleanup(func() {
		close(server.closed)
		server.gateway.Close()
		server.udp.Close()
	})

	trustCertificate(t, server.gateway.Certificate().Raw)
	return server
}

// trustCertificate makes TLS clients trust the DER-encoded certificate. It
// must be called before the system's certificates are first loaded.
func trustCertificate(t *testing.T, certificate []byte) {
	t.Helper()

	file := filepath.Join(t.TempDir(), "certificate.pem")
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})
	if err := os.WriteFile(file, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", file)
}

// endpoint is the voice server's endpoint, as Discord gives it.
func (s *fakeVoiceServer) endpoint() string {
	return strings.TrimPrefix(s.gateway.URL, "https://")
}

// nextSpeaking returns the next Speaking command that a client sends.
func (s *fakeVoiceServer) nextSpeaking(timeout time.Duration) (voicegateway.SpeakingEvent, error) {
	select {
	case event := <-s.speaking:
		return event, nil
	case err := <-s.errs:
		return voicegateway.SpeakingEvent{}, err
	case <-time.After(timeout):
		return voicegateway.SpeakingEvent{}, errors.New("timed out waiting for a Speaking command")
	}
}

// fail reports err to nextSpeaking, unless an error is already reported.
func (s *fakeVoiceServer) fail(err error) {
	select {
	case s.errs <- err:
	default:
	}
}

// ipDiscoveryLength is the length of both an IP discovery request and its
// response.
//
// https://docs.discord.food/topics/voice-connections#ip-discovery
const ipDiscoveryLength = 74

// serveUDP answers IP discovery requests with the address that they are sent
// from being the loopback address.
func (s *fakeVoiceServer) serveUDP() {
	request := make([]byte, 1500)
	for {
		n, addr, err := s.udp.ReadFrom(request)
		if err != nil {
			return
		}
		if n != ipDiscoveryLength {
			continue
		}

		response := make([]byte, ipDiscoveryLength)
		binary.BigEndian.PutUint16(response[0:2], 2) // response
		binary.BigEndian.PutUint16(response[2:4], ipDiscoveryLength-4)
		copy(response[4:8], request[4:8]) // SSRC
		copy(response[8:72], "127.0.0.1")
		binary.BigEndian.PutUint16(response[72:74], uint16(addr.(*net.UDPAddr).Port))

		if _, err := s.udp.WriteTo(response, addr); err != nil {
			s.fail(fmt.Errorf("failed to answer IP discovery: %w", err))
		}
	}
}

func (s *fakeVoiceServer) serveGateway(w http.ResponseWriter, r *http.Request) {
	conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
	if err != nil {
		s.fail(fmt.Errorf("failed to upgrade to a websocket: %w", err))
		return
	}
	defer conn.Close()

	if err := s.converse(conn); err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		s.fail(err)
	}
}

// converse answers what a client sends over conn until either closes it.
func (s *fakeVoiceServer) converse(conn *websocket.Conn) error {
	send := func(event ws.Event) error {
		return conn.WriteJSON(ws.Op{Code: event.Op(), Data: event})
	}

	if err := send(&voicegateway.HelloEvent{HeartbeatInterval: discord.Milliseconds(time.Minute / time.Millisecond)}); err != nil {
		return err
	}

	for {
		var op struct {
			Code ws.OpCode       `json:"op"`
			Data json.RawMessage `json:"d"`
		}
		if err := conn.ReadJSON(&op); err != nil {
			return err
		}

		newEvent := voicegateway.OpUnmarshalers.Lookup(op.Code, "")
		if newEvent == nil {
			return fmt.Errorf("unknown op %d", op.Code)
		}
		event := newEvent()
		if err := json.Unmarshal(op.Data, event); err != nil {
			return fmt.Errorf("failed to decode op %d: %w", op.Code, err)
		}

		var err error
		switch event := event.(type) {
		case *voicegateway.IdentifyCommand:
			err = send(&voicegateway.ReadyEvent{
				SSRC:  s.ssrc.Add(1),
				IP:    "127.0.0.1",
				Port:  s.udp.LocalAddr().(*net.UDPAddr).Port,
				Modes: udp.Modes(),
			})
		case *voicegateway.SelectProtocolCommand:
			err = send(&voicegateway.SessionDescriptionEvent{Mode: event.Data.Mode})
		case *voicegateway.HeartbeatCommand:
			err = send(&voicegateway.HeartbeatAckEvent{T: event.T})
		case *voicegateway.SpeakingEvent:
			select {
			case s.speaking <- *event:
			case <-s.closed:
			}
		}
		if err != nil {
			return err
		}
	}
}
