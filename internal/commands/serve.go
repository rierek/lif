package commands

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/rierek/lif/internal/agent"
	"github.com/rierek/lif/internal/cors"
	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/internal/graph"
	"github.com/rierek/lif/internal/portalloc"
	"github.com/rierek/lif/internal/terminal"
	"github.com/rierek/lif/internal/web"
	"github.com/rierek/lif/internal/worktree"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/forge"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

var (
	serveHost           string
	servePort           int
	corsOrigins         []string
	allowRemoteTerminal bool
)

const centralAgentPrompt = `You are the planning agent for this project. Your primary role is to help manage and organize work through items.

IMPORTANT: Do NOT use Claude Code's built-in worktree system (EnterWorktree tool). This project has its own worktree management. To start work on an item, use the GraphQL createWorktree mutation instead: mutation { createWorktree(name: "<short-kebab-name>") { id path } } — then hand the item to the new worktree's agent with: mutation { sendAgentMessage(itemId: "<worktree id>", message: "Start working on item <item id>") }

CRITICAL — ASKING QUESTIONS:
You are running inside a web UI, NOT an interactive terminal. The user CANNOT see or respond to plain-text questions in your output. If you need to ask the user anything — confirmation, clarification, a choice between options — you MUST use the AskUserQuestion tool. Every single time. No exceptions. Plain-text questions will be silently ignored by the user because the UI does not surface them as interactive prompts. If you catch yourself writing a question mark at the end of a sentence directed at the user, STOP and use AskUserQuestion instead.

Your responsibilities:
- **Create and manage items**: When the user describes work to be done, create items for it rather than doing the work directly. Break large tasks into smaller, well-defined items with clear descriptions.
- **Organize work**: Help prioritize, categorize, and structure items. Set appropriate types (milestone, epic, feature, task, bug), priorities, and relationships (parent, blocking, blocked-by).
- **Start work on items**: When the user wants to begin working on a specific item, use the createWorktree GraphQL mutation (see above) to create a worktree for it. NEVER use the EnterWorktree tool.
- **Nudge towards items**: If the user asks you to implement something directly, suggest creating an item for it instead. The actual implementation work should happen in item-specific worktree agents, not here.
- **Review and refine**: Help the user review existing items, refine descriptions, update statuses, and maintain a clean backlog.

You have access to the lif CLI and can use GraphQL queries to inspect and modify items. Focus on planning and coordination — leave implementation to the worktree agents.`

var serveCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"server", "s"},
	Short:   "Start the web server",
	Long: `Start the web server.

By default the server listens on 127.0.0.1 only, so only this machine can reach
it. --host with any other address makes it reachable from the network. The
server then asks every client for an access token, printed at startup, and
serves the terminal only with --allow-remote-terminal. There is no TLS.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Determine the port: CLI flag > config > default
		port := servePort
		if !cmd.Flags().Changed("port") {
			port = cfg.GetServerPort()
		}

		// Determine CORS origins: CLI flag > config > default
		origins := corsOrigins
		if !cmd.Flags().Changed("cors-origin") {
			origins = cfg.GetCORSOrigins()
		}

		return runServer(serveHost, port, origins, allowRemoteTerminal)
	},
}

// gqlWebsocketImplementation enforces checker.CheckOriginFunc before accepting
// a GraphQL websocket connection, then hands off to gqlgen's default
// coder/websocket implementation. gqlgen dropped the gorilla/websocket
// Upgrader field in v0.17.9x in favor of the pluggable WebsocketImplementation
// interface, which has no CheckOrigin callback of its own.
type gqlWebsocketImplementation struct {
	checkOrigin func(r *http.Request) bool
}

func (g gqlWebsocketImplementation) Accept(w http.ResponseWriter, r *http.Request, options transport.WebsocketAcceptOptions) (transport.WebsocketConn, error) {
	if !g.checkOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return nil, fmt.Errorf("websocket: origin %q not allowed", r.Header.Get("Origin"))
	}
	// Origin is already verified above, so the coder/websocket layer doesn't
	// need to check it again.
	impl := transport.CoderWebsocketImplementation{AcceptOptions: coderws.AcceptOptions{InsecureSkipVerify: true}}
	return impl.Accept(w, r, options)
}

// runServer serves the web UI and the API on host:port until it is told to
// stop. The address the listener really gets decides who may use the server;
// see serveAccess.
func runServer(host string, port int, origins []string, allowRemoteTerminal bool) error {
	// Bind first: a taken port fails before anything else starts, and the real
	// address, not the text of --host, decides the access rules.
	ln, err := net.Listen("tcp", net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	defer ln.Close()
	access, err := newServeAccess(ln.Addr(), allowRemoteTerminal)
	if err != nil {
		return err
	}
	// A loopback listener also answers under its own address (::1, 127.0.0.2),
	// which the default list does not name.
	origins = access.withOwnAddress(origins)

	// Start file watcher for subscriptions
	if err := core.StartWatching(); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}
	defer core.Unwatch()

	// Set up origin checker for CORS and WebSocket
	checker := cors.NewChecker(origins)

	// Set Gin to release mode for cleaner output
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	router := gin.New()

	// Add middleware
	router.Use(accessLog(gin.DefaultWriter))
	router.Use(gin.Recovery())

	// Access guards come first: a refused request takes no WebSocket slot and
	// reaches no handler.
	router.Use(access.middleware(checker)...)

	// Resource limits: bound each request body, and cap concurrent WebSocket
	// connections across every upgrade endpoint.
	router.Use(maxBodyMiddleware(maxRequestBodyBytes))
	router.Use(newConnLimiter(maxWebSocketConnections).Middleware())

	// CORS middleware
	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed := checker.CORSOrigin(origin); allowed != "" {
			c.Header("Access-Control-Allow-Origin", allowed)
			if allowed != "*" {
				c.Header("Vary", "Origin")
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
	// Resolve worktree root directory (default: ~/.lif/worktrees/<project>/, or
	// ~/.beans/worktrees/<project>/ when that legacy root exists)
	projectName := cfg.GetProjectName()
	if projectName == "" {
		projectName = filepath.Base(cfg.ConfigDir())
	}
	worktreeRoot, _, err := cfg.ResolveWorktreePath(projectName)
	if err != nil {
		return fmt.Errorf("failed to resolve worktree path: %w", err)
	}
	if err := os.MkdirAll(worktreeRoot, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory %s: %w", worktreeRoot, err)
	}
	log.Printf("[lif] worktrees directory: %s", worktreeRoot)

	// Warn if old in-repo worktrees directory exists
	oldWorktreeDir := filepath.Join(core.Root(), ".worktrees")
	if entries, err := os.ReadDir(oldWorktreeDir); err == nil && len(entries) > 0 {
		log.Printf("[lif] WARNING: found old worktrees in %s — worktrees are now created in %s. You may want to recreate existing worktrees and remove the old directory.", oldWorktreeDir, worktreeRoot)
	}

	wtManager := newWorktreeManager(cfg, core.Root(), worktreeRoot)

	// Watch existing worktrees for item changes
	if existingWTs, err := wtManager.List(); err == nil {
		for _, wt := range existingWTs {
			if err := core.WatchWorktreeItems(wt.Path); err != nil {
				fmt.Printf("[lif] warning: failed to watch worktree %s: %v\n", wt.ID, err)
			}
		}
	}

	// Create workspace port allocator and allocate port for central workspace
	portAlloc := portalloc.NewDefault()
	portAlloc.Allocate(graph.CentralSessionID)

	// Restore persisted ports for existing worktrees (or allocate new ones)
	if existingWTs, err := wtManager.List(); err == nil {
		for _, wt := range existingWTs {
			var port int
			if savedPort := wtManager.GetPort(wt.ID); savedPort > 0 {
				port = portAlloc.AllocateSpecific(wt.ID, savedPort)
			} else {
				port = portAlloc.Allocate(wt.ID)
			}
			// Persist the port (writes back the actual port, which may differ
			// from the saved one if there was a conflict).
			if err := wtManager.SavePort(wt.ID, port); err != nil {
				log.Printf("[lif] warning: failed to save port for %s: %v", wt.ID, err)
			}
		}
	}

	// Create terminal session manager with workspace port injection. Left nil
	// when the terminal is disabled: the GraphQL terminal and run operations
	// then fail with "terminal support not available", as in `lif graphql`.
	var termMgr *terminal.Manager
	if access.terminalEnabled() {
		termMgr = terminal.NewManager(workspaceEnvFunc(portAlloc))
		defer termMgr.Shutdown()
	}

	// Create agent session manager (with conversation persistence)
	agentMgr := agent.NewManager(core.Root(), func(itemID string) string {
		// Central/planning agent gets a planning-focused prompt
		if itemID == graph.CentralSessionID {
			return centralAgentPrompt
		}

		// Item-specific agents get context about the item they're working on
		b, err := core.Get(itemID)
		if err != nil {
			return ""
		}
		return itemAgentContext(b)
	}, agent.DefaultMode(cfg.GetDefaultMode()))
	if effort := cfg.GetDefaultEffort(); config.IsValidEffortLevel(effort) {
		agentMgr.SetDefaultEffort(agent.EffortLevel(effort))
	}
	defer agentMgr.Shutdown()

	// Inject a system prompt that tells the agent which worktree/directory it's in.
	// This is separate from context (which goes in the first user message) because
	// the system prompt persists across --resume, ensuring the agent always knows
	// its workspace identity.
	agentMgr.SetSystemPromptProvider(func(itemID string) string {
		if itemID == graph.CentralSessionID {
			return centralPlanningPrompt(filepath.Dir(core.Root()))
		}
		wtPath := wtManager.WorktreePath(itemID)
		if wtPath == "" {
			return ""
		}
		return fmt.Sprintf("You are a workspace agent working in a git worktree. Your worktree ID is %q and your working directory is: %s\nAll file modifications MUST be within this directory. NEVER modify files in the main repository or other worktrees.\nNEVER force-push to main. NEVER push to origin/main. The Integrate action is local-only.\nNEVER merge pull requests. After creating a PR, stop and report the URL. Do not run `gh pr merge` or any equivalent.\nNEVER assume CI checks are absent. Checks take time to register after a push. If `gh pr checks` returns empty, it means checks haven't started yet, not that none are configured.", itemID, wtPath)
	})

	// Post an info message to the workspace's agent chat when setup finishes.
	wtManager.SetOnSetupDone(func(worktreeID string, success bool, output string) {
		if success {
			agentMgr.AddInfoMessage(worktreeID, "Workspace setup completed successfully.")
		} else {
			agentMgr.AddInfoMessage(worktreeID, fmt.Sprintf("Workspace setup failed:\n```\n%s\n```", output))
		}
	})

	// Auto-generate workspace descriptions when the first user message is sent.
	// Runs a cheap Claude call (Haiku) in the background to summarize what
	// the workspace is doing, then stores it as worktree metadata.
	agentMgr.SetOnFirstUserMessage(func(itemID string, message string) {
		// Only generate for worktree agents, not central
		if itemID == graph.CentralSessionID {
			return
		}
		// Only if this worktree exists and doesn't already have a description
		if wts, err := wtManager.List(); err == nil {
			for _, wt := range wts {
				if wt.ID == itemID && wt.Description == "" {
					desc := agent.GenerateDescription(message)
					if desc != "" {
						if err := wtManager.UpdateDescription(itemID, desc); err != nil {
							log.Printf("[lif] failed to save workspace description for %s: %v", itemID, err)
						} else {
							log.Printf("[lif] generated workspace description for %s: %q", itemID, desc)
						}
					}
					break
				}
			}
		}
	})

	// Update workspace activity timestamp when an agent completes a turn,
	// so the sidebar sorts workspaces by most recently active.
	agentMgr.SetOnTurnComplete(func(itemID string) {
		if itemID == graph.CentralSessionID || wtManager == nil {
			return
		}
		if err := wtManager.TouchLastActive(itemID); err != nil {
			log.Printf("[lif] failed to update last_active_at for %s: %v", itemID, err)
		}
	})

	// When item files change in a worktree, also notify the worktree manager
	// so the worktree subscription re-emits with updated detected item IDs.
	if wtManager != nil {
		core.SetOnWorktreeItemsChanged(wtManager.Notify)
	}

	// Detect git forge (GitHub, GitLab, etc.) for PR integration
	projectRoot := filepath.Dir(core.Root())
	forgeProvider := forge.Detect(projectRoot)
	if forgeProvider != nil {
		fmt.Printf("[lif] detected forge: %s (using %s CLI)\n", forgeProvider.Name(), forgeProvider.CLIName())
	}

	// Provide workspace context (PR status, git state) for quick reply generation.
	agentMgr.SetQuickReplyContext(func(itemID string) string {
		if itemID == graph.CentralSessionID || wtManager == nil {
			return ""
		}
		var branch, wtPath string
		if wts, err := wtManager.List(); err == nil {
			for _, wt := range wts {
				if wt.ID == itemID {
					branch = wt.Branch
					wtPath = wt.Path
					break
				}
			}
		}
		if wtPath == "" {
			return ""
		}

		var lines []string
		if branch != "" {
			lines = append(lines, fmt.Sprintf("Branch: %s", branch))
		}
		if gitutil.HasChanges(wtPath) {
			lines = append(lines, "Has uncommitted changes: yes")
		}
		if gitutil.HasUnmergedCommits(wtPath, wtManager.BaseRef()) {
			lines = append(lines, "Has commits not yet merged to base branch: yes")
		}
		if gitutil.HasUnpushedCommits(wtPath) {
			lines = append(lines, "Has unpushed commits: yes")
		}
		if gitutil.HasConflicts(wtPath, wtManager.BaseRef()) {
			lines = append(lines, "Has conflicts with base branch: yes")
		}
		if forgeProvider != nil && branch != "" {
			if pr, _ := forgeProvider.FindPR(context.Background(), projectRoot, branch); pr != nil {
				state := pr.State
				if pr.IsDraft {
					state = "draft"
				}
				lines = append(lines, fmt.Sprintf("Pull request: #%d (%s)", pr.Number, state))
				if pr.Checks != "" {
					lines = append(lines, fmt.Sprintf("CI checks: %s", pr.Checks))
				}
				if pr.ReviewApproved {
					lines = append(lines, "Review: approved")
				}
			} else {
				lines = append(lines, "Pull request: none")
			}
		}
		return strings.Join(lines, "\n")
	})

	// Create GraphQL server with explicit transports
	es := graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			CoreResolver: &itemgraph.CoreResolver{Core: core},
			WorktreeMgr:  wtManager,
			AgentMgr:     agentMgr,
			TerminalMgr:  termMgr,
			PortAlloc:    portAlloc,
			Forge:        forgeProvider,
			ProjectRoot:  projectRoot,
		},
	})
	gqlHandler := handler.New(es)

	// Add transports in order (WebSocket first for upgrade handling)
	gqlHandler.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Implementation:        gqlWebsocketImplementation{checkOrigin: checker.CheckOriginFunc()},
	})
	gqlHandler.AddTransport(transport.Options{})
	gqlHandler.AddTransport(transport.GET{})
	gqlHandler.AddTransport(transport.POST{})

	// GraphQL API endpoint (handle all methods for WebSocket upgrade)
	router.Any("/api/graphql", gin.WrapH(gqlHandler))

	// GraphQL Playground
	router.GET("/playground", gin.WrapH(playground.Handler("Lif GraphQL", "/api/graphql")))

	// Serve agent chat image attachments
	router.GET("/api/attachments/:beanId/:filename", func(c *gin.Context) {
		itemID := c.Param("beanId")
		filename := c.Param("filename")
		path, err := agentMgr.AttachmentPath(itemID, filename)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(path); err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Disposition", "inline")
		c.Header("X-Content-Type-Options", "nosniff")
		c.File(path)
	})

	// Terminal WebSocket endpoint
	if access.terminalEnabled() {
		RegisterTerminalRoute(router, termMgr, wtManager, checker.CheckOriginFunc(), filepath.Dir(core.Root()))
	} else {
		registerTerminalDisabled(router)
	}

	// Serve the embedded frontend SPA
	router.NoRoute(gin.WrapH(web.Handler()))

	// Create HTTP server
	server := newHTTPServer(ln.Addr().String(), router)

	// Set up signal handling with context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Channel to listen for server errors
	serverErr := make(chan error, 1)

	// The listener is already open: print what it exposes, then serve on it.
	interfaceAddrs, _ := net.InterfaceAddrs() // none: a listener on all interfaces is shown as localhost
	for _, line := range access.startupLines(origins, interfaceAddrs) {
		fmt.Println(line)
	}
	go func() {
		serverErr <- server.Serve(ln)
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		if err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case <-ctx.Done():
		fmt.Printf("\nShutting down...\n")

		// Hard deadline: if graceful shutdown takes too long, force exit.
		// This prevents zombie processes when cleanup hangs (e.g. a claude
		// process ignores SIGINT, or a WebSocket handler blocks).
		go func() {
			time.Sleep(10 * time.Second)
			fmt.Fprintf(os.Stderr, "Shutdown deadline exceeded, forcing exit\n")
			os.Exit(1)
		}()

		// Create context with timeout for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Graceful shutdown timed out: %v\n", err)
			fmt.Println("Forcing exit...")
			server.Close() // Force close all connections
		}
		fmt.Println("Server stopped")
	}

	return nil
}

// newWorktreeManager builds the worktree manager for the store at storeRoot.
// Item changes inside a worktree are looked up under the directory that
// carries the same name as the main store's directory.
func newWorktreeManager(c *config.Config, storeRoot, worktreeRoot string) *worktree.Manager {
	return worktree.NewManager(c.ConfigDir(), worktreeRoot, c.GetWorktreeBaseRef(), c.GetWorktreeSetup(),
		worktree.WithStoreDir(filepath.Base(storeRoot)),
		worktree.WithFetchTimeout(c.GetWorktreeFetchTimeout()),
	)
}

func RegisterServeCmd(root *cobra.Command) {
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "Address to listen on; any address but a loopback one makes the server reachable from the network and requires the access token")
	serveCmd.Flags().IntVarP(&servePort, "port", "p", config.DefaultServerPort, "Port to listen on")
	serveCmd.Flags().StringSliceVar(&corsOrigins, "cors-origin", cors.DefaultOrigins, "Origins and hosts of web pages that may use the server besides its own UI (CORS, WebSocket, requests that change state); replaces the default list; * adds the CORS header for every origin and allows nothing else")
	serveCmd.Flags().BoolVar(&allowRemoteTerminal, "allow-remote-terminal", false, "Serve the terminal even when the server listens beyond loopback")
	root.AddCommand(serveCmd)
}

// workspaceEnvFunc returns the terminal environment function of a serve
// process: every workspace shell session and run session gets the workspace's
// port as LIF_WORKSPACE_PORT and, for user scripts of the former tool, as
// BEANS_WORKSPACE_PORT with the same value. Run sessions use
// "${workspaceId}__run" as session ID — the suffix is stripped so they share
// the same port as the workspace's shell session. A session of a workspace
// without an allocated port gets none.
func workspaceEnvFunc(portAlloc *portalloc.Allocator) terminal.EnvFunc {
	return func(sessionID string) []string {
		workspaceID := strings.TrimSuffix(sessionID, "__run")
		port, err := portAlloc.Get(workspaceID)
		if err != nil {
			return nil
		}
		return []string{
			fmt.Sprintf("%s=%d", config.WorkspacePortEnvVar, port),
			fmt.Sprintf("%s=%d", config.LegacyWorkspacePortEnvVar, port),
		}
	}
}

// itemAgentContext is the first-message context an item's agent session
// starts with: the worktree rules plus the item it works on.
func itemAgentContext(b *item.Item) string {
	var sb strings.Builder
	sb.WriteString("IMPORTANT: Do NOT use Claude Code's built-in worktree system (EnterWorktree tool). You are already working inside a lif-managed worktree.\n\n")
	sb.WriteString("CRITICAL — ASKING QUESTIONS:\nYou are running inside a web UI, NOT an interactive terminal. The user CANNOT see or respond to plain-text questions in your output. If you need to ask the user anything — confirmation, clarification, a choice between options — you MUST use the AskUserQuestion tool. Every single time. No exceptions. Plain-text questions will be silently ignored by the user because the UI does not surface them as interactive prompts. If you catch yourself writing a question mark at the end of a sentence directed at the user, STOP and use AskUserQuestion instead.\n\n")
	sb.WriteString("IMPORTANT: You MUST only create or modify files within this worktree directory. NEVER make changes to files in the main repository or any other worktree. All your file operations (reads are fine anywhere, but writes, edits, and deletions) must be scoped to your current working directory.\n\n")
	fmt.Fprintf(&sb, "You are working on item %s: %q\n", b.ID, b.Title)
	fmt.Fprintf(&sb, "Type: %s | Status: %s", b.Type, b.Status)
	if b.Priority != "" {
		fmt.Fprintf(&sb, " | Priority: %s", b.Priority)
	}
	sb.WriteString("\n")
	if b.Body != "" {
		fmt.Fprintf(&sb, "\nDescription:\n%s", b.Body)
	}
	return sb.String()
}

// centralPlanningPrompt is the system prompt of the central planning agent,
// which works in the main repository at root.
func centralPlanningPrompt(root string) string {
	return fmt.Sprintf("You are the central planning agent for the Lif project. Your working directory is the main repository at: %s\nNEVER merge pull requests. After creating a PR, stop and report the URL. Do not run `gh pr merge` or any equivalent.\nNEVER assume CI checks are absent. Checks take time to register after a push. If `gh pr checks` returns empty, it means checks haven't started yet, not that none are configured.", root)
}
