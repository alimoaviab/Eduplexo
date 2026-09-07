// Package messaging implements the secure school communication system.
//
// Allowed conversations: Student↔Teacher, Student↔Admin, Teacher↔Admin
// NOT allowed: Student↔Student, Parent↔Parent
//
// Features:
//   - Real-time messaging via WebSocket
//   - Seen/delivered status
//   - Typing indicators
//   - Reply to messages
//   - Auto-delete after 7 days
//   - Anti-spam (rate limiting)
//   - File attachments (images, PDFs, docs)
//   - Emergency broadcasts
package messaging

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eduplexo/backend-go/internal/api"
	"github.com/eduplexo/backend-go/internal/cache"
	"github.com/eduplexo/backend-go/internal/domain/access"
	rt "github.com/eduplexo/backend-go/internal/realtime"
	"github.com/eduplexo/backend-go/internal/store"
	"github.com/go-chi/chi/v5"
)

const (
	messageExpiry       = 7 * 24 * time.Hour // 7 days
	studentRateLimit    = 10                 // messages per minute
	teacherRateLimit    = 30
	adminRateLimit      = 60
	maxMessageLength    = 2000
	maxAttachmentSizeMB = 10
)

// Handler serves the /api/messages/* routes.
type Handler struct {
	Store   *store.MemStore
	Persist func(table string, doc any)
	Cache   *cache.Client
	Hub     rt.Notifier

	// In-memory rate limiter: userID → []timestamp
	rateMu  sync.Mutex
	rateMap map[string][]time.Time
}

func New(s *store.MemStore, persist func(string, any), c *cache.Client, hub *rt.Hub) *Handler {
	return &Handler{
		Store:   s,
		Persist: persist,
		Cache:   c,
		Hub:     hub,
		rateMap: make(map[string][]time.Time),
	}
}

// ─── Conversations ───────────────────────────────────────────────────────

// ListConversations returns all conversations for the current user.
// GET /api/messages/conversations
func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)

	h.Store.RLock()
	defer h.Store.RUnlock()

	conversations := make([]map[string]any, 0)
	for _, conv := range h.Store.Conversations {
		if conv.SchoolID != ctx.SchoolID {
			continue
		}
		// Check if user is a participant
		isParticipant := false
		for _, p := range conv.Participants {
			if p.UserID == ctx.UserID {
				isParticipant = true
				break
			}
		}
		if !isParticipant {
			continue
		}

		// Get other participant info
		var otherUser *conversationUser
		for _, p := range conv.Participants {
			if p.UserID != ctx.UserID {
				otherUser = h.resolveUser(p.UserID)
				break
			}
		}

		// Count unread messages
		unread := 0
		var lastMsg *store.ChatMessage
		for _, msg := range h.Store.ChatMessages {
			if msg.ConversationID != conv.ID {
				continue
			}
			if lastMsg == nil || msg.CreatedAt.After(lastMsg.CreatedAt) {
				lastMsg = msg
			}
			if msg.SenderID != ctx.UserID && !msg.SeenAt.IsZero() == false && msg.SeenAt.IsZero() {
				unread++
			}
		}

		entry := map[string]any{
			"_id":          conv.ID,
			"type":         conv.Type,
			"created_at":   conv.CreatedAt,
			"unread_count": unread,
		}
		if otherUser != nil {
			entry["other_user"] = otherUser
		}
		if lastMsg != nil {
			entry["last_message"] = map[string]any{
				"text":       truncate(lastMsg.Text, 50),
				"sender_id":  lastMsg.SenderID,
				"created_at": lastMsg.CreatedAt,
			}
		}
		conversations = append(conversations, entry)
	}

	// Sort by last message time (most recent first)
	sort.SliceStable(conversations, func(i, j int) bool {
		iLast, _ := conversations[i]["last_message"].(map[string]any)
		jLast, _ := conversations[j]["last_message"].(map[string]any)
		if iLast == nil {
			return false
		}
		if jLast == nil {
			return true
		}
		iTime, _ := iLast["created_at"].(time.Time)
		jTime, _ := jLast["created_at"].(time.Time)
		return iTime.After(jTime)
	})

	api.WriteResult(w, api.Ok(conversations))
}

// CreateConversation starts a new conversation between two users.
// POST /api/messages/conversations
func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	var body struct {
		RecipientID string `json:"recipient_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}

	if body.RecipientID == "" {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "recipient_id is required.", 400, nil))
		return
	}

	// Validate the conversation is allowed
	recipientRole := h.getUserRole(body.RecipientID, ctx.SchoolID)
	if recipientRole == "" {
		api.WriteResult(w, api.Fail("NOT_FOUND", "Recipient not found.", 404, nil))
		return
	}

	if !isConversationAllowed(ctx.Role, recipientRole) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "This conversation type is not allowed.", 403, nil))
		return
	}

	// Check if conversation already exists
	h.Store.RLock()
	for _, conv := range h.Store.Conversations {
		if conv.SchoolID != ctx.SchoolID || conv.Type != "private" {
			continue
		}
		hasMe := false
		hasRecipient := false
		for _, p := range conv.Participants {
			if p.UserID == ctx.UserID {
				hasMe = true
			}
			if p.UserID == body.RecipientID {
				hasRecipient = true
			}
		}
		if hasMe && hasRecipient {
			h.Store.RUnlock()
			api.WriteResult(w, api.Ok(map[string]any{"_id": conv.ID, "existing": true}))
			return
		}
	}
	h.Store.RUnlock()

	// Create new conversation
	now := time.Now()
	conv := &store.Conversation{
		ID:       store.NewID("conv"),
		SchoolID: ctx.SchoolID,
		Type:     "private",
		Participants: []store.ConversationParticipant{
			{UserID: ctx.UserID, Role: ctx.Role, JoinedAt: now},
			{UserID: body.RecipientID, Role: recipientRole, JoinedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	h.Store.Lock()
	h.Store.Conversations = append(h.Store.Conversations, conv)
	h.Store.Unlock()
	h.Persist("conversations", conv)

	api.WriteResult(w, api.Ok(map[string]any{"_id": conv.ID, "existing": false}))
}

// ─── Messages ────────────────────────────────────────────────────────────

// ListMessages returns messages for a conversation.
// GET /api/messages/conversations/{id}/messages
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	convID := chi.URLParam(r, "id")

	// Verify user is participant
	if !h.isParticipant(ctx.UserID, convID, ctx.SchoolID) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "You are not part of this conversation.", 403, nil))
		return
	}

	h.Store.RLock()
	messages := make([]map[string]any, 0)
	now := time.Now()
	for _, msg := range h.Store.ChatMessages {
		if msg.ConversationID != convID {
			continue
		}
		// Skip expired messages
		if !msg.ExpiresAt.IsZero() && msg.ExpiresAt.Before(now) {
			continue
		}

		daysLeft := 0
		if !msg.ExpiresAt.IsZero() {
			daysLeft = int(time.Until(msg.ExpiresAt).Hours() / 24)
			if daysLeft < 0 {
				daysLeft = 0
			}
		}

		entry := map[string]any{
			"_id":             msg.ID,
			"sender_id":       msg.SenderID,
			"text":            msg.Text,
			"attachment_url":  msg.AttachmentURL,
			"attachment_type": msg.AttachmentType,
			"reply_to_id":     msg.ReplyToID,
			"is_seen":         !msg.SeenAt.IsZero(),
			"delivered_at":    msg.DeliveredAt,
			"seen_at":         msg.SeenAt,
			"expires_in_days": daysLeft,
			"created_at":      msg.CreatedAt,
		}
		messages = append(messages, entry)
	}
	h.Store.RUnlock()

	// Sort oldest first
	sort.SliceStable(messages, func(i, j int) bool {
		iTime, _ := messages[i]["created_at"].(time.Time)
		jTime, _ := messages[j]["created_at"].(time.Time)
		return iTime.Before(jTime)
	})

	api.WriteResult(w, api.Ok(messages))
}

// SendMessage sends a new message in a conversation.
// POST /api/messages/conversations/{id}/messages
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	convID := chi.URLParam(r, "id")

	var body struct {
		Text           string `json:"text"`
		AttachmentURL  string `json:"attachment_url"`
		AttachmentType string `json:"attachment_type"`
		ReplyToID      string `json:"reply_to_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}

	if strings.TrimSpace(body.Text) == "" && body.AttachmentURL == "" {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Message text or attachment is required.", 400, nil))
		return
	}

	if len(body.Text) > maxMessageLength {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Message too long (max 2000 characters).", 400, nil))
		return
	}

	// Rate limiting
	if !h.checkRateLimit(ctx.UserID, ctx.Role) {
		api.WriteResult(w, api.Fail("RATE_LIMITED", "You are sending messages too fast. Please wait a moment.", 429, nil))
		return
	}

	// Verify user is participant
	if !h.isParticipant(ctx.UserID, convID, ctx.SchoolID) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "You are not part of this conversation.", 403, nil))
		return
	}

	now := time.Now()
	msg := &store.ChatMessage{
		ID:             store.NewID("msg"),
		ConversationID: convID,
		SenderID:       ctx.UserID,
		Text:           strings.TrimSpace(body.Text),
		AttachmentURL:  body.AttachmentURL,
		AttachmentType: body.AttachmentType,
		ReplyToID:      body.ReplyToID,
		DeliveredAt:    now,
		ExpiresAt:      now.Add(messageExpiry),
		CreatedAt:      now,
	}

	h.Store.Lock()
	h.Store.ChatMessages = append(h.Store.ChatMessages, msg)
	h.Store.Unlock()
	h.Persist("chat_messages", msg)

	// Send real-time notification to recipient
	h.notifyRecipient(ctx, convID, msg)

	api.WriteResult(w, api.Ok(map[string]any{
		"_id":        msg.ID,
		"text":       msg.Text,
		"created_at": msg.CreatedAt,
		"expires_at": msg.ExpiresAt,
	}))
}

// MarkSeen marks messages as seen.
// POST /api/messages/conversations/{id}/seen
func (h *Handler) MarkSeen(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	convID := chi.URLParam(r, "id")

	if !h.isParticipant(ctx.UserID, convID, ctx.SchoolID) {
		api.WriteResult(w, api.Fail("FORBIDDEN", "You are not part of this conversation.", 403, nil))
		return
	}

	now := time.Now()
	h.Store.Lock()
	for _, msg := range h.Store.ChatMessages {
		if msg.ConversationID == convID && msg.SenderID != ctx.UserID && msg.SeenAt.IsZero() {
			msg.SeenAt = now
		}
	}
	h.Store.Unlock()

	// Notify sender that messages were seen
	h.Store.RLock()
	for _, conv := range h.Store.Conversations {
		if conv.ID == convID {
			for _, p := range conv.Participants {
				if p.UserID != ctx.UserID {
					h.Hub.SendToUser(ctx.SchoolID, p.UserID, rt.Message{
						Type:    "message_seen",
						Payload: map[string]any{"conversation_id": convID, "seen_by": ctx.UserID, "seen_at": now},
					})
					break
				}
			}
			break
		}
	}
	h.Store.RUnlock()

	api.WriteResult(w, api.Ok(map[string]any{"success": true}))
}

// ─── Typing Indicator ────────────────────────────────────────────────────

// Typing sends a typing indicator to the other participant.
// POST /api/messages/conversations/{id}/typing
func (h *Handler) Typing(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	convID := chi.URLParam(r, "id")

	var body struct {
		IsTyping bool `json:"is_typing"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	// Send typing event to other participant
	h.Store.RLock()
	for _, conv := range h.Store.Conversations {
		if conv.ID == convID && conv.SchoolID == ctx.SchoolID {
			for _, p := range conv.Participants {
				if p.UserID != ctx.UserID {
					eventType := "typing_start"
					if !body.IsTyping {
						eventType = "typing_stop"
					}
					h.Hub.SendToUser(ctx.SchoolID, p.UserID, rt.Message{
						Type:    eventType,
						Payload: map[string]any{"conversation_id": convID, "user_id": ctx.UserID},
					})
					break
				}
			}
			break
		}
	}
	h.Store.RUnlock()

	w.WriteHeader(http.StatusNoContent)
}

// ─── Contacts ────────────────────────────────────────────────────────────

// ListContacts returns users the current user can chat with.
// GET /api/messages/contacts
func (h *Handler) ListContacts(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)

	h.Store.RLock()
	defer h.Store.RUnlock()

	// Build class name lookup
	classNames := make(map[string]string)
	for _, c := range h.Store.Classes {
		if c.SchoolID == ctx.SchoolID {
			classNames[c.ID] = c.Name
		}
	}

	// Build student class/section lookup by UserID
	type studentInfo struct {
		ClassName string
		Section   string
	}
	studentClassMap := make(map[string]studentInfo)
	for _, s := range h.Store.Students {
		if s.SchoolID == ctx.SchoolID && s.UserID != "" {
			cn := classNames[s.ClassID]
			studentClassMap[s.UserID] = studentInfo{ClassName: cn, Section: s.Section}
		}
	}

	contacts := make([]map[string]any, 0)
	for _, u := range h.Store.Users {
		if u.SchoolID != ctx.SchoolID || u.ID == ctx.UserID {
			continue
		}
		if u.Status != "active" {
			continue
		}
		if !isConversationAllowed(ctx.Role, u.Role) {
			continue
		}

		entry := map[string]any{
			"_id":  u.ID,
			"name": strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName),
			"role": u.Role,
		}

		// Add class info for students who have user accounts
		if info, ok := studentClassMap[u.ID]; ok {
			entry["class_name"] = info.ClassName
			entry["section"] = info.Section
		}

		contacts = append(contacts, entry)
	}

	// Also add Student records (who don't have user accounts) as contacts
	if isConversationAllowed(ctx.Role, "student") {
		teacherClassIDs := map[string]bool{}
		if ctx.Role == "teacher" {
			teacherClassIDs = access.TeacherClassIDsLocked(h.Store, ctx)
		}

		for _, s := range h.Store.Students {
			if s.SchoolID != ctx.SchoolID || s.Status != "active" {
				continue
			}
			// Teachers only see students in their assigned classes
			if ctx.Role == "teacher" && !teacherClassIDs[s.ClassID] {
				continue
			}

			// If the student has a user account, they were already added above
			if s.UserID != "" {
				continue
			}

			entry := map[string]any{
				"_id":        s.ID,
				"name":       strings.TrimSpace(s.FirstName + " " + s.LastName),
				"role":       "student",
				"class_name": classNames[s.ClassID],
				"section":    s.Section,
			}
			contacts = append(contacts, entry)
		}
	}

	// Sort: teachers first, then students alphabetically
	sort.SliceStable(contacts, func(i, j int) bool {
		ri := contacts[i]["role"].(string)
		rj := contacts[j]["role"].(string)
		if ri != rj {
			order := map[string]int{"admin": 0, "teacher": 1, "student": 2}
			return order[ri] < order[rj]
		}
		ni := contacts[i]["name"].(string)
		nj := contacts[j]["name"].(string)
		return ni < nj
	})

	api.WriteResult(w, api.Ok(contacts))
}

// ─── Broadcast ───────────────────────────────────────────────────────────

// SendBroadcast sends a broadcast message (admin only).
// POST /api/messages/broadcast
func (h *Handler) SendBroadcast(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)
	if ctx.Role != "admin" && ctx.Role != "super_admin" {
		api.WriteResult(w, api.Fail("FORBIDDEN", "Only admins can send broadcasts.", 403, nil))
		return
	}

	var body struct {
		TargetGroup  string   `json:"target_group"` // "all", "all_teachers", "all_students", "selected"
		Message      string   `json:"message"`
		Type         string   `json:"type"`          // "text", "emergency", "notice"
		RecipientIDs []string `json:"recipient_ids"` // for "selected" target
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "Invalid JSON body.", 400, nil))
		return
	}

	if body.Message == "" || body.TargetGroup == "" {
		api.WriteResult(w, api.Fail("VALIDATION_ERROR", "message and target_group are required.", 400, nil))
		return
	}

	// Normalize target group names
	targetGroup := body.TargetGroup
	switch targetGroup {
	case "all_students":
		targetGroup = "students"
	case "all_teachers":
		targetGroup = "teachers"
	}

	now := time.Now()
	broadcast := &store.Broadcast{
		ID:          store.NewID("brd"),
		SchoolID:    ctx.SchoolID,
		SenderID:    ctx.UserID,
		TargetGroup: targetGroup,
		Message:     body.Message,
		Type:        body.Type,
		CreatedAt:   now,
	}

	h.Store.Lock()
	h.Store.Broadcasts = append(h.Store.Broadcasts, broadcast)
	h.Store.Unlock()
	h.Persist("broadcasts", broadcast)

	// Send real-time notification to all targets
	go h.deliverBroadcast(ctx.SchoolID, broadcast, body.RecipientIDs)

	api.WriteResult(w, api.Ok(map[string]any{
		"_id":     broadcast.ID,
		"message": "Broadcast sent successfully.",
	}))
}

// ListBroadcasts returns broadcasts for the current user.
// GET /api/messages/broadcasts
func (h *Handler) ListBroadcasts(w http.ResponseWriter, r *http.Request) {
	ctx := api.FromRequest(r)

	h.Store.RLock()
	defer h.Store.RUnlock()

	results := make([]map[string]any, 0)
	for _, b := range h.Store.Broadcasts {
		if b.SchoolID != ctx.SchoolID {
			continue
		}
		if !h.isBroadcastTarget(ctx, b) {
			continue
		}
		results = append(results, map[string]any{
			"_id":          b.ID,
			"sender_id":    b.SenderID,
			"target_group": b.TargetGroup,
			"message":      b.Message,
			"type":         b.Type,
			"created_at":   b.CreatedAt,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		iTime, _ := results[i]["created_at"].(time.Time)
		jTime, _ := results[j]["created_at"].(time.Time)
		return iTime.After(jTime)
	})

	api.WriteResult(w, api.Ok(results))
}

// ─── Cleanup ─────────────────────────────────────────────────────────────

// CleanupExpired removes messages older than 7 days. Called by a cron/ticker.
func (h *Handler) CleanupExpired() int {
	now := time.Now()
	removed := 0

	h.Store.Lock()
	filtered := make([]*store.ChatMessage, 0, len(h.Store.ChatMessages))
	for _, msg := range h.Store.ChatMessages {
		if !msg.ExpiresAt.IsZero() && msg.ExpiresAt.Before(now) {
			removed++
			continue
		}
		filtered = append(filtered, msg)
	}
	h.Store.ChatMessages = filtered
	h.Store.Unlock()

	return removed
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func isConversationAllowed(role1, role2 string) bool {
	if role1 == "super_admin" || role2 == "super_admin" || role1 == "admin" || role2 == "admin" {
		return true
	}

	// Allowed: student↔teacher, teacher↔teacher, parent↔teacher
	// NOT allowed: student↔student, parent↔parent
	pair := role1 + ":" + role2
	allowed := map[string]bool{
		"student:teacher": true, "teacher:student": true,
		"teacher:teacher": true,
		"parent:teacher":  true, "teacher:parent": true,
	}
	return allowed[pair]
}

func (h *Handler) getUserRole(userID, schoolID string) string {
	h.Store.RLock()
	defer h.Store.RUnlock()
	for _, u := range h.Store.Users {
		if u.ID == userID && u.SchoolID == schoolID {
			return u.Role
		}
	}
	// Fallback: check if it's a Student record without a User account
	for _, s := range h.Store.Students {
		if s.ID == userID && s.SchoolID == schoolID {
			return "student"
		}
	}
	return ""
}

func (h *Handler) isParticipant(userID, convID, schoolID string) bool {
	h.Store.RLock()
	defer h.Store.RUnlock()
	for _, conv := range h.Store.Conversations {
		if conv.ID == convID && conv.SchoolID == schoolID {
			for _, p := range conv.Participants {
				if p.UserID == userID {
					return true
				}
			}
			return false
		}
	}
	return false
}

type conversationUser struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

func (h *Handler) resolveUser(userID string) *conversationUser {
	for _, u := range h.Store.Users {
		if u.ID == userID {
			return &conversationUser{
				ID:   u.ID,
				Name: strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName),
				Role: u.Role,
			}
		}
	}
	for _, s := range h.Store.Students {
		if s.ID == userID {
			return &conversationUser{
				ID:   s.ID,
				Name: strings.TrimSpace(s.FirstName + " " + s.LastName),
				Role: "student",
			}
		}
	}
	return nil
}

func (h *Handler) checkRateLimit(userID, role string) bool {
	limit := studentRateLimit
	if role == "teacher" {
		limit = teacherRateLimit
	} else if role == "admin" {
		limit = adminRateLimit
	}

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	h.rateMu.Lock()
	defer h.rateMu.Unlock()

	// Clean old entries
	timestamps := h.rateMap[userID]
	filtered := make([]time.Time, 0)
	for _, t := range timestamps {
		if t.After(windowStart) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= limit {
		h.rateMap[userID] = filtered
		return false
	}

	h.rateMap[userID] = append(filtered, now)
	return true
}

func (h *Handler) notifyRecipient(ctx *api.RequestContext, convID string, msg *store.ChatMessage) {
	h.Store.RLock()
	defer h.Store.RUnlock()

	for _, conv := range h.Store.Conversations {
		if conv.ID == convID {
			for _, p := range conv.Participants {
				if p.UserID != ctx.UserID {
					// Get sender name
					senderName := ""
					for _, u := range h.Store.Users {
						if u.ID == ctx.UserID {
							senderName = strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName)
							break
						}
					}

					h.Hub.SendToUser(ctx.SchoolID, p.UserID, rt.Message{
						Type: "message_receive",
						Payload: map[string]any{
							"conversation_id": convID,
							"message_id":      msg.ID,
							"sender_id":       msg.SenderID,
							"sender_name":     senderName,
							"text":            truncate(msg.Text, 100),
							"created_at":      msg.CreatedAt,
						},
					})
					break
				}
			}
			break
		}
	}
}

func (h *Handler) deliverBroadcast(schoolID string, b *store.Broadcast, recipientIDs []string) {
	// NOTE: No school-wide realtime push happens here. Broadcasts are delivered
	// per-recipient below (as private conversations + message_receive), so a
	// message targeted at teachers/a class/specific users can never leak to
	// every connected member of the school through the WebSocket fan-out.
	// Non-targeted users still see broadcasts via GET /api/messages/broadcasts,
	// which filters with isBroadcastTarget.

	// Get sender name
	h.Store.RLock()
	senderName := "Admin"
	for _, u := range h.Store.Users {
		if u.ID == b.SenderID {
			senderName = strings.TrimSpace(u.Profile.FirstName + " " + u.Profile.LastName)
			if senderName == "" {
				senderName = "Admin"
			}
			break
		}
	}

	type targetUser struct {
		ID   string
		Role string
	}
	var targets []targetUser

	if b.TargetGroup == "selected" {
		idMap := make(map[string]bool)
		for _, rid := range recipientIDs {
			idMap[rid] = true
		}
		for _, u := range h.Store.Users {
			if u.SchoolID == schoolID && u.ID != b.SenderID && idMap[u.ID] {
				targets = append(targets, targetUser{ID: u.ID, Role: u.Role})
			}
		}
	} else {
		for _, u := range h.Store.Users {
			if u.SchoolID != schoolID || u.ID == b.SenderID {
				continue
			}
			match := false
			switch b.TargetGroup {
			case "all":
				match = true
			case "teachers":
				match = (u.Role == "teacher")
			case "students":
				match = (u.Role == "student")
			case "parents":
				match = (u.Role == "parent")
			default:
				if strings.HasPrefix(b.TargetGroup, "class:") {
					classID := strings.TrimPrefix(b.TargetGroup, "class:")
					for _, s := range h.Store.Students {
						if s.UserID == u.ID && s.ClassID == classID {
							match = true
							break
						}
					}
				}
			}
			if match {
				targets = append(targets, targetUser{ID: u.ID, Role: u.Role})
			}
		}
	}
	h.Store.RUnlock()

	// Deliver to each target user
	for _, target := range targets {
		var convID string

		// 1. Try to find existing conversation
		h.Store.RLock()
		for _, conv := range h.Store.Conversations {
			if conv.SchoolID != schoolID || conv.Type != "private" {
				continue
			}
			hasSender := false
			hasRecipient := false
			for _, p := range conv.Participants {
				if p.UserID == b.SenderID {
					hasSender = true
				}
				if p.UserID == target.ID {
					hasRecipient = true
				}
			}
			if hasSender && hasRecipient {
				convID = conv.ID
				break
			}
		}
		h.Store.RUnlock()

		// 2. Create conversation if it doesn't exist
		if convID == "" {
			h.Store.Lock()
			// Double check under write lock
			for _, conv := range h.Store.Conversations {
				if conv.SchoolID != schoolID || conv.Type != "private" {
					continue
				}
				hasSender := false
				hasRecipient := false
				for _, p := range conv.Participants {
					if p.UserID == b.SenderID {
						hasSender = true
					}
					if p.UserID == target.ID {
						hasRecipient = true
					}
				}
				if hasSender && hasRecipient {
					convID = conv.ID
					break
				}
			}

			if convID == "" {
				now := time.Now()
				conv := &store.Conversation{
					ID:       store.NewID("conv"),
					SchoolID: schoolID,
					Type:     "private",
					Participants: []store.ConversationParticipant{
						{UserID: b.SenderID, Role: "admin", JoinedAt: now},
						{UserID: target.ID, Role: target.Role, JoinedAt: now},
					},
					CreatedAt: now,
					UpdatedAt: now,
				}
				h.Store.Conversations = append(h.Store.Conversations, conv)
				convID = conv.ID
				h.Store.Unlock()
				h.Persist("conversations", conv)
			} else {
				h.Store.Unlock()
			}
		}

		// 3. Create and append ChatMessage
		now := time.Now()
		msg := &store.ChatMessage{
			ID:             store.NewID("msg"),
			ConversationID: convID,
			SenderID:       b.SenderID,
			Text:           b.Message,
			DeliveredAt:    now,
			ExpiresAt:      now.Add(messageExpiry),
			CreatedAt:      now,
		}

		h.Store.Lock()
		h.Store.ChatMessages = append(h.Store.ChatMessages, msg)
		for i, conv := range h.Store.Conversations {
			if conv.ID == convID {
				h.Store.Conversations[i].UpdatedAt = now
				break
			}
		}
		h.Store.Unlock()

		h.Persist("chat_messages", msg)

		// 4. Notify the recipient in real-time
		h.Hub.SendToUser(schoolID, target.ID, rt.Message{
			Type: "message_receive",
			Payload: map[string]any{
				"conversation_id": convID,
				"message_id":      msg.ID,
				"sender_id":       msg.SenderID,
				"sender_name":     senderName,
				"text":            msg.Text,
				"created_at":      msg.CreatedAt,
			},
		})
	}
}

func (h *Handler) isBroadcastTarget(ctx *api.RequestContext, b *store.Broadcast) bool {
	switch b.TargetGroup {
	case "all":
		return true
	case "teachers":
		return ctx.Role == "teacher" || ctx.Role == "admin"
	case "parents":
		return ctx.Role == "parent" || ctx.Role == "admin"
	case "students":
		return ctx.Role == "student" || ctx.Role == "admin"
	default:
		// class:{id} — check if user belongs to that class
		if strings.HasPrefix(b.TargetGroup, "class:") {
			return ctx.Role == "admin" // simplified; expand as needed
		}
		return ctx.Role == "admin"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
