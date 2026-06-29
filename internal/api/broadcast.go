package api

import "encoding/json"

// streamMessage is the envelope for everything pushed over the SSE stream, so a
// single connection can carry different update kinds and clients can branch on
// "kind". Exactly one payload field is set per message.
type streamMessage struct {
	Kind    string           `json:"kind"` // "event" | "comment"
	Event   *eventResponse   `json:"event,omitempty"`
	Comment *commentResponse `json:"comment,omitempty"`
	// EventID ties a comment back to the event it belongs to, so the timeline
	// can route it to the right thread.
	EventID string `json:"event_id,omitempty"`
}

// broadcastEvent pushes a newly created event to the org's stream subscribers,
// scoped to its environment.
func broadcastEvent(hub *Hub, orgID, envID string, ev eventResponse) {
	if data, err := json.Marshal(streamMessage{Kind: "event", Event: &ev}); err == nil {
		hub.Broadcast(orgID, envID, data)
	}
}

// broadcastComment pushes a newly created comment to the org's stream
// subscribers, scoped to the environment of the event it's attached to.
func broadcastComment(hub *Hub, orgID, envID, eventID string, cm commentResponse) {
	if data, err := json.Marshal(streamMessage{Kind: "comment", Comment: &cm, EventID: eventID}); err == nil {
		hub.Broadcast(orgID, envID, data)
	}
}
