// Package audit provides event-driven audit logging for domain actions.
package audit

// auditPayload is the local representation of user event data for audit logging.
// The JSON field names are the contract shared with the user module's published events.
type auditPayload struct {
	UserID    string `json:"user_id"`
	ActorID   string `json:"actor_id"`
	IPAddress string `json:"ip_address,omitempty"`
}

func (p auditPayload) userID() string    { return p.UserID }
func (p auditPayload) actorID() string   { return p.ActorID }
func (p auditPayload) ipAddress() string { return p.IPAddress }
