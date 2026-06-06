package ws

import (
	"encoding/json"
	"log/slog"
	"sync"
)

// PollManager holds in-memory poll votes per channel (prototype; resets on node restart).
type PollManager struct {
	mu   sync.RWMutex
	data map[string]map[string]*channelPoll // channelID -> pollID
}

type channelPoll struct {
	Opts      []pollOptCounts `json:"opts"`
	Tot       int             `json:"tot"`
	userVotes map[string]string
}

type pollOptCounts struct {
	ID string `json:"id"`
	V  int    `json:"v"`
}

func newPollManager() *PollManager {
	return &PollManager{data: make(map[string]map[string]*channelPoll)}
}

// Default poll definitions per channel (demo).
var defaultPollDefs = map[string]map[string][]string{
	"_default": {
		"p1": {"a", "b"},
		"p2": {"c", "d"},
	},
}

func (pm *PollManager) ensurePoll(channelID, pollID string) *channelPoll {
	if pm.data[channelID] == nil {
		pm.data[channelID] = make(map[string]*channelPoll)
	}
	if p, ok := pm.data[channelID][pollID]; ok {
		return p
	}
	defs := defaultPollDefs["_default"]
	optIDs, ok := defs[pollID]
	if !ok {
		return nil
	}
	p := &channelPoll{
		Opts:      make([]pollOptCounts, len(optIDs)),
		userVotes: make(map[string]string),
	}
	for i, id := range optIDs {
		p.Opts[i] = pollOptCounts{ID: id, V: 0}
	}
	pm.data[channelID][pollID] = p
	return p
}

func (pm *PollManager) Vote(channelID, userID, pollID, optionID string) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	p := pm.ensurePoll(channelID, pollID)
	if p == nil {
		return false
	}
	valid := false
	for _, o := range p.Opts {
		if o.ID == optionID {
			valid = true
			break
		}
	}
	if !valid {
		return false
	}
	if prev, ok := p.userVotes[userID]; ok {
		if prev == optionID {
			for i := range p.Opts {
				if p.Opts[i].ID == optionID {
					p.Opts[i].V--
					p.Tot--
					break
				}
			}
			delete(p.userVotes, userID)
			return true
		}
		for i := range p.Opts {
			if p.Opts[i].ID == prev {
				p.Opts[i].V--
				p.Tot--
				break
			}
		}
	}
	for i := range p.Opts {
		if p.Opts[i].ID == optionID {
			p.Opts[i].V++
			p.Tot++
			break
		}
	}
	p.userVotes[userID] = optionID
	return true
}

func (pm *PollManager) Snapshot(channelID string) map[string]channelPoll {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	ch := pm.data[channelID]
	out := make(map[string]channelPoll)
	for pid, optIDs := range defaultPollDefs["_default"] {
		if ch != nil {
			if p, ok := ch[pid]; ok {
				out[pid] = p.copy()
				continue
			}
		}
		opts := make([]pollOptCounts, len(optIDs))
		for i, id := range optIDs {
			opts[i] = pollOptCounts{ID: id, V: 0}
		}
		out[pid] = channelPoll{Opts: opts, Tot: 0}
	}
	return out
}

func (p *channelPoll) copy() channelPoll {
	opts := make([]pollOptCounts, len(p.Opts))
	copy(opts, p.Opts)
	return channelPoll{Opts: opts, Tot: p.Tot}
}

func (h *Hub) handlePollVote(c *Client, data json.RawMessage) {
	var req struct {
		ChannelID string `json:"channel_id"`
		PollID    string `json:"poll_id"`
		OptionID  string `json:"option_id"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return
	}
	if req.ChannelID == "" {
		req.ChannelID = c.ChannelID
	}
	if req.ChannelID == "" || req.PollID == "" || req.OptionID == "" {
		return
	}
	if !h.polls.Vote(req.ChannelID, c.UserID, req.PollID, req.OptionID) {
		slog.Debug("poll: invalid vote", "user", c.UserID, "poll", req.PollID)
		return
	}
	h.broadcastPollUpdate(req.ChannelID)
}

func (h *Hub) broadcastPollUpdate(channelID string) {
	snap := h.polls.Snapshot(channelID)
	payload := map[string]any{
		"channel_id": channelID,
		"polls":      snap,
	}
	data, _ := json.Marshal(payload)
	env := Envelope{Event: "poll_update", Data: data}
	h.mu.RLock()
	for _, cl := range h.clients {
		if cl.ChannelID == channelID {
			cl.Send(env)
		}
	}
	h.mu.RUnlock()
}
