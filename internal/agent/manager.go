package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// ContextProvider returns context text to inject into a new agent conversation
// for the given itemID. Return "" to skip injection.
type ContextProvider func(itemID string) string

// SystemPromptProvider returns a system prompt to append to the default system
// prompt for the given itemID. Return "" to skip injection.
type SystemPromptProvider func(itemID string) string

// OnFirstUserMessageFunc is called when the first user message is sent to a new session.
// Receives the itemID (which is the worktree ID for workspace agents) and
// the user's message text.
type OnFirstUserMessageFunc func(itemID string, message string)

// OnTurnCompleteFunc is called when an agent finishes a turn (receives a result event).
// Receives the itemID (which is the worktree ID for workspace agents).
type OnTurnCompleteFunc func(itemID string)

// QuickReplyContextFunc returns workspace context to include in the quick reply
// generation prompt for the given itemID. Return "" to skip context injection.
type QuickReplyContextFunc func(itemID string) string

// DefaultMode controls the initial mode for new agent sessions.
type DefaultMode string

const (
	DefaultModeAct  DefaultMode = "act"
	DefaultModePlan DefaultMode = "plan"
)

// EffortLevel controls the thinking effort for new agent sessions.
type EffortLevel string

const (
	EffortLevelLow    EffortLevel = "low"
	EffortLevelMedium EffortLevel = "medium"
	EffortLevelHigh   EffortLevel = "high"
	EffortLevelMax    EffortLevel = "max"
)

// Manager manages agent sessions — one per worktree (keyed by itemID).
// It holds sessions in memory and provides pub/sub for session updates.
type Manager struct {
	mu                   sync.RWMutex
	sessions             map[string]*Session
	processes            map[string]*runningProcess
	store                *store // JSONL persistence (nil if no itemsDir)
	contextProvider      ContextProvider
	systemPromptProvider SystemPromptProvider
	onFirstUserMessage   OnFirstUserMessageFunc
	onTurnComplete       OnTurnCompleteFunc
	quickReplyContext    QuickReplyContextFunc
	defaultMode          DefaultMode
	defaultEffort        EffortLevel

	subMu       sync.Mutex
	subscribers map[string][]chan struct{}

	globalSubMu       sync.Mutex
	globalSubscribers []chan struct{}
}

// NewManager creates a new agent session manager.
// If itemsDir is non-empty, conversations are persisted to .beans/conversations/.
// permissionMode controls the default mode for new sessions ("act", "plan").
// If empty, defaults to "act".
func NewManager(itemsDir string, contextProvider ContextProvider, defaultMode ...DefaultMode) *Manager {
	mode := DefaultModeAct
	if len(defaultMode) > 0 && defaultMode[0] != "" {
		mode = defaultMode[0]
	}
	m := &Manager{
		sessions:        make(map[string]*Session),
		processes:       make(map[string]*runningProcess),
		subscribers:     make(map[string][]chan struct{}),
		contextProvider: contextProvider,
		defaultMode:     mode,
	}

	if itemsDir != "" {
		s, err := newStore(itemsDir)
		if err != nil {
			log.Printf("[agent] warning: conversation persistence disabled: %v", err)
		} else {
			m.store = s
		}
	}

	return m
}

// SetSystemPromptProvider registers a callback that returns a system prompt to
// append for new sessions. Must be called during initialization.
func (m *Manager) SetSystemPromptProvider(fn SystemPromptProvider) {
	m.systemPromptProvider = fn
}

// SetOnFirstUserMessage registers a callback that fires when the first user message
// is sent to a new session. Must be called during initialization, before any messages are sent.
func (m *Manager) SetOnFirstUserMessage(fn OnFirstUserMessageFunc) {
	m.onFirstUserMessage = fn
}

// SetOnTurnComplete registers a callback that fires when an agent finishes a turn.
// Used to update workspace activity timestamps for sidebar sorting.
func (m *Manager) SetOnTurnComplete(fn OnTurnCompleteFunc) {
	m.onTurnComplete = fn
}

// SetQuickReplyContext registers a callback that returns workspace context
// (PR status, git state, etc.) for inclusion in quick reply generation prompts.
func (m *Manager) SetQuickReplyContext(fn QuickReplyContextFunc) {
	m.quickReplyContext = fn
}

// GetSession returns a snapshot of the session for the given itemID, or nil.
// If no in-memory session exists but a persisted conversation is found, it is loaded.
func (m *Manager) GetSession(itemID string) *Session {
	m.mu.RLock()
	s, ok := m.sessions[itemID]
	m.mu.RUnlock()

	if !ok {
		// Try loading from disk
		if m.store == nil {
			return nil
		}
		msgs, sessionID, err := m.store.load(itemID)
		if err != nil || len(msgs) == 0 {
			return nil
		}
		// Materialize the session in memory
		m.mu.Lock()
		// Double-check another goroutine didn't create it
		if s2, ok2 := m.sessions[itemID]; ok2 {
			snap := s2.snapshot()
			m.mu.Unlock()
			return &snap
		}
		s = m.newBaseSession(itemID)
		s.Messages = msgs
		s.SessionID = sessionID
		m.sessions[itemID] = s
		m.mu.Unlock()
	}

	m.mu.RLock()
	snap := s.snapshot()
	m.mu.RUnlock()
	return &snap
}

// SendMessage sends a user message to the agent for the given worktree.
// If no session exists, one is created. If no process is running, one is spawned.
// Images are optional base64-decoded uploads that will be stored and forwarded to Claude.
func (m *Manager) SendMessage(itemID, workDir, message string, images []ImageUpload, attachmentPaths ...string) error {
	// Save images to disk before acquiring the lock
	var imageRefs []ImageRef
	if m.store != nil && len(images) > 0 {
		for _, img := range images {
			ref, err := m.store.saveImage(itemID, img.MediaType, img.Data)
			if err != nil {
				return fmt.Errorf("save image: %w", err)
			}
			imageRefs = append(imageRefs, ref)
		}
	}

	m.mu.Lock()

	// Get or create session
	session, ok := m.sessions[itemID]
	if !ok {
		session = m.loadOrCreateSession(itemID, workDir)
		m.sessions[itemID] = session
	}

	// Ensure WorkDir is set (may be empty if loaded from disk by GetSession)
	if session.WorkDir == "" && workDir != "" {
		session.WorkDir = workDir
	}

	// Build context prefix from attached file paths (injected into Claude's
	// stdin message but NOT stored in the conversation or shown in the UI)
	var contextPrefix string
	if len(attachmentPaths) > 0 {
		contextPrefix = fmt.Sprintf("[The user has attached the following files/directories for context: %s]\n\n",
			strings.Join(attachmentPaths, ", "))
	}

	// Append user message and clear turn state
	userMsg := Message{Role: RoleUser, Content: message, Images: imageRefs, Attachments: attachmentPaths, ContextPrefix: contextPrefix}
	session.Messages = append(session.Messages, userMsg)
	session.Error = ""
	session.PendingInteraction = nil
	session.ToolInvocations = nil
	session.QuickReplies = nil

	// Check if this is the first user message in the session (for description
	// generation). We can't just check !ok because AddInfoMessage may have
	// pre-created the session (e.g. from workspace setup notifications).
	isFirstUserMessage := countUserMessages(session.Messages) == 1

	// Persist user message
	if m.store != nil {
		if err := m.store.appendMessage(itemID, userMsg); err != nil {
			log.Printf("[agent:%s] failed to persist user message: %v", itemID, err)
		}
	}

	// Check if we have a running process
	proc, hasProc := m.processes[itemID]
	session.Status = StatusRunning
	m.mu.Unlock()

	// Notify subscribers that we have a new user message + running status
	m.notify(itemID)

	// Fire onFirstUserMessage callback when this is the first user message
	if isFirstUserMessage && m.onFirstUserMessage != nil {
		go m.onFirstUserMessage(itemID, message)
	}

	if hasProc && proc != nil {
		// Send message to existing process via stdin — Claude Code's stream-json
		// protocol handles interleaving even if the agent is mid-turn
		return m.sendToProcess(proc, itemID, contextPrefix+message, imageRefs)
	}

	// Spawn a new process
	go m.spawnAndRun(itemID, session)
	return nil
}

// StopSession kills the running process for a session and sets it to idle.
func (m *Manager) StopSession(itemID string) error {
	m.mu.Lock()
	proc, hasProc := m.processes[itemID]
	session, hasSession := m.sessions[itemID]
	if hasSession {
		session.Status = StatusIdle
	}
	if hasProc {
		delete(m.processes, itemID)
	}
	m.mu.Unlock()

	if hasProc && proc != nil {
		proc.kill()
	}

	m.notify(itemID)
	return nil
}

// Subscribe returns a channel that receives a signal whenever the session
// for the given itemID changes. Call Unsubscribe when done.
func (m *Manager) Subscribe(itemID string) chan struct{} {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	ch := make(chan struct{}, 1)
	m.subscribers[itemID] = append(m.subscribers[itemID], ch)
	return ch
}

// Unsubscribe removes a subscription channel.
func (m *Manager) Unsubscribe(itemID string, ch chan struct{}) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	subs := m.subscribers[itemID]
	for i, sub := range subs {
		if sub == ch {
			m.subscribers[itemID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// SubscribeGlobal returns a channel that receives a signal whenever any
// agent session changes. Call UnsubscribeGlobal when done.
func (m *Manager) SubscribeGlobal() chan struct{} {
	m.globalSubMu.Lock()
	defer m.globalSubMu.Unlock()
	ch := make(chan struct{}, 1)
	m.globalSubscribers = append(m.globalSubscribers, ch)
	return ch
}

// UnsubscribeGlobal removes a global subscription channel.
func (m *Manager) UnsubscribeGlobal(ch chan struct{}) {
	m.globalSubMu.Lock()
	defer m.globalSubMu.Unlock()
	for i, sub := range m.globalSubscribers {
		if sub == ch {
			m.globalSubscribers = append(m.globalSubscribers[:i], m.globalSubscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// ListRunningSessions returns the item IDs and statuses of all in-memory sessions.
func (m *Manager) ListRunningSessions() []ActiveAgent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []ActiveAgent
	for id, s := range m.sessions {
		if s.Status == StatusRunning {
			result = append(result, ActiveAgent{ItemID: id, Status: s.Status})
		}
	}
	return result
}

// notify sends a signal to all subscribers for the given itemID,
// and also notifies global subscribers.
func (m *Manager) notify(itemID string) {
	m.subMu.Lock()
	for _, ch := range m.subscribers[itemID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	m.subMu.Unlock()

	m.globalSubMu.Lock()
	for _, ch := range m.globalSubscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	m.globalSubMu.Unlock()
}

// AddInfoMessage appends an informational message to a session's chat history.
// Info messages are visible to the user but never sent to Claude.
// If no session exists for the itemID, one is created.
func (m *Manager) AddInfoMessage(itemID, content string) {
	msg := Message{Role: RoleInfo, Content: content}

	m.mu.Lock()
	session, ok := m.sessions[itemID]
	if !ok {
		session = m.newBaseSession(itemID)
		m.sessions[itemID] = session
	}
	session.Messages = append(session.Messages, msg)
	m.mu.Unlock()

	// Persist info message
	if m.store != nil {
		if err := m.store.appendMessage(itemID, msg); err != nil {
			log.Printf("[agent:%s] failed to persist info message: %v", itemID, err)
		}
	}

	m.notify(itemID)
}

// SetPlanMode toggles plan mode for a session, killing any running process
// since --permission-mode is a startup flag that requires respawning.
func (m *Manager) SetPlanMode(itemID string, planMode bool) error {
	m.mu.Lock()
	session, hasSession := m.sessions[itemID]
	if !hasSession {
		// Create session in memory so the mode is set before any messages
		session = m.newBaseSession(itemID)
		session.PlanMode = planMode
		m.sessions[itemID] = session
		m.mu.Unlock()
		m.notify(itemID)
		return nil
	}

	if session.PlanMode == planMode {
		m.mu.Unlock()
		return nil
	}

	session.PlanMode = planMode

	proc, hasProc := m.processes[itemID]
	if hasProc {
		delete(m.processes, itemID)
		session.Status = StatusIdle
	}
	m.mu.Unlock()

	if hasProc && proc != nil {
		proc.kill()
	}

	m.notify(itemID)
	return nil
}

// SetActMode toggles act mode for a session, killing any running process
// since --dangerously-skip-permissions is a startup flag that requires respawning.
func (m *Manager) SetActMode(itemID string, actMode bool) error {
	m.mu.Lock()
	session, hasSession := m.sessions[itemID]
	if !hasSession {
		session = m.newBaseSession(itemID)
		session.ActMode = actMode
		m.sessions[itemID] = session
		m.mu.Unlock()
		m.notify(itemID)
		return nil
	}

	if session.ActMode == actMode {
		m.mu.Unlock()
		return nil
	}

	session.ActMode = actMode

	proc, hasProc := m.processes[itemID]
	if hasProc {
		delete(m.processes, itemID)
		session.Status = StatusIdle
	}
	m.mu.Unlock()

	if hasProc && proc != nil {
		proc.kill()
	}

	m.notify(itemID)
	return nil
}

// SetEffort sets the thinking effort level for a session, killing any running
// process since --effort is a startup flag that requires respawning.
func (m *Manager) SetEffort(itemID string, effort string) error {
	m.mu.Lock()
	session, hasSession := m.sessions[itemID]
	if !hasSession {
		session = &Session{
			ID:           itemID,
			AgentType:    "claude",
			Status:       StatusIdle,
			Effort:       effort,
			streamingIdx: -1,
		}
		m.sessions[itemID] = session
		m.mu.Unlock()
		m.notify(itemID)
		return nil
	}

	if session.Effort == effort {
		m.mu.Unlock()
		return nil
	}

	session.Effort = effort

	proc, hasProc := m.processes[itemID]
	if hasProc {
		delete(m.processes, itemID)
		session.Status = StatusIdle
	}
	m.mu.Unlock()

	if hasProc && proc != nil {
		proc.kill()
	}

	m.notify(itemID)
	return nil
}

// SetPendingInteraction sets a pending interaction on a session, creating the
// session if it doesn't exist. Used for testing the plan approval UI.
func (m *Manager) SetPendingInteraction(itemID string, interaction *PendingInteraction) error {
	m.mu.Lock()
	session, hasSession := m.sessions[itemID]
	if !hasSession {
		session = &Session{
			ID:           itemID,
			AgentType:    "claude",
			Status:       StatusIdle,
			streamingIdx: -1,
		}
		m.sessions[itemID] = session
	}
	session.PendingInteraction = interaction
	m.mu.Unlock()

	m.notify(itemID)
	return nil
}

// ClearSession stops any running process, removes the session from memory,
// and deletes the persisted conversation file.
func (m *Manager) ClearSession(itemID string) error {
	// Delete persisted conversation BEFORE removing the in-memory session
	// and killing the process. This prevents a race where spawnAndRun's
	// cleanup goroutine notifies subscribers, causing GetSession to
	// re-materialize the session from a still-existing JSONL file.
	if m.store != nil {
		if err := m.store.clear(itemID); err != nil {
			log.Printf("[agent:%s] failed to clear conversation file: %v", itemID, err)
		}
	}

	m.mu.Lock()
	proc, hasProc := m.processes[itemID]
	if hasProc {
		delete(m.processes, itemID)
	}
	delete(m.sessions, itemID)
	m.mu.Unlock()

	if hasProc && proc != nil {
		proc.kill()
	}

	m.notify(itemID)
	return nil
}

// AttachmentPath returns the filesystem path for a stored image attachment.
// Used by the HTTP handler to serve images.
func (m *Manager) AttachmentPath(itemID, imageID string) (string, error) {
	if m.store == nil {
		return "", fmt.Errorf("no store configured")
	}
	return m.store.attachmentPath(itemID, imageID)
}

// pruneOrphanedAttachments removes attachment files that are no longer referenced
// by any message in the session. Called after compact to reclaim disk space.
func (m *Manager) pruneOrphanedAttachments(itemID string) {
	if m.store == nil {
		return
	}
	m.mu.RLock()
	s, ok := m.sessions[itemID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	var keepIDs []string
	for _, msg := range s.Messages {
		for _, img := range msg.Images {
			keepIDs = append(keepIDs, img.ID)
		}
	}
	m.mu.RUnlock()

	if err := m.store.pruneAttachments(itemID, keepIDs); err != nil {
		log.Printf("[agent:%s] failed to prune orphaned attachments: %v", itemID, err)
	}
}

// Shutdown kills all running processes. Call on server shutdown.
// Processes are killed concurrently to avoid accumulating per-process
// timeouts (each kill waits up to 3s for graceful exit).
func (m *Manager) Shutdown() {
	m.mu.Lock()
	procs := make(map[string]*runningProcess, len(m.processes))
	for k, v := range m.processes {
		procs[k] = v
	}
	m.processes = make(map[string]*runningProcess)
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, proc := range procs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proc.kill()
		}()
	}
	wg.Wait()
}

// SetDefaultEffort sets the default effort level applied to newly created sessions.
// Must be called during initialization, before any sessions are created.
func (m *Manager) SetDefaultEffort(effort EffortLevel) {
	m.defaultEffort = effort
}

// newBaseSession returns a freshly initialized session with default mode and effort applied.
func (m *Manager) newBaseSession(itemID string) *Session {
	s := &Session{
		ID:           itemID,
		AgentType:    "claude",
		Status:       StatusIdle,
		Effort:       string(m.defaultEffort),
		streamingIdx: -1,
	}
	m.applyDefaultMode(s)
	return s
}

// applyDefaultMode sets ActMode and PlanMode on a session based on the manager's default.
func (m *Manager) applyDefaultMode(s *Session) {
	switch m.defaultMode {
	case DefaultModePlan:
		s.PlanMode = true
		s.ActMode = false
	default: // act
		s.PlanMode = false
		s.ActMode = true
	}
}

// loadOrCreateSession loads a session from disk if persisted, or creates a new one.
// Must be called with m.mu held.
func (m *Manager) loadOrCreateSession(itemID, workDir string) *Session {
	session := m.newBaseSession(itemID)
	session.WorkDir = workDir

	if m.systemPromptProvider != nil {
		session.SystemPrompt = m.systemPromptProvider(itemID)
	}

	if m.store != nil {
		msgs, sessionID, err := m.store.load(itemID)
		if err != nil {
			log.Printf("[agent:%s] failed to load conversation: %v", itemID, err)
		} else if len(msgs) > 0 {
			session.Messages = msgs
			session.SessionID = sessionID
		}
	}

	return session
}

// countUserMessages returns how many messages in the slice have RoleUser.
func countUserMessages(msgs []Message) int {
	n := 0
	for _, m := range msgs {
		if m.Role == RoleUser {
			n++
		}
	}
	return n
}
