package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// SFU terminates WebRTC at the Go node and forwards Opus RTP between channel members.
type SFU struct {
	api   *webrtc.API
	mu    sync.Mutex
	rooms map[string]*voiceRoom
	peers map[string]*voiceMember
}

type voiceRoom struct {
	channelID string
	mu        sync.Mutex
	members   map[string]*voiceMember
}

type voiceMember struct {
	userID    string
	channelID string
	client    *Client
	pc        *webrtc.PeerConnection

	mu            sync.Mutex
	publishers    map[string]*publisherFwd // remote speakers playing on this client
	uplink        *webrtc.TrackRemote
	fanoutCancel  context.CancelFunc
	fanoutTracks  map[string]*webrtc.TrackLocalStaticRTP // targetUserID -> track for fan-out writes
	renegMu       sync.Mutex
	renegTimer    *time.Timer
}

type publisherFwd struct {
	sender *webrtc.RTPSender
	cancel context.CancelFunc
	track  *webrtc.TrackLocalStaticRTP
}

func newSFU() *SFU {
	me := &webrtc.MediaEngine{}
	if err := me.RegisterDefaultCodecs(); err != nil {
		slog.Error("sfu: codec registration failed", "err", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(me))
	return &SFU{
		api:   api,
		rooms: make(map[string]*voiceRoom),
		peers: make(map[string]*voiceMember),
	}
}

type sessionDesc struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

func parseSessionDesc(raw json.RawMessage) (webrtc.SessionDescription, error) {
	var sd sessionDesc
	if err := json.Unmarshal(raw, &sd); err != nil {
		return webrtc.SessionDescription{}, err
	}
	return webrtc.SessionDescription{
		Type: webrtc.NewSDPType(sd.Type),
		SDP:  sd.SDP,
	}, nil
}

func (s *SFU) roomFor(channelID string) *voiceRoom {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rooms[channelID]
	if !ok {
		r = &voiceRoom{channelID: channelID, members: make(map[string]*voiceMember)}
		s.rooms[channelID] = r
	}
	return r
}

func (s *SFU) tryRenegotiateOffer(c *Client, offer webrtc.SessionDescription) bool {
	s.mu.Lock()
	m := s.peers[c.UserID]
	s.mu.Unlock()
	if m == nil || m.pc == nil {
		return false
	}
	st := m.pc.ConnectionState()
	if st != webrtc.PeerConnectionStateConnected && st != webrtc.PeerConnectionStateConnecting {
		return false
	}
	if m.channelID != c.ChannelID {
		return false
	}
	if err := m.pc.SetRemoteDescription(offer); err != nil {
		slog.Debug("sfu: offer renegotiate SetRemoteDescription", "user", c.UserID, "err", err)
		return false
	}
	answer, err := m.pc.CreateAnswer(nil)
	if err != nil {
		return false
	}
	if err := m.pc.SetLocalDescription(answer); err != nil {
		return false
	}
	payload, _ := json.Marshal(sessionDesc{Type: answer.Type.String(), SDP: answer.SDP})
	out, _ := json.Marshal(map[string]any{"payload": json.RawMessage(payload)})
	c.Send(Envelope{Event: "webrtc_answer", Data: out})
	slog.Info("sfu: offer renegotiated", "user", c.UserID)
	return true
}

func (s *SFU) HandleOffer(c *Client, data json.RawMessage) {
	var req struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &req); err != nil || len(req.Payload) == 0 {
		return
	}
	offer, err := parseSessionDesc(req.Payload)
	if err != nil || offer.Type != webrtc.SDPTypeOffer {
		slog.Warn("sfu: invalid offer", "user", c.UserID, "err", err)
		return
	}

	if s.tryRenegotiateOffer(c, offer) {
		return
	}

	s.RemovePeer(c.UserID)

	pc, err := s.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	})
	if err != nil {
		slog.Error("sfu: NewPeerConnection", "user", c.UserID, "err", err)
		return
	}

	member := &voiceMember{
		userID:       c.UserID,
		channelID:    c.ChannelID,
		client:       c,
		pc:           pc,
		publishers:   make(map[string]*publisherFwd),
		fanoutTracks: make(map[string]*webrtc.TrackLocalStaticRTP),
	}

	pc.OnICECandidate(func(cand *webrtc.ICECandidate) {
		if cand == nil {
			return
		}
		init := cand.ToJSON()
		payload, _ := json.Marshal(init)
		out, _ := json.Marshal(map[string]any{"payload": json.RawMessage(payload)})
		c.Send(Envelope{Event: "webrtc_ice", Data: out})
	})

	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		slog.Info("sfu: pc state", "user", c.UserID, "state", st.String())
		// Do not RemovePeer on transient "closed" during ICE/renegotiation — that drops the whole session.
		if st == webrtc.PeerConnectionStateFailed {
			slog.Warn("sfu: pc failed", "user", c.UserID)
		}
	})

	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if remote.Kind() != webrtc.RTPCodecTypeAudio {
			return
		}
		slog.Info("sfu: uplink track", "user", c.UserID, "channel", c.ChannelID)
		s.startPublisher(member, remote)
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		slog.Error("sfu: SetRemoteDescription", "user", c.UserID, "err", err)
		_ = pc.Close()
		return
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		slog.Error("sfu: CreateAnswer", "user", c.UserID, "err", err)
		_ = pc.Close()
		return
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		slog.Error("sfu: SetLocalDescription", "user", c.UserID, "err", err)
		_ = pc.Close()
		return
	}

	s.mu.Lock()
	s.peers[c.UserID] = member
	s.mu.Unlock()

	room := s.roomFor(c.ChannelID)
	room.mu.Lock()
	room.members[c.UserID] = member
	for uid, other := range room.members {
		if uid == c.UserID {
			continue
		}
		other.mu.Lock()
		hasUplink := other.uplink != nil
		other.mu.Unlock()
		if hasUplink {
			s.wireSubscriber(other, member)
		}
	}
	room.mu.Unlock()

	payload, _ := json.Marshal(sessionDesc{Type: answer.Type.String(), SDP: answer.SDP})
	out, _ := json.Marshal(map[string]any{"payload": json.RawMessage(payload)})
	c.Send(Envelope{Event: "webrtc_answer", Data: out})
}

func (s *SFU) startPublisher(source *voiceMember, remote *webrtc.TrackRemote) {
	source.mu.Lock()
	if source.fanoutCancel != nil {
		source.fanoutCancel()
	}
	source.uplink = remote
	ctx, cancel := context.WithCancel(context.Background())
	source.fanoutCancel = cancel
	source.mu.Unlock()

	room := s.roomFor(source.channelID)
	room.mu.Lock()
	for uid, sub := range room.members {
		if uid == source.userID {
			continue
		}
		s.wireSubscriber(source, sub)
	}
	room.mu.Unlock()

	go s.fanoutLoop(ctx, source)
}

func (s *SFU) wireSubscriber(source, target *voiceMember) {
	target.mu.Lock()
	if _, ok := target.publishers[source.userID]; ok {
		target.mu.Unlock()
		return
	}

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		source.userID,
	)
	if err != nil {
		target.mu.Unlock()
		slog.Error("sfu: NewTrackLocalStaticRTP", "err", err)
		return
	}

	sender, err := target.pc.AddTrack(track)
	if err != nil {
		target.mu.Unlock()
		slog.Error("sfu: AddTrack", "to", target.userID, "from", source.userID, "err", err)
		return
	}

	_, cancel := context.WithCancel(context.Background())
	target.publishers[source.userID] = &publisherFwd{sender: sender, cancel: cancel, track: track}
	target.mu.Unlock()

	source.mu.Lock()
	source.fanoutTracks[target.userID] = track
	source.mu.Unlock()

	s.scheduleRenegotiate(target)
}

func (s *SFU) scheduleRenegotiate(m *voiceMember) {
	m.renegMu.Lock()
	if m.renegTimer != nil {
		m.renegTimer.Stop()
	}
	m.renegTimer = time.AfterFunc(200*time.Millisecond, func() {
		s.renegotiate(m)
	})
	m.renegMu.Unlock()
}

func (s *SFU) fanoutLoop(ctx context.Context, source *voiceMember) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		source.mu.Lock()
		remote := source.uplink
		tracks := make([]*webrtc.TrackLocalStaticRTP, 0, len(source.fanoutTracks))
		for _, t := range source.fanoutTracks {
			tracks = append(tracks, t)
		}
		source.mu.Unlock()

		if remote == nil {
			return
		}

		pkt, _, err := remote.ReadRTP()
		if err != nil {
			if err != io.EOF {
				slog.Debug("sfu: ReadRTP ended", "user", source.userID, "err", err)
			}
			return
		}

		// NoNoise / server-side DSP hooks pkt.Payload before distribution.
		for _, out := range tracks {
			cloned := cloneRTP(pkt)
			if cloned == nil {
				continue
			}
			if writeErr := out.WriteRTP(cloned); writeErr != nil {
				slog.Debug("sfu: WriteRTP", "from", source.userID, "err", writeErr)
			}
		}
	}
}

func cloneRTP(pkt *rtp.Packet) *rtp.Packet {
	if pkt == nil {
		return nil
	}
	raw, err := pkt.Marshal()
	if err != nil {
		return nil
	}
	dup := &rtp.Packet{}
	if err := dup.Unmarshal(raw); err != nil {
		return nil
	}
	return dup
}

func (s *SFU) renegotiate(m *voiceMember) {
	m.mu.Lock()
	pc := m.pc
	client := m.client
	m.mu.Unlock()
	if pc == nil || client == nil {
		return
	}
	if pc.ConnectionState() == webrtc.PeerConnectionStateClosed {
		return
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		slog.Error("sfu: renegotiate offer", "user", m.userID, "err", err)
		return
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		return
	}
	payload, _ := json.Marshal(sessionDesc{Type: offer.Type.String(), SDP: offer.SDP})
	out, _ := json.Marshal(map[string]any{"renegotiate": true, "payload": json.RawMessage(payload)})
	client.Send(Envelope{Event: "webrtc_offer", Data: out})
}

func (s *SFU) HandleAnswer(c *Client, data json.RawMessage) {
	var req struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	answer, err := parseSessionDesc(req.Payload)
	if err != nil {
		return
	}
	s.mu.Lock()
	m := s.peers[c.UserID]
	s.mu.Unlock()
	if m == nil || m.pc == nil {
		return
	}
	if err := m.pc.SetRemoteDescription(answer); err != nil {
		slog.Warn("sfu: SetRemoteDescription answer", "user", c.UserID, "err", err)
	}
}

func (s *SFU) HandleICE(c *Client, data json.RawMessage) {
	var req struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	var init webrtc.ICECandidateInit
	if err := json.Unmarshal(req.Payload, &init); err != nil || init.Candidate == "" {
		return
	}
	s.mu.Lock()
	m := s.peers[c.UserID]
	s.mu.Unlock()
	if m == nil || m.pc == nil {
		return
	}
	if err := m.pc.AddICECandidate(init); err != nil {
		slog.Debug("sfu: AddICECandidate", "user", c.UserID, "err", err)
	}
}

func (s *SFU) RemovePeer(userID string) {
	s.mu.Lock()
	m, ok := s.peers[userID]
	if ok {
		delete(s.peers, userID)
	}
	s.mu.Unlock()
	if !ok || m == nil {
		return
	}

	if m.fanoutCancel != nil {
		m.fanoutCancel()
	}
	if m.pc != nil {
		_ = m.pc.Close()
	}

	m.mu.Lock()
	for _, fwd := range m.publishers {
		fwd.cancel()
	}
	m.mu.Unlock()

	if m.channelID == "" {
		return
	}

	room := s.roomFor(m.channelID)
	room.mu.Lock()
	delete(room.members, userID)

	for _, sub := range room.members {
		sub.mu.Lock()
		if fwd, exists := sub.publishers[userID]; exists {
			fwd.cancel()
			if fwd.sender != nil {
				_ = sub.pc.RemoveTrack(fwd.sender)
			}
			delete(sub.publishers, userID)
			sub.mu.Unlock()
			s.scheduleRenegotiate(sub)
		} else {
			sub.mu.Unlock()
		}

		other := sub
		other.mu.Lock()
		delete(other.fanoutTracks, userID)
		other.mu.Unlock()
	}

	if len(room.members) == 0 {
		s.mu.Lock()
		delete(s.rooms, m.channelID)
		s.mu.Unlock()
	}
	room.mu.Unlock()
}

func (s *SFU) OnChannelChange(userID string) {
	s.mu.Lock()
	m := s.peers[userID]
	s.mu.Unlock()
	if m == nil {
		return
	}
	// Update channel without tearing down WS; client sends a fresh offer.
	if m.fanoutCancel != nil {
		m.fanoutCancel()
	}
	m.mu.Lock()
	m.uplink = nil
	m.fanoutTracks = make(map[string]*webrtc.TrackLocalStaticRTP)
	for _, fwd := range m.publishers {
		fwd.cancel()
	}
	m.publishers = make(map[string]*publisherFwd)
	m.mu.Unlock()
}

func (s *SFU) removeFromRoom(channelID, userID string) {
	if channelID == "" {
		return
	}
	s.mu.Lock()
	r := s.rooms[channelID]
	s.mu.Unlock()
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.members, userID)
	empty := len(r.members) == 0
	r.mu.Unlock()
	if empty {
		s.mu.Lock()
		delete(s.rooms, channelID)
		s.mu.Unlock()
	}
}

func (s *SFU) joinRoom(member *voiceMember, channelID string) {
	room := s.roomFor(channelID)
	room.mu.Lock()
	room.members[member.userID] = member
	for uid, other := range room.members {
		if uid == member.userID {
			continue
		}
		other.mu.Lock()
		hasUplink := other.uplink != nil
		other.mu.Unlock()
		if hasUplink {
			s.wireSubscriber(other, member)
		}
		member.mu.Lock()
		selfUplink := member.uplink != nil
		member.mu.Unlock()
		if selfUplink {
			s.wireSubscriber(member, other)
		}
	}
	room.mu.Unlock()
}

func (s *SFU) UpdateMemberChannel(userID, channelID string) {
	s.mu.Lock()
	m := s.peers[userID]
	s.mu.Unlock()
	if m == nil {
		return
	}
	oldCh := m.channelID
	if oldCh == channelID {
		return
	}
	m.channelID = channelID
	if oldCh != "" {
		s.removeFromRoom(oldCh, userID)
	}
	if channelID != "" {
		s.joinRoom(m, channelID)
	}
}
