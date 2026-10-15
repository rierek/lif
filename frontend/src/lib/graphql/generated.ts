import type { TypedDocumentNode as DocumentNode } from 'urql';

export type Maybe<T> = T | null;
export type InputMaybe<T> = Maybe<T>;
export type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]?: Maybe<T[SubKey]> };
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & { [SubKey in K]: Maybe<T[SubKey]> };
export type MakeEmpty<T extends { [key: string]: unknown }, K extends keyof T> = { [_ in K]?: never };
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: { input: string; output: string; }
  String: { input: string; output: string; }
  Boolean: { input: boolean; output: boolean; }
  Int: { input: number; output: number; }
  Float: { input: number; output: number; }
  Map: { input: any; output: any; }
  Time: { input: string; output: string; }
};

/** Lightweight status for tracking which items have running agents */
export type ActiveAgentStatus = {
  /** Item ID with an active agent */
  itemId: Scalars['ID']['output'];
  /** Current agent status */
  status: AgentSessionStatus;
};

/** An action that can be performed by the agent */
export type AgentAction = {
  /** Description of what the action does */
  description?: Maybe<Scalars['String']['output']>;
  /** Whether the action is currently disabled */
  disabled: Scalars['Boolean']['output'];
  /** Reason why the action is disabled (shown as tooltip) */
  disabledReason?: Maybe<Scalars['String']['output']>;
  /** Unique identifier for the action */
  id: Scalars['ID']['output'];
  /** Human-readable label */
  label: Scalars['String']['output'];
};

/** A single message in an agent conversation */
export type AgentMessage = {
  /** File/directory paths attached via @-mention (only present on user messages) */
  attachments: Array<Scalars['String']['output']>;
  /** Text content */
  content: Scalars['String']['output'];
  /** Unified diff output (only present on tool messages for Write/Edit tools) */
  diff?: Maybe<Scalars['String']['output']>;
  /** Attached images (empty for assistant/tool messages) */
  images: Array<AgentMessageImage>;
  /** Message role */
  role: AgentMessageRole;
};

/** An image attached to an agent message */
export type AgentMessageImage = {
  /** MIME type (e.g. image/png) */
  mediaType: Scalars['String']['output'];
  /** URL to fetch the image */
  url: Scalars['String']['output'];
};

/** Role of an agent message sender */
export enum AgentMessageRole {
  Assistant = 'ASSISTANT',
  Info = 'INFO',
  Tool = 'TOOL',
  User = 'USER'
}

/** An agent chat session within a worktree */
export type AgentSession = {
  /** Whether the agent is in act mode (fully autonomous, no permission prompts) */
  actMode: Scalars['Boolean']['output'];
  /** Agent type (e.g., 'claude') */
  agentType: Scalars['String']['output'];
  /** Thinking effort level (e.g. 'low', 'medium', 'high', 'max'), null when using CLI default */
  effort?: Maybe<Scalars['String']['output']>;
  /** Last error message, if any */
  error?: Maybe<Scalars['String']['output']>;
  /** Item ID (worktree identifier) */
  itemId: Scalars['ID']['output'];
  /** Chat messages in chronological order */
  messages: Array<AgentMessage>;
  /** Pending blocking interaction awaiting user response */
  pendingInteraction?: Maybe<PendingInteraction>;
  /** Whether the agent is in plan mode (read-only) */
  planMode: Scalars['Boolean']['output'];
  /** Suggested quick reply messages generated after a turn completes */
  quickReplies: Array<Scalars['String']['output']>;
  /** Current session status */
  status: AgentSessionStatus;
  /** Currently running subagent activities (one per concurrent Agent tool call) */
  subagentActivities: Array<SubagentActivity>;
  /** Transient system status (e.g. 'compacting'), null when idle */
  systemStatus?: Maybe<Scalars['String']['output']>;
  /** Working directory / worktree path for this session */
  workDir?: Maybe<Scalars['String']['output']>;
};

/** Status of an agent session */
export enum AgentSessionStatus {
  Error = 'ERROR',
  Idle = 'IDLE',
  Running = 'RUNNING'
}

/** A selectable option within an AskUserQuestion */
export type AskUserOption = {
  /** Explanation of what this option means */
  description: Scalars['String']['output'];
  /** Display text for this option */
  label: Scalars['String']['output'];
};

/** A structured question with selectable options from the AskUserQuestion tool */
export type AskUserQuestion = {
  /** Short label displayed as a chip/tag */
  header: Scalars['String']['output'];
  /** Whether multiple options can be selected */
  multiSelect: Scalars['Boolean']['output'];
  /** Available choices */
  options: Array<AskUserOption>;
  /** The full question text */
  question: Scalars['String']['output'];
};

/**
 * Structured body modifications applied atomically.
 * Operations are applied in order: all replacements sequentially, then append.
 * If any operation fails, the entire mutation fails (transactional).
 */
export type BodyModification = {
  /**
   * Text to append after all replacements.
   * Appended with blank line separator.
   */
  append?: InputMaybe<Scalars['String']['input']>;
  /**
   * Text replacements applied sequentially in array order.
   * Each old text must match exactly once at the time it's applied.
   */
  replace?: InputMaybe<Array<ReplaceOperation>>;
};

/** Branch status relative to the base branch */
export type BranchStatus = {
  /** Number of commits on the base branch not reachable from this branch */
  commitsBehind: Scalars['Int']['output'];
  /** Whether rebasing onto the base branch would produce merge conflicts */
  hasConflicts: Scalars['Boolean']['output'];
};

/** Type of change that occurred to an item */
export enum ChangeType {
  Created = 'CREATED',
  Deleted = 'DELETED',
  /** All existing items sent as a single batch when subscription starts (emitted when includeInitial=true) */
  InitialSnapshot = 'INITIAL_SNAPSHOT',
  Updated = 'UPDATED'
}

/** Input for creating a new item */
export type CreateItemInput = {
  /** Item IDs that are blocking this item */
  blockedBy?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Item IDs this item is blocking */
  blocking?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Markdown body content */
  body?: InputMaybe<Scalars['String']['input']>;
  /** Parent item ID (validated against type hierarchy) */
  parent?: InputMaybe<Scalars['String']['input']>;
  /** Custom ID prefix (overrides config prefix for this item) */
  prefix?: InputMaybe<Scalars['String']['input']>;
  /** Priority level (defaults to 'normal') */
  priority?: InputMaybe<Scalars['String']['input']>;
  /** Status (defaults to 'todo') */
  status?: InputMaybe<Scalars['String']['input']>;
  /** Tags for categorization */
  tags?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Item title (required) */
  title: Scalars['String']['input'];
  /** Item type (defaults to 'task') */
  type?: InputMaybe<Scalars['String']['input']>;
};

/** Input for attaching a file or directory as context to an agent message. */
export type FileAttachmentInput = {
  /** Relative file or directory path */
  path: Scalars['String']['input'];
};

/** A changed file in a git working tree */
export type FileChange = {
  /** Number of added lines */
  additions: Scalars['Int']['output'];
  /** Number of deleted lines */
  deletions: Scalars['Int']['output'];
  /** File path relative to the repo/worktree root */
  path: Scalars['String']['output'];
  /** Whether this change is staged */
  staged: Scalars['Boolean']['output'];
  /** Change status: modified, added, deleted, untracked, renamed */
  status: Scalars['String']['output'];
};

/** A file entry in the project, used for @-mention autocomplete. */
export type FileEntry = {
  /** Relative path from the workspace root */
  path: Scalars['String']['output'];
};

/** Input for uploading an image attachment */
export type ImageInput = {
  /** Base64-encoded image data */
  data: Scalars['String']['input'];
  /** MIME type (image/jpeg, image/png, image/gif, image/webp) */
  mediaType: Scalars['String']['input'];
};

/** Type of blocking interaction */
export enum InteractionType {
  AskUser = 'ASK_USER',
  EnterPlan = 'ENTER_PLAN',
  ExitPlan = 'EXIT_PLAN'
}

/** An item represents an issue/task in the tracker */
export type Item = {
  /** Items that block this one (incoming blocking links) */
  blockedBy: Array<Item>;
  /** IDs of items that are blocking this item (direct field) */
  blockedByIds: Array<Scalars['String']['output']>;
  /** Items this one is blocking (resolved from blockingIds) */
  blocking: Array<Item>;
  /** IDs of items this item is blocking */
  blockingIds: Array<Scalars['String']['output']>;
  /** Markdown body content */
  body: Scalars['String']['output'];
  /** Child items (items with this as parent) */
  children: Array<Item>;
  /** Creation timestamp */
  createdAt: Scalars['Time']['output'];
  /** Content hash for optimistic concurrency control */
  etag: Scalars['String']['output'];
  /** Custom front matter keys not covered by the known schema (null if none) */
  extra?: Maybe<Scalars['Map']['output']>;
  /** Unique identifier (NanoID) */
  id: Scalars['ID']['output'];
  /** Terminal status (scrapped or completed) inherited from the nearest terminal ancestor, if any */
  implicitStatus?: Maybe<Scalars['String']['output']>;
  /** ID of the ancestor item that provides the implicit status */
  implicitStatusFrom?: Maybe<Scalars['String']['output']>;
  /** Whether this item has unsaved runtime changes (not yet persisted to disk) */
  isDirty: Scalars['Boolean']['output'];
  /** Fractional index for manual ordering within status groups */
  order: Scalars['String']['output'];
  /** Parent item (resolved from parentId) */
  parent?: Maybe<Item>;
  /** Parent item ID (optional, type-restricted) */
  parentId?: Maybe<Scalars['String']['output']>;
  /** Relative path from the store directory */
  path: Scalars['String']['output'];
  /** Priority level (critical, high, normal, low, deferred) */
  priority: Scalars['String']['output'];
  /** Human-readable slug from filename */
  slug?: Maybe<Scalars['String']['output']>;
  /** Current status (draft, todo, in-progress, completed, scrapped) */
  status: Scalars['String']['output'];
  /** Tags for categorization */
  tags: Array<Scalars['String']['output']>;
  /** Item title */
  title: Scalars['String']['output'];
  /** Item type (milestone, epic, bug, feature, task) */
  type: Scalars['String']['output'];
  /** Last update timestamp */
  updatedAt: Scalars['Time']['output'];
  /** ID of the worktree this item is linked to (null if not linked to any worktree) */
  worktreeId?: Maybe<Scalars['String']['output']>;
};


/** An item represents an issue/task in the tracker */
export type ItemBlockedByArgs = {
  filter?: InputMaybe<ItemFilter>;
};


/** An item represents an issue/task in the tracker */
export type ItemBlockingArgs = {
  filter?: InputMaybe<ItemFilter>;
};


/** An item represents an issue/task in the tracker */
export type ItemChildrenArgs = {
  filter?: InputMaybe<ItemFilter>;
};

/** Represents a change to an item */
export type ItemChangeEvent = {
  /** The item that changed (null for INITIAL_SNAPSHOT and DELETED events) */
  item?: Maybe<Item>;
  /** ID of the item that changed (empty for INITIAL_SNAPSHOT events) */
  itemId: Scalars['ID']['output'];
  /** All items as a batch (only present for INITIAL_SNAPSHOT events) */
  items?: Maybe<Array<Item>>;
  /** Type of change that occurred */
  type: ChangeType;
};

/** Filter options for querying items */
export type ItemFilter = {
  /** Include only items blocked by this specific item ID (via blocked_by field) */
  blockedById?: InputMaybe<Scalars['String']['input']>;
  /** Include only items that are blocking this specific item ID */
  blockingId?: InputMaybe<Scalars['String']['input']>;
  /** Exclude items that inherit a terminal status (scrapped or completed) from an ancestor */
  excludeImplicitTerminal?: InputMaybe<Scalars['Boolean']['input']>;
  /** Exclude items with these priorities */
  excludePriority?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Exclude items with these statuses */
  excludeStatus?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Exclude items with any of these tags */
  excludeTags?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Exclude items with these types */
  excludeType?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Include only items that have explicit blocked-by entries */
  hasBlockedBy?: InputMaybe<Scalars['Boolean']['input']>;
  /** Include only items that are blocking other items */
  hasBlocking?: InputMaybe<Scalars['Boolean']['input']>;
  /** Include only items with a parent */
  hasParent?: InputMaybe<Scalars['Boolean']['input']>;
  /** Include items that are blocked — explicitly (direct blockers) or implicitly (ancestor is blocked) */
  isBlocked?: InputMaybe<Scalars['Boolean']['input']>;
  /** Filter items that are explicitly blocked (have direct active blockers) */
  isExplicitlyBlocked?: InputMaybe<Scalars['Boolean']['input']>;
  /** Filter items that are implicitly blocked (an ancestor in the parent chain is blocked) */
  isImplicitlyBlocked?: InputMaybe<Scalars['Boolean']['input']>;
  /** Exclude items that have explicit blocked-by entries */
  noBlockedBy?: InputMaybe<Scalars['Boolean']['input']>;
  /** Exclude items that are blocking other items */
  noBlocking?: InputMaybe<Scalars['Boolean']['input']>;
  /** Exclude items that have a parent */
  noParent?: InputMaybe<Scalars['Boolean']['input']>;
  /** Include only items with this specific parent ID */
  parentId?: InputMaybe<Scalars['String']['input']>;
  /** Include only items with these priorities (OR logic) */
  priority?: InputMaybe<Array<Scalars['String']['input']>>;
  /**
   * Full-text search across slug, title, and body using Bleve query syntax.
   *
   * Examples:
   * - "login" - exact term match
   * - "login~" - fuzzy match (1 edit distance)
   * - "login~2" - fuzzy match (2 edit distance)
   * - "log*" - wildcard prefix
   * - "\"user login\"" - exact phrase
   * - "user login" - either term (a list of terms is an OR)
   * - "+user +login" - both terms required
   * - "login -user" - "login" without "user"
   * - "slug:auth" - search only slug field
   * - "title:login" - search only title field
   * - "body:auth" - search only body field
   */
  search?: InputMaybe<Scalars['String']['input']>;
  /** Include only items with these statuses (OR logic) */
  status?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Include only items with any of these tags (OR logic) */
  tags?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Include only items with these types (OR logic) */
  type?: InputMaybe<Array<Scalars['String']['input']>>;
};

export type Mutation = {
  /** Add an item to the blocked-by list (this item is blocked by targetId) */
  addBlockedBy: Item;
  /** Add an item to the blocking list */
  addBlocking: Item;
  /**
   * Archive an item by moving it to the archive directory.
   * Only items with archive-eligible statuses (completed, scrapped) can be archived.
   */
  archiveItem: Scalars['Boolean']['output'];
  /**
   * Clear the agent session for an item. Stops any running process, removes the
   * session from memory, and deletes persisted conversation history.
   */
  clearAgentSession: Scalars['Boolean']['output'];
  /** Create a new item */
  createItem: Item;
  /** Create a new worktree. Returns the created worktree with a generated ID. */
  createWorktree: Worktree;
  /** Delete an item by ID (automatically removes incoming links) */
  deleteItem: Scalars['Boolean']['output'];
  /**
   * Discard a file change (restore tracked file or remove untracked file).
   * If staged is true, the file is unstaged first.
   * The filePath is relative to the repo/worktree root.
   * If path is null, operates on the project root.
   */
  discardFileChange: Scalars['Boolean']['output'];
  /**
   * Execute a predefined agent action (e.g., "commit", "review") by injecting
   * the corresponding prompt into the agent conversation.
   */
  executeAgentAction: Scalars['Boolean']['output'];
  /**
   * Open a workspace directory in VS Code. For the main workspace, opens the
   * project root. For worktrees, opens the worktree directory.
   */
  openInEditor: Scalars['Boolean']['output'];
  /** Remove an item from the blocked-by list */
  removeBlockedBy: Item;
  /** Remove an item from the blocking list */
  removeBlocking: Item;
  /** Remove a worktree by its ID (works for both item-attached and standalone worktrees). */
  removeWorktree: Scalars['Boolean']['output'];
  /** Save all dirty items to disk. Returns the number of items saved. */
  saveDirtyItems: Scalars['Int']['output'];
  /** Save a specific item to disk (must be dirty). Returns true if saved. */
  saveItem: Scalars['Boolean']['output'];
  /**
   * Send a message to the agent in a worktree. Starts a session if none exists.
   * Optionally attach images (base64-encoded).
   */
  sendAgentMessage: Scalars['Boolean']['output'];
  /**
   * Set act mode for an agent session. Act mode makes the agent fully autonomous
   * (no permission prompts). Kills any running process since the flag requires respawning.
   */
  setAgentActMode: Scalars['Boolean']['output'];
  /**
   * Set the thinking effort level for an agent session. Kills any running process
   * since --effort is a startup flag. Use "low", "medium", "high", or "max".
   * Empty string clears the override (uses CLI default).
   */
  setAgentEffort: Scalars['Boolean']['output'];
  /**
   * Set a pending interaction on an agent session (creates session if needed).
   * Used to simulate blocking tool calls for testing the approval UI.
   */
  setAgentPendingInteraction: Scalars['Boolean']['output'];
  /**
   * Set plan mode for an agent session. Plan mode makes the agent read-only
   * (can explore/analyze but not edit). Kills any running process since the
   * permission mode is a startup flag.
   */
  setAgentPlanMode: Scalars['Boolean']['output'];
  /** Set or clear the parent of an item (validates type hierarchy) */
  setParent: Item;
  /**
   * Start the configured run command in a dedicated terminal session for a workspace.
   * Returns the allocated port number for the workspace.
   * If a run session is already active, it is stopped first.
   */
  startRun: Scalars['Int']['output'];
  /** Stop the running agent in a worktree. */
  stopAgent: Scalars['Boolean']['output'];
  /** Stop the run session for a workspace. */
  stopRun: Scalars['Boolean']['output'];
  /** Update an existing item */
  updateItem: Item;
  /**
   * Write input data to an existing terminal session's PTY.
   * Creates the session if it doesn't exist yet.
   */
  writeTerminalInput: Scalars['Boolean']['output'];
};


export type MutationAddBlockedByArgs = {
  id: Scalars['ID']['input'];
  ifMatch?: InputMaybe<Scalars['String']['input']>;
  targetId: Scalars['ID']['input'];
};


export type MutationAddBlockingArgs = {
  id: Scalars['ID']['input'];
  ifMatch?: InputMaybe<Scalars['String']['input']>;
  targetId: Scalars['ID']['input'];
};


export type MutationArchiveItemArgs = {
  id: Scalars['ID']['input'];
};


export type MutationClearAgentSessionArgs = {
  itemId: Scalars['ID']['input'];
};


export type MutationCreateItemArgs = {
  input: CreateItemInput;
};


export type MutationCreateWorktreeArgs = {
  name: Scalars['String']['input'];
};


export type MutationDeleteItemArgs = {
  id: Scalars['ID']['input'];
};


export type MutationDiscardFileChangeArgs = {
  filePath: Scalars['String']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
  staged: Scalars['Boolean']['input'];
};


export type MutationExecuteAgentActionArgs = {
  actionId: Scalars['ID']['input'];
  itemId: Scalars['ID']['input'];
};


export type MutationOpenInEditorArgs = {
  workspaceId: Scalars['ID']['input'];
};


export type MutationRemoveBlockedByArgs = {
  id: Scalars['ID']['input'];
  ifMatch?: InputMaybe<Scalars['String']['input']>;
  targetId: Scalars['ID']['input'];
};


export type MutationRemoveBlockingArgs = {
  id: Scalars['ID']['input'];
  ifMatch?: InputMaybe<Scalars['String']['input']>;
  targetId: Scalars['ID']['input'];
};


export type MutationRemoveWorktreeArgs = {
  id: Scalars['ID']['input'];
};


export type MutationSaveItemArgs = {
  id: Scalars['ID']['input'];
};


export type MutationSendAgentMessageArgs = {
  attachments?: InputMaybe<Array<FileAttachmentInput>>;
  images?: InputMaybe<Array<ImageInput>>;
  itemId: Scalars['ID']['input'];
  message: Scalars['String']['input'];
};


export type MutationSetAgentActModeArgs = {
  actMode: Scalars['Boolean']['input'];
  itemId: Scalars['ID']['input'];
};


export type MutationSetAgentEffortArgs = {
  effort: Scalars['String']['input'];
  itemId: Scalars['ID']['input'];
};


export type MutationSetAgentPendingInteractionArgs = {
  itemId: Scalars['ID']['input'];
  planContent?: InputMaybe<Scalars['String']['input']>;
  type: InteractionType;
};


export type MutationSetAgentPlanModeArgs = {
  itemId: Scalars['ID']['input'];
  planMode: Scalars['Boolean']['input'];
};


export type MutationSetParentArgs = {
  id: Scalars['ID']['input'];
  ifMatch?: InputMaybe<Scalars['String']['input']>;
  parentId?: InputMaybe<Scalars['String']['input']>;
};


export type MutationStartRunArgs = {
  workspaceId: Scalars['ID']['input'];
};


export type MutationStopAgentArgs = {
  itemId: Scalars['ID']['input'];
};


export type MutationStopRunArgs = {
  workspaceId: Scalars['ID']['input'];
};


export type MutationUpdateItemArgs = {
  id: Scalars['ID']['input'];
  input: UpdateItemInput;
};


export type MutationWriteTerminalInputArgs = {
  data: Scalars['String']['input'];
  sessionId: Scalars['String']['input'];
};

/** A blocking interaction the agent is waiting for user approval on */
export type PendingInteraction = {
  /** Plan file content (for EXIT_PLAN only) */
  planContent?: Maybe<Scalars['String']['output']>;
  /** Structured questions with selectable options (for ASK_USER only) */
  questions?: Maybe<Array<AskUserQuestion>>;
  /** Type of interaction */
  type: InteractionType;
};

/** A pull/merge request on a git forge (GitHub, GitLab, etc.) */
export type PullRequest = {
  /** Aggregate CI check status: pass, fail, pending */
  checkStatus: Scalars['String']['output'];
  /** Whether this PR is a draft */
  isDraft: Scalars['Boolean']['output'];
  /** Whether the forge reports the PR can be merged */
  mergeable: Scalars['Boolean']['output'];
  /** PR number on the forge */
  number: Scalars['Int']['output'];
  /** Whether review requirements are met */
  reviewApproved: Scalars['Boolean']['output'];
  /** Current state: open, closed, merged */
  state: Scalars['String']['output'];
  /** PR title */
  title: Scalars['String']['output'];
  /** Web URL to view the PR */
  url: Scalars['String']['output'];
};

export type Query = {
  /**
   * Get available agent actions for an item.
   * When skipForge is true, forge-dependent actions (PR buttons) are omitted
   * for a faster response — useful for the initial render before polling kicks in.
   */
  agentActions: Array<AgentAction>;
  /**
   * Whether agent functionality is enabled in the project configuration.
   * When false, the UI should hide agent chats, status panes, and worktree features.
   */
  agentEnabled: Scalars['Boolean']['output'];
  /** Get the current agent session for a worktree (null if none) */
  agentSession?: Maybe<AgentSession>;
  /**
   * Get all file changes compared to the upstream branch (committed + staged + unstaged + untracked).
   * If path is null, uses the project root.
   */
  allFileChanges: Array<FileChange>;
  /**
   * Get the unified diff for a specific file compared to the upstream branch merge-base.
   * Shows the complete change from merge-base to working tree.
   */
  allFileDiff: Scalars['String']['output'];
  /**
   * Get branch status for a worktree: how far behind the base branch and whether
   * a rebase would conflict. If path is null, uses the project root.
   */
  branchStatus: BranchStatus;
  /** Get file changes for a directory. If path is null, uses the project root. */
  fileChanges: Array<FileChange>;
  /**
   * Get the unified diff for a specific file. Returns the diff as a string.
   * The filePath is relative to the repo/worktree root.
   * If staged is true, shows the staged diff; otherwise shows the working tree diff.
   */
  fileDiff: Scalars['String']['output'];
  /** Whether any items have unsaved runtime changes */
  hasDirtyItems: Scalars['Boolean']['output'];
  /** Check whether a run session is alive for a workspace. */
  isRunning: Scalars['Boolean']['output'];
  /** Get a single item by ID. Accepts either the full ID or the short ID without prefix (e.g., "abc1"). */
  item?: Maybe<Item>;
  /** List items with optional filtering */
  items: Array<Item>;
  /**
   * List files tracked by git in a workspace directory.
   * Performs case-insensitive substring matching — space-separated terms must all
   * match somewhere in the file path. Used for @-mention autocomplete.
   */
  listFiles: Array<FileEntry>;
  /** The current branch of the main repository. */
  mainBranch: Scalars['String']['output'];
  /**
   * Human-readable project name from configuration.
   * Returns empty string if not configured.
   */
  projectName: Scalars['String']['output'];
  /** Get the allocated port for a workspace. Returns 0 if not allocated. */
  workspacePort: Scalars['Int']['output'];
  /**
   * The configured base ref for worktree branches (from worktree.base_ref config).
   * Used as the rebase target. Defaults to "main".
   */
  worktreeBaseRef: Scalars['String']['output'];
  /**
   * The configured integration mode for worktrees (from worktree.integrate config).
   * "local" = squash-merge locally (hides PR buttons).
   * "pr" = push and create PRs (hides Integrate button).
   * Defaults to "local".
   */
  worktreeIntegrateMode: Scalars['String']['output'];
  /**
   * Shell command to run the project (from worktree.run config).
   * When non-empty, the UI shows a "Run" button in workspace toolbars.
   */
  worktreeRunCommand: Scalars['String']['output'];
  /** List active git worktrees created by items */
  worktrees: Array<Worktree>;
};


export type QueryAgentActionsArgs = {
  itemId: Scalars['ID']['input'];
  skipForge?: InputMaybe<Scalars['Boolean']['input']>;
};


export type QueryAgentSessionArgs = {
  itemId: Scalars['ID']['input'];
};


export type QueryAllFileChangesArgs = {
  path?: InputMaybe<Scalars['String']['input']>;
};


export type QueryAllFileDiffArgs = {
  filePath: Scalars['String']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
};


export type QueryBranchStatusArgs = {
  path?: InputMaybe<Scalars['String']['input']>;
};


export type QueryFileChangesArgs = {
  path?: InputMaybe<Scalars['String']['input']>;
};


export type QueryFileDiffArgs = {
  filePath: Scalars['String']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
  staged: Scalars['Boolean']['input'];
};


export type QueryIsRunningArgs = {
  workspaceId: Scalars['ID']['input'];
};


export type QueryItemArgs = {
  id: Scalars['ID']['input'];
};


export type QueryItemsArgs = {
  filter?: InputMaybe<ItemFilter>;
};


export type QueryListFilesArgs = {
  limit?: InputMaybe<Scalars['Int']['input']>;
  prefix: Scalars['String']['input'];
  workspaceId?: InputMaybe<Scalars['ID']['input']>;
};


export type QueryWorkspacePortArgs = {
  workspaceId: Scalars['ID']['input'];
};

/** A single text replacement operation. */
export type ReplaceOperation = {
  /** Replacement text (can be empty to delete the matched text) */
  new: Scalars['String']['input'];
  /** Text to find (must occur exactly once, cannot be empty) */
  old: Scalars['String']['input'];
};

/** Tracks real-time activity of a running subagent (Agent tool invocation) */
export type SubagentActivity = {
  /** Tool currently being used by the subagent (empty string when idle) */
  currentTool: Scalars['String']['output'];
  /** What the subagent is currently doing */
  description: Scalars['String']['output'];
  /** Sequential index (1-based) for display */
  index: Scalars['Int']['output'];
  /** Unique task identifier for this subagent */
  taskId: Scalars['String']['output'];
};

export type Subscription = {
  /**
   * Subscribe to active agent status changes across all sessions.
   * Emits the list of currently running agents whenever any session status changes.
   */
  activeAgentStatuses: Array<ActiveAgentStatus>;
  /**
   * Subscribe to agent session updates for a worktree.
   * Emits the full session state whenever it changes (new messages, status changes).
   */
  agentSessionChanged: AgentSession;
  /**
   * Subscribe to item change events (created, updated, deleted).
   *
   * When includeInitial is true, all existing items are emitted as a single
   * INITIAL_SNAPSHOT event containing the full list, followed by real-time
   * changes. This eliminates race conditions between loading and subscribing.
   *
   * A client that cannot keep up also receives an INITIAL_SNAPSHOT: when the
   * server has to drop events for a slow subscriber, it sends the full state
   * again rather than letting the client build on an incomplete history.
   */
  itemChanged: ItemChangeEvent;
  /**
   * Subscribe to workspace git status changes.
   * Emits the status of all workspaces (main repo + worktrees) periodically.
   * The main workspace uses ID "__central__".
   */
  workspaceStatuses: Array<WorkspaceStatus>;
  /**
   * Subscribe to worktree changes. Emits the full list of active worktrees
   * whenever a worktree is created or removed.
   */
  worktreesChanged: Array<Worktree>;
};


export type SubscriptionAgentSessionChangedArgs = {
  itemId: Scalars['ID']['input'];
};


export type SubscriptionItemChangedArgs = {
  includeInitial?: InputMaybe<Scalars['Boolean']['input']>;
};

/** Input for updating an existing item */
export type UpdateItemInput = {
  /** Add items to blocked-by list (validates cycles and existence) */
  addBlockedBy?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Add items to blocking list (validates cycles and existence) */
  addBlocking?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Add tags to existing list */
  addTags?: InputMaybe<Array<Scalars['String']['input']>>;
  /** New body content (full replacement, mutually exclusive with bodyMod) */
  body?: InputMaybe<Scalars['String']['input']>;
  /** Structured body modifications (mutually exclusive with body) */
  bodyMod?: InputMaybe<BodyModification>;
  /** ETag for optimistic concurrency control (optional) */
  ifMatch?: InputMaybe<Scalars['String']['input']>;
  /** Fractional index for manual ordering (used by board drag-and-drop) */
  order?: InputMaybe<Scalars['String']['input']>;
  /** Set parent item ID (null/empty to clear, validates type hierarchy) */
  parent?: InputMaybe<Scalars['String']['input']>;
  /** New priority */
  priority?: InputMaybe<Scalars['String']['input']>;
  /** Remove items from blocked-by list */
  removeBlockedBy?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Remove items from blocking list */
  removeBlocking?: InputMaybe<Array<Scalars['String']['input']>>;
  /** Remove tags from existing list */
  removeTags?: InputMaybe<Array<Scalars['String']['input']>>;
  /** New status */
  status?: InputMaybe<Scalars['String']['input']>;
  /** Replace all tags (nil preserves existing, mutually exclusive with addTags/removeTags) */
  tags?: InputMaybe<Array<Scalars['String']['input']>>;
  /** New title */
  title?: InputMaybe<Scalars['String']['input']>;
  /** New type */
  type?: InputMaybe<Scalars['String']['input']>;
};

/** Git status for a workspace (main repo or worktree) */
export type WorkspaceStatus = {
  /** Whether the workspace has uncommitted changes or untracked files */
  hasChanges: Scalars['Boolean']['output'];
  /** Whether the workspace has commits not yet merged into the base branch */
  hasUnmergedCommits: Scalars['Boolean']['output'];
  /** Workspace identifier (__central__ for main repo, worktree ID for worktrees) */
  id: Scalars['ID']['output'];
};

/** A git worktree, either associated with an item or standalone */
export type Worktree = {
  /** Git branch name */
  branch: Scalars['String']['output'];
  /** Number of commits on the base branch that are not in this worktree branch */
  commitsBehind: Scalars['Int']['output'];
  /** Auto-generated summary of what this workspace is doing */
  description?: Maybe<Scalars['String']['output']>;
  /** Whether the worktree has uncommitted changes or untracked files */
  hasChanges: Scalars['Boolean']['output'];
  /** Whether rebasing onto the base branch would produce merge conflicts */
  hasConflicts: Scalars['Boolean']['output'];
  /** Whether the worktree has commits not yet merged into the base branch */
  hasUnmergedCommits: Scalars['Boolean']['output'];
  /** Unique worktree identifier */
  id: Scalars['ID']['output'];
  /** Items detected from changes in this worktree vs the base branch */
  items: Array<Item>;
  /** Human-readable name */
  name?: Maybe<Scalars['String']['output']>;
  /** Filesystem path to the worktree */
  path: Scalars['String']['output'];
  /** Open pull/merge request for this worktree's branch (null if none) */
  pullRequest?: Maybe<PullRequest>;
  /** Error message if setup failed */
  setupError?: Maybe<Scalars['String']['output']>;
  /** Post-creation setup status (null if no setup configured) */
  setupStatus?: Maybe<WorktreeSetupStatus>;
};

/** Status of a worktree's post-creation setup command */
export enum WorktreeSetupStatus {
  Done = 'DONE',
  Failed = 'FAILED',
  Running = 'RUNNING'
}

export type ItemFieldsFragment = { id: string, slug?: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId?: string | null, blockingIds: Array<string>, worktreeId?: string | null };

export type WorktreeFieldsFragment = { id: string, name?: string | null, description?: string | null, branch: string, path: string, setupStatus?: WorktreeSetupStatus | null, setupError?: string | null, items: Array<{ id: string }>, pullRequest?: { number: number, title: string, state: string, url: string, isDraft: boolean, checkStatus: string, reviewApproved: boolean, mergeable: boolean } | null };

export type AgentSessionFieldsFragment = { itemId: string, agentType: string, status: AgentSessionStatus, error?: string | null, effort?: string | null, planMode: boolean, actMode: boolean, systemStatus?: string | null, workDir?: string | null, quickReplies: Array<string>, messages: Array<{ role: AgentMessageRole, content: string, attachments: Array<string>, diff?: string | null, images: Array<{ url: string, mediaType: string }> }>, pendingInteraction?: { type: InteractionType, planContent?: string | null, questions?: Array<{ header: string, question: string, multiSelect: boolean, options: Array<{ label: string, description: string }> }> | null } | null, subagentActivities: Array<{ taskId: string, index: number, description: string, currentTool: string }> };

export type FileChangeFieldsFragment = { path: string, status: string, additions: number, deletions: number, staged: boolean };

export type AgentActionFieldsFragment = { id: string, label: string, description?: string | null, disabled: boolean, disabledReason?: string | null };

export type ItemChangedSubscriptionVariables = Exact<{
  includeInitial: Scalars['Boolean']['input'];
}>;


export type ItemChangedSubscription = { itemChanged: { type: ChangeType, itemId: string, item?: { id: string, slug?: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId?: string | null, blockingIds: Array<string>, worktreeId?: string | null } | null, items?: Array<{ id: string, slug?: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId?: string | null, blockingIds: Array<string>, worktreeId?: string | null }> | null } };

export type WorktreesChangedSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type WorktreesChangedSubscription = { worktreesChanged: Array<{ id: string, name?: string | null, description?: string | null, branch: string, path: string, setupStatus?: WorktreeSetupStatus | null, setupError?: string | null, items: Array<{ id: string }>, pullRequest?: { number: number, title: string, state: string, url: string, isDraft: boolean, checkStatus: string, reviewApproved: boolean, mergeable: boolean } | null }> };

export type AgentSessionChangedSubscriptionVariables = Exact<{
  itemId: Scalars['ID']['input'];
}>;


export type AgentSessionChangedSubscription = { agentSessionChanged: { itemId: string, agentType: string, status: AgentSessionStatus, error?: string | null, effort?: string | null, planMode: boolean, actMode: boolean, systemStatus?: string | null, workDir?: string | null, quickReplies: Array<string>, messages: Array<{ role: AgentMessageRole, content: string, attachments: Array<string>, diff?: string | null, images: Array<{ url: string, mediaType: string }> }>, pendingInteraction?: { type: InteractionType, planContent?: string | null, questions?: Array<{ header: string, question: string, multiSelect: boolean, options: Array<{ label: string, description: string }> }> | null } | null, subagentActivities: Array<{ taskId: string, index: number, description: string, currentTool: string }> } };

export type ActiveAgentStatusesSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type ActiveAgentStatusesSubscription = { activeAgentStatuses: Array<{ itemId: string, status: AgentSessionStatus }> };

export type WorkspaceStatusesSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type WorkspaceStatusesSubscription = { workspaceStatuses: Array<{ id: string, hasChanges: boolean, hasUnmergedCommits: boolean }> };

export type ConfigQueryVariables = Exact<{ [key: string]: never; }>;


export type ConfigQuery = { projectName: string, mainBranch: string, agentEnabled: boolean, worktreeBaseRef: string, worktreeRunCommand: string, worktreeIntegrateMode: string };

export type WorktreesQueryVariables = Exact<{ [key: string]: never; }>;


export type WorktreesQuery = { worktrees: Array<{ id: string, hasChanges: boolean, hasUnmergedCommits: boolean }> };

export type FileChangesQueryVariables = Exact<{
  path?: InputMaybe<Scalars['String']['input']>;
}>;


export type FileChangesQuery = { fileChanges: Array<{ path: string, status: string, additions: number, deletions: number, staged: boolean }> };

export type AllFileChangesQueryVariables = Exact<{
  path?: InputMaybe<Scalars['String']['input']>;
}>;


export type AllFileChangesQuery = { allFileChanges: Array<{ path: string, status: string, additions: number, deletions: number, staged: boolean }> };

export type BranchStatusQueryVariables = Exact<{
  path?: InputMaybe<Scalars['String']['input']>;
}>;


export type BranchStatusQuery = { branchStatus: { commitsBehind: number, hasConflicts: boolean } };

export type AgentActionsQueryVariables = Exact<{
  itemId: Scalars['ID']['input'];
  skipForge?: InputMaybe<Scalars['Boolean']['input']>;
}>;


export type AgentActionsQuery = { agentActions: Array<{ id: string, label: string, description?: string | null, disabled: boolean, disabledReason?: string | null }> };

export type FileDiffQueryVariables = Exact<{
  filePath: Scalars['String']['input'];
  staged: Scalars['Boolean']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
}>;


export type FileDiffQuery = { fileDiff: string };

export type AllFileDiffQueryVariables = Exact<{
  filePath: Scalars['String']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
}>;


export type AllFileDiffQuery = { allFileDiff: string };

export type CreateItemMutationVariables = Exact<{
  input: CreateItemInput;
}>;


export type CreateItemMutation = { createItem: { id: string, slug?: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId?: string | null, blockingIds: Array<string>, worktreeId?: string | null } };

export type UpdateItemMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  input: UpdateItemInput;
}>;


export type UpdateItemMutation = { updateItem: { id: string, slug?: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId?: string | null, blockingIds: Array<string>, worktreeId?: string | null } };

export type UpdateItemStatusMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  input: UpdateItemInput;
}>;


export type UpdateItemStatusMutation = { updateItem: { id: string, status: string } };

export type UpdateItemOrderMutationVariables = Exact<{
  id: Scalars['ID']['input'];
  input: UpdateItemInput;
}>;


export type UpdateItemOrderMutation = { updateItem: { id: string, status: string, order: string, parentId?: string | null } };

export type DeleteItemMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type DeleteItemMutation = { deleteItem: boolean };

export type ArchiveItemMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type ArchiveItemMutation = { archiveItem: boolean };

export type CreateWorktreeMutationVariables = Exact<{
  name: Scalars['String']['input'];
}>;


export type CreateWorktreeMutation = { createWorktree: { id: string, name?: string | null, description?: string | null, branch: string, path: string, setupStatus?: WorktreeSetupStatus | null, setupError?: string | null, items: Array<{ id: string }>, pullRequest?: { number: number, title: string, state: string, url: string, isDraft: boolean, checkStatus: string, reviewApproved: boolean, mergeable: boolean } | null } };

export type RemoveWorktreeMutationVariables = Exact<{
  id: Scalars['ID']['input'];
}>;


export type RemoveWorktreeMutation = { removeWorktree: boolean };

export type SendAgentMessageMutationVariables = Exact<{
  itemId: Scalars['ID']['input'];
  message: Scalars['String']['input'];
  images?: InputMaybe<Array<ImageInput> | ImageInput>;
  attachments?: InputMaybe<Array<FileAttachmentInput> | FileAttachmentInput>;
}>;


export type SendAgentMessageMutation = { sendAgentMessage: boolean };

export type ListFilesQueryVariables = Exact<{
  workspaceId?: InputMaybe<Scalars['ID']['input']>;
  prefix: Scalars['String']['input'];
  limit?: InputMaybe<Scalars['Int']['input']>;
}>;


export type ListFilesQuery = { listFiles: Array<{ path: string }> };

export type StopAgentMutationVariables = Exact<{
  itemId: Scalars['ID']['input'];
}>;


export type StopAgentMutation = { stopAgent: boolean };

export type SetAgentPlanModeMutationVariables = Exact<{
  itemId: Scalars['ID']['input'];
  planMode: Scalars['Boolean']['input'];
}>;


export type SetAgentPlanModeMutation = { setAgentPlanMode: boolean };

export type SetAgentActModeMutationVariables = Exact<{
  itemId: Scalars['ID']['input'];
  actMode: Scalars['Boolean']['input'];
}>;


export type SetAgentActModeMutation = { setAgentActMode: boolean };

export type SetAgentEffortMutationVariables = Exact<{
  itemId: Scalars['ID']['input'];
  effort: Scalars['String']['input'];
}>;


export type SetAgentEffortMutation = { setAgentEffort: boolean };

export type ClearAgentSessionMutationVariables = Exact<{
  itemId: Scalars['ID']['input'];
}>;


export type ClearAgentSessionMutation = { clearAgentSession: boolean };

export type ExecuteAgentActionMutationVariables = Exact<{
  itemId: Scalars['ID']['input'];
  actionId: Scalars['ID']['input'];
}>;


export type ExecuteAgentActionMutation = { executeAgentAction: boolean };

export type WriteTerminalInputMutationVariables = Exact<{
  sessionId: Scalars['String']['input'];
  data: Scalars['String']['input'];
}>;


export type WriteTerminalInputMutation = { writeTerminalInput: boolean };

export type DiscardFileChangeMutationVariables = Exact<{
  filePath: Scalars['String']['input'];
  staged: Scalars['Boolean']['input'];
  path?: InputMaybe<Scalars['String']['input']>;
}>;


export type DiscardFileChangeMutation = { discardFileChange: boolean };

export type OpenInEditorMutationVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
}>;


export type OpenInEditorMutation = { openInEditor: boolean };

export type StartRunMutationVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
}>;


export type StartRunMutation = { startRun: number };

export type StopRunMutationVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
}>;


export type StopRunMutation = { stopRun: boolean };

export type IsRunningQueryVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
}>;


export type IsRunningQuery = { isRunning: boolean };

export type WorkspacePortQueryVariables = Exact<{
  workspaceId: Scalars['ID']['input'];
}>;


export type WorkspacePortQuery = { workspacePort: number };

export const ItemFieldsFragmentDoc = {"kind":"Document","definitions":[{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ItemFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Item"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"slug"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"priority"}},{"kind":"Field","name":{"kind":"Name","value":"tags"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"body"}},{"kind":"Field","name":{"kind":"Name","value":"order"}},{"kind":"Field","name":{"kind":"Name","value":"parentId"}},{"kind":"Field","name":{"kind":"Name","value":"blockingIds"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeId"}}]}}]} as unknown as DocumentNode<ItemFieldsFragment, unknown>;
export const WorktreeFieldsFragmentDoc = {"kind":"Document","definitions":[{"kind":"FragmentDefinition","name":{"kind":"Name","value":"WorktreeFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Worktree"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"description"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"items"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}}]}},{"kind":"Field","name":{"kind":"Name","value":"setupStatus"}},{"kind":"Field","name":{"kind":"Name","value":"setupError"}},{"kind":"Field","name":{"kind":"Name","value":"pullRequest"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"number"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"state"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"isDraft"}},{"kind":"Field","name":{"kind":"Name","value":"checkStatus"}},{"kind":"Field","name":{"kind":"Name","value":"reviewApproved"}},{"kind":"Field","name":{"kind":"Name","value":"mergeable"}}]}}]}}]} as unknown as DocumentNode<WorktreeFieldsFragment, unknown>;
export const AgentSessionFieldsFragmentDoc = {"kind":"Document","definitions":[{"kind":"FragmentDefinition","name":{"kind":"Name","value":"AgentSessionFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"AgentSession"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"itemId"}},{"kind":"Field","name":{"kind":"Name","value":"agentType"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"images"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"mediaType"}}]}},{"kind":"Field","name":{"kind":"Name","value":"attachments"}},{"kind":"Field","name":{"kind":"Name","value":"diff"}}]}},{"kind":"Field","name":{"kind":"Name","value":"error"}},{"kind":"Field","name":{"kind":"Name","value":"effort"}},{"kind":"Field","name":{"kind":"Name","value":"planMode"}},{"kind":"Field","name":{"kind":"Name","value":"actMode"}},{"kind":"Field","name":{"kind":"Name","value":"systemStatus"}},{"kind":"Field","name":{"kind":"Name","value":"pendingInteraction"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"planContent"}},{"kind":"Field","name":{"kind":"Name","value":"questions"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"header"}},{"kind":"Field","name":{"kind":"Name","value":"question"}},{"kind":"Field","name":{"kind":"Name","value":"multiSelect"}},{"kind":"Field","name":{"kind":"Name","value":"options"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"label"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]}},{"kind":"Field","name":{"kind":"Name","value":"workDir"}},{"kind":"Field","name":{"kind":"Name","value":"subagentActivities"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"taskId"}},{"kind":"Field","name":{"kind":"Name","value":"index"}},{"kind":"Field","name":{"kind":"Name","value":"description"}},{"kind":"Field","name":{"kind":"Name","value":"currentTool"}}]}},{"kind":"Field","name":{"kind":"Name","value":"quickReplies"}}]}}]} as unknown as DocumentNode<AgentSessionFieldsFragment, unknown>;
export const FileChangeFieldsFragmentDoc = {"kind":"Document","definitions":[{"kind":"FragmentDefinition","name":{"kind":"Name","value":"FileChangeFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"FileChange"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"additions"}},{"kind":"Field","name":{"kind":"Name","value":"deletions"}},{"kind":"Field","name":{"kind":"Name","value":"staged"}}]}}]} as unknown as DocumentNode<FileChangeFieldsFragment, unknown>;
export const AgentActionFieldsFragmentDoc = {"kind":"Document","definitions":[{"kind":"FragmentDefinition","name":{"kind":"Name","value":"AgentActionFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"AgentAction"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"label"}},{"kind":"Field","name":{"kind":"Name","value":"description"}},{"kind":"Field","name":{"kind":"Name","value":"disabled"}},{"kind":"Field","name":{"kind":"Name","value":"disabledReason"}}]}}]} as unknown as DocumentNode<AgentActionFieldsFragment, unknown>;
export const ItemChangedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"ItemChanged"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"includeInitial"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"itemChanged"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"includeInitial"},"value":{"kind":"Variable","name":{"kind":"Name","value":"includeInitial"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"itemId"}},{"kind":"Field","name":{"kind":"Name","value":"item"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ItemFields"}}]}},{"kind":"Field","name":{"kind":"Name","value":"items"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ItemFields"}}]}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ItemFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Item"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"slug"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"priority"}},{"kind":"Field","name":{"kind":"Name","value":"tags"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"body"}},{"kind":"Field","name":{"kind":"Name","value":"order"}},{"kind":"Field","name":{"kind":"Name","value":"parentId"}},{"kind":"Field","name":{"kind":"Name","value":"blockingIds"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeId"}}]}}]} as unknown as DocumentNode<ItemChangedSubscription, ItemChangedSubscriptionVariables>;
export const WorktreesChangedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"WorktreesChanged"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"worktreesChanged"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"WorktreeFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"WorktreeFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Worktree"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"description"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"items"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}}]}},{"kind":"Field","name":{"kind":"Name","value":"setupStatus"}},{"kind":"Field","name":{"kind":"Name","value":"setupError"}},{"kind":"Field","name":{"kind":"Name","value":"pullRequest"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"number"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"state"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"isDraft"}},{"kind":"Field","name":{"kind":"Name","value":"checkStatus"}},{"kind":"Field","name":{"kind":"Name","value":"reviewApproved"}},{"kind":"Field","name":{"kind":"Name","value":"mergeable"}}]}}]}}]} as unknown as DocumentNode<WorktreesChangedSubscription, WorktreesChangedSubscriptionVariables>;
export const AgentSessionChangedDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"AgentSessionChanged"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"agentSessionChanged"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"AgentSessionFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"AgentSessionFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"AgentSession"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"itemId"}},{"kind":"Field","name":{"kind":"Name","value":"agentType"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"messages"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"role"}},{"kind":"Field","name":{"kind":"Name","value":"content"}},{"kind":"Field","name":{"kind":"Name","value":"images"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"mediaType"}}]}},{"kind":"Field","name":{"kind":"Name","value":"attachments"}},{"kind":"Field","name":{"kind":"Name","value":"diff"}}]}},{"kind":"Field","name":{"kind":"Name","value":"error"}},{"kind":"Field","name":{"kind":"Name","value":"effort"}},{"kind":"Field","name":{"kind":"Name","value":"planMode"}},{"kind":"Field","name":{"kind":"Name","value":"actMode"}},{"kind":"Field","name":{"kind":"Name","value":"systemStatus"}},{"kind":"Field","name":{"kind":"Name","value":"pendingInteraction"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"planContent"}},{"kind":"Field","name":{"kind":"Name","value":"questions"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"header"}},{"kind":"Field","name":{"kind":"Name","value":"question"}},{"kind":"Field","name":{"kind":"Name","value":"multiSelect"}},{"kind":"Field","name":{"kind":"Name","value":"options"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"label"}},{"kind":"Field","name":{"kind":"Name","value":"description"}}]}}]}}]}},{"kind":"Field","name":{"kind":"Name","value":"workDir"}},{"kind":"Field","name":{"kind":"Name","value":"subagentActivities"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"taskId"}},{"kind":"Field","name":{"kind":"Name","value":"index"}},{"kind":"Field","name":{"kind":"Name","value":"description"}},{"kind":"Field","name":{"kind":"Name","value":"currentTool"}}]}},{"kind":"Field","name":{"kind":"Name","value":"quickReplies"}}]}}]} as unknown as DocumentNode<AgentSessionChangedSubscription, AgentSessionChangedSubscriptionVariables>;
export const ActiveAgentStatusesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"ActiveAgentStatuses"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"activeAgentStatuses"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"itemId"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<ActiveAgentStatusesSubscription, ActiveAgentStatusesSubscriptionVariables>;
export const WorkspaceStatusesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"subscription","name":{"kind":"Name","value":"WorkspaceStatuses"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspaceStatuses"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"hasChanges"}},{"kind":"Field","name":{"kind":"Name","value":"hasUnmergedCommits"}}]}}]}}]} as unknown as DocumentNode<WorkspaceStatusesSubscription, WorkspaceStatusesSubscriptionVariables>;
export const ConfigDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Config"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"projectName"}},{"kind":"Field","name":{"kind":"Name","value":"mainBranch"}},{"kind":"Field","name":{"kind":"Name","value":"agentEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeBaseRef"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeRunCommand"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeIntegrateMode"}}]}}]} as unknown as DocumentNode<ConfigQuery, ConfigQueryVariables>;
export const WorktreesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Worktrees"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"worktrees"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"hasChanges"}},{"kind":"Field","name":{"kind":"Name","value":"hasUnmergedCommits"}}]}}]}}]} as unknown as DocumentNode<WorktreesQuery, WorktreesQueryVariables>;
export const FileChangesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"FileChanges"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"fileChanges"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"FileChangeFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"FileChangeFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"FileChange"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"additions"}},{"kind":"Field","name":{"kind":"Name","value":"deletions"}},{"kind":"Field","name":{"kind":"Name","value":"staged"}}]}}]} as unknown as DocumentNode<FileChangesQuery, FileChangesQueryVariables>;
export const AllFileChangesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"AllFileChanges"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"allFileChanges"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"FileChangeFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"FileChangeFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"FileChange"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"additions"}},{"kind":"Field","name":{"kind":"Name","value":"deletions"}},{"kind":"Field","name":{"kind":"Name","value":"staged"}}]}}]} as unknown as DocumentNode<AllFileChangesQuery, AllFileChangesQueryVariables>;
export const BranchStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"BranchStatus"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"branchStatus"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"commitsBehind"}},{"kind":"Field","name":{"kind":"Name","value":"hasConflicts"}}]}}]}}]} as unknown as DocumentNode<BranchStatusQuery, BranchStatusQueryVariables>;
export const AgentActionsDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"AgentActions"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"skipForge"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"agentActions"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}},{"kind":"Argument","name":{"kind":"Name","value":"skipForge"},"value":{"kind":"Variable","name":{"kind":"Name","value":"skipForge"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"AgentActionFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"AgentActionFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"AgentAction"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"label"}},{"kind":"Field","name":{"kind":"Name","value":"description"}},{"kind":"Field","name":{"kind":"Name","value":"disabled"}},{"kind":"Field","name":{"kind":"Name","value":"disabledReason"}}]}}]} as unknown as DocumentNode<AgentActionsQuery, AgentActionsQueryVariables>;
export const FileDiffDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"FileDiff"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"filePath"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"staged"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"fileDiff"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"filePath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"filePath"}}},{"kind":"Argument","name":{"kind":"Name","value":"staged"},"value":{"kind":"Variable","name":{"kind":"Name","value":"staged"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}]}]}}]} as unknown as DocumentNode<FileDiffQuery, FileDiffQueryVariables>;
export const AllFileDiffDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"AllFileDiff"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"filePath"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"allFileDiff"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"filePath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"filePath"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}]}]}}]} as unknown as DocumentNode<AllFileDiffQuery, AllFileDiffQueryVariables>;
export const CreateItemDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateItem"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"CreateItemInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createItem"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ItemFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ItemFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Item"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"slug"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"priority"}},{"kind":"Field","name":{"kind":"Name","value":"tags"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"body"}},{"kind":"Field","name":{"kind":"Name","value":"order"}},{"kind":"Field","name":{"kind":"Name","value":"parentId"}},{"kind":"Field","name":{"kind":"Name","value":"blockingIds"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeId"}}]}}]} as unknown as DocumentNode<CreateItemMutation, CreateItemMutationVariables>;
export const UpdateItemDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateItem"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"UpdateItemInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateItem"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"ItemFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"ItemFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Item"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"slug"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"type"}},{"kind":"Field","name":{"kind":"Name","value":"priority"}},{"kind":"Field","name":{"kind":"Name","value":"tags"}},{"kind":"Field","name":{"kind":"Name","value":"createdAt"}},{"kind":"Field","name":{"kind":"Name","value":"updatedAt"}},{"kind":"Field","name":{"kind":"Name","value":"body"}},{"kind":"Field","name":{"kind":"Name","value":"order"}},{"kind":"Field","name":{"kind":"Name","value":"parentId"}},{"kind":"Field","name":{"kind":"Name","value":"blockingIds"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeId"}}]}}]} as unknown as DocumentNode<UpdateItemMutation, UpdateItemMutationVariables>;
export const UpdateItemStatusDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateItemStatus"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"UpdateItemInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateItem"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}}]}}]}}]} as unknown as DocumentNode<UpdateItemStatusMutation, UpdateItemStatusMutationVariables>;
export const UpdateItemOrderDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"UpdateItemOrder"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"input"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"UpdateItemInput"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"updateItem"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}},{"kind":"Argument","name":{"kind":"Name","value":"input"},"value":{"kind":"Variable","name":{"kind":"Name","value":"input"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"status"}},{"kind":"Field","name":{"kind":"Name","value":"order"}},{"kind":"Field","name":{"kind":"Name","value":"parentId"}}]}}]}}]} as unknown as DocumentNode<UpdateItemOrderMutation, UpdateItemOrderMutationVariables>;
export const DeleteItemDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DeleteItem"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"deleteItem"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<DeleteItemMutation, DeleteItemMutationVariables>;
export const ArchiveItemDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ArchiveItem"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"archiveItem"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<ArchiveItemMutation, ArchiveItemMutationVariables>;
export const CreateWorktreeDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"CreateWorktree"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"name"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"createWorktree"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"name"},"value":{"kind":"Variable","name":{"kind":"Name","value":"name"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"FragmentSpread","name":{"kind":"Name","value":"WorktreeFields"}}]}}]}},{"kind":"FragmentDefinition","name":{"kind":"Name","value":"WorktreeFields"},"typeCondition":{"kind":"NamedType","name":{"kind":"Name","value":"Worktree"}},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}},{"kind":"Field","name":{"kind":"Name","value":"name"}},{"kind":"Field","name":{"kind":"Name","value":"description"}},{"kind":"Field","name":{"kind":"Name","value":"branch"}},{"kind":"Field","name":{"kind":"Name","value":"path"}},{"kind":"Field","name":{"kind":"Name","value":"items"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"id"}}]}},{"kind":"Field","name":{"kind":"Name","value":"setupStatus"}},{"kind":"Field","name":{"kind":"Name","value":"setupError"}},{"kind":"Field","name":{"kind":"Name","value":"pullRequest"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"number"}},{"kind":"Field","name":{"kind":"Name","value":"title"}},{"kind":"Field","name":{"kind":"Name","value":"state"}},{"kind":"Field","name":{"kind":"Name","value":"url"}},{"kind":"Field","name":{"kind":"Name","value":"isDraft"}},{"kind":"Field","name":{"kind":"Name","value":"checkStatus"}},{"kind":"Field","name":{"kind":"Name","value":"reviewApproved"}},{"kind":"Field","name":{"kind":"Name","value":"mergeable"}}]}}]}}]} as unknown as DocumentNode<CreateWorktreeMutation, CreateWorktreeMutationVariables>;
export const RemoveWorktreeDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"RemoveWorktree"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"id"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"removeWorktree"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"id"},"value":{"kind":"Variable","name":{"kind":"Name","value":"id"}}}]}]}}]} as unknown as DocumentNode<RemoveWorktreeMutation, RemoveWorktreeMutationVariables>;
export const SendAgentMessageDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SendAgentMessage"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"message"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"images"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ImageInput"}}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"attachments"}},"type":{"kind":"ListType","type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"FileAttachmentInput"}}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"sendAgentMessage"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}},{"kind":"Argument","name":{"kind":"Name","value":"message"},"value":{"kind":"Variable","name":{"kind":"Name","value":"message"}}},{"kind":"Argument","name":{"kind":"Name","value":"images"},"value":{"kind":"Variable","name":{"kind":"Name","value":"images"}}},{"kind":"Argument","name":{"kind":"Name","value":"attachments"},"value":{"kind":"Variable","name":{"kind":"Name","value":"attachments"}}}]}]}}]} as unknown as DocumentNode<SendAgentMessageMutation, SendAgentMessageMutationVariables>;
export const ListFilesDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"ListFiles"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"prefix"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"limit"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"Int"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"listFiles"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}},{"kind":"Argument","name":{"kind":"Name","value":"prefix"},"value":{"kind":"Variable","name":{"kind":"Name","value":"prefix"}}},{"kind":"Argument","name":{"kind":"Name","value":"limit"},"value":{"kind":"Variable","name":{"kind":"Name","value":"limit"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"path"}}]}}]}}]} as unknown as DocumentNode<ListFilesQuery, ListFilesQueryVariables>;
export const StopAgentDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"StopAgent"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"stopAgent"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}}]}]}}]} as unknown as DocumentNode<StopAgentMutation, StopAgentMutationVariables>;
export const SetAgentPlanModeDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetAgentPlanMode"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"planMode"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setAgentPlanMode"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}},{"kind":"Argument","name":{"kind":"Name","value":"planMode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"planMode"}}}]}]}}]} as unknown as DocumentNode<SetAgentPlanModeMutation, SetAgentPlanModeMutationVariables>;
export const SetAgentActModeDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetAgentActMode"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"actMode"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setAgentActMode"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}},{"kind":"Argument","name":{"kind":"Name","value":"actMode"},"value":{"kind":"Variable","name":{"kind":"Name","value":"actMode"}}}]}]}}]} as unknown as DocumentNode<SetAgentActModeMutation, SetAgentActModeMutationVariables>;
export const SetAgentEffortDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"SetAgentEffort"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"effort"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"setAgentEffort"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}},{"kind":"Argument","name":{"kind":"Name","value":"effort"},"value":{"kind":"Variable","name":{"kind":"Name","value":"effort"}}}]}]}}]} as unknown as DocumentNode<SetAgentEffortMutation, SetAgentEffortMutationVariables>;
export const ClearAgentSessionDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ClearAgentSession"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"clearAgentSession"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}}]}]}}]} as unknown as DocumentNode<ClearAgentSessionMutation, ClearAgentSessionMutationVariables>;
export const ExecuteAgentActionDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"ExecuteAgentAction"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"actionId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"executeAgentAction"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"itemId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"itemId"}}},{"kind":"Argument","name":{"kind":"Name","value":"actionId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"actionId"}}}]}]}}]} as unknown as DocumentNode<ExecuteAgentActionMutation, ExecuteAgentActionMutationVariables>;
export const WriteTerminalInputDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"WriteTerminalInput"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"sessionId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"data"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"writeTerminalInput"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"sessionId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"sessionId"}}},{"kind":"Argument","name":{"kind":"Name","value":"data"},"value":{"kind":"Variable","name":{"kind":"Name","value":"data"}}}]}]}}]} as unknown as DocumentNode<WriteTerminalInputMutation, WriteTerminalInputMutationVariables>;
export const DiscardFileChangeDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"DiscardFileChange"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"filePath"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"staged"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"Boolean"}}}},{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"path"}},"type":{"kind":"NamedType","name":{"kind":"Name","value":"String"}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"discardFileChange"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"filePath"},"value":{"kind":"Variable","name":{"kind":"Name","value":"filePath"}}},{"kind":"Argument","name":{"kind":"Name","value":"staged"},"value":{"kind":"Variable","name":{"kind":"Name","value":"staged"}}},{"kind":"Argument","name":{"kind":"Name","value":"path"},"value":{"kind":"Variable","name":{"kind":"Name","value":"path"}}}]}]}}]} as unknown as DocumentNode<DiscardFileChangeMutation, DiscardFileChangeMutationVariables>;
export const OpenInEditorDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"OpenInEditor"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"openInEditor"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}]}]}}]} as unknown as DocumentNode<OpenInEditorMutation, OpenInEditorMutationVariables>;
export const StartRunDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"StartRun"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"startRun"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}]}]}}]} as unknown as DocumentNode<StartRunMutation, StartRunMutationVariables>;
export const StopRunDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"mutation","name":{"kind":"Name","value":"StopRun"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"stopRun"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}]}]}}]} as unknown as DocumentNode<StopRunMutation, StopRunMutationVariables>;
export const IsRunningDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"IsRunning"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"isRunning"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}]}]}}]} as unknown as DocumentNode<IsRunningQuery, IsRunningQueryVariables>;
export const WorkspacePortDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"WorkspacePort"},"variableDefinitions":[{"kind":"VariableDefinition","variable":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}},"type":{"kind":"NonNullType","type":{"kind":"NamedType","name":{"kind":"Name","value":"ID"}}}}],"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"workspacePort"},"arguments":[{"kind":"Argument","name":{"kind":"Name","value":"workspaceId"},"value":{"kind":"Variable","name":{"kind":"Name","value":"workspaceId"}}}]}]}}]} as unknown as DocumentNode<WorkspacePortQuery, WorkspacePortQueryVariables>;