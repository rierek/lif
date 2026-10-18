/** Internal type. DO NOT USE DIRECTLY. */
type Exact<T extends { [key: string]: unknown }> = { [K in keyof T]: T[K] };
/** Internal type. DO NOT USE DIRECTLY. */
export type Incremental<T> = T | { [P in keyof T]?: P extends ' $fragmentName' | '__typename' ? T[P] : never };
import type { TypedDocumentNode as DocumentNode } from 'urql';

/** Role of an agent message sender */
export enum AgentMessageRole {
  Assistant = 'ASSISTANT',
  Info = 'INFO',
  Tool = 'TOOL',
  User = 'USER'
}

/** Status of an agent session */
export enum AgentSessionStatus {
  Error = 'ERROR',
  Idle = 'IDLE',
  Running = 'RUNNING'
}

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
  append?: string | null | undefined;
  /**
   * Text replacements applied sequentially in array order.
   * Each old text must match exactly once at the time it's applied.
   */
  replace?: Array<ReplaceOperation> | null | undefined;
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
  blockedBy?: Array<string> | null | undefined;
  /** Item IDs this item is blocking */
  blocking?: Array<string> | null | undefined;
  /** Markdown body content */
  body?: string | null | undefined;
  /** Parent item ID (validated against type hierarchy) */
  parent?: string | null | undefined;
  /** Custom ID prefix (overrides config prefix for this item) */
  prefix?: string | null | undefined;
  /** Priority level (defaults to 'normal') */
  priority?: string | null | undefined;
  /** Status (defaults to 'todo') */
  status?: string | null | undefined;
  /** Tags for categorization */
  tags?: Array<string> | null | undefined;
  /** Item title (required) */
  title: string;
  /** Item type (defaults to 'task') */
  type?: string | null | undefined;
};

/** Input for attaching a file or directory as context to an agent message. */
export type FileAttachmentInput = {
  /** Relative file or directory path */
  path: string;
};

/** Input for uploading an image attachment */
export type ImageInput = {
  /** Base64-encoded image data */
  data: string;
  /** MIME type (image/jpeg, image/png, image/gif, image/webp) */
  mediaType: string;
};

/** Type of blocking interaction */
export enum InteractionType {
  AskUser = 'ASK_USER',
  EnterPlan = 'ENTER_PLAN',
  ExitPlan = 'EXIT_PLAN'
}

/** A single text replacement operation. */
export type ReplaceOperation = {
  /** Replacement text (can be empty to delete the matched text) */
  new: string;
  /** Text to find (must occur exactly once, cannot be empty) */
  old: string;
};

/** Input for updating an existing item */
export type UpdateItemInput = {
  /** Add items to blocked-by list (validates cycles and existence) */
  addBlockedBy?: Array<string> | null | undefined;
  /** Add items to blocking list (validates cycles and existence) */
  addBlocking?: Array<string> | null | undefined;
  /** Add tags to existing list */
  addTags?: Array<string> | null | undefined;
  /** New body content (full replacement, mutually exclusive with bodyMod) */
  body?: string | null | undefined;
  /** Structured body modifications (mutually exclusive with body) */
  bodyMod?: BodyModification | null | undefined;
  /** ETag for optimistic concurrency control (optional) */
  ifMatch?: string | null | undefined;
  /** Fractional index for manual ordering (used by board drag-and-drop) */
  order?: string | null | undefined;
  /** Set parent item ID (null/empty to clear, validates type hierarchy) */
  parent?: string | null | undefined;
  /** New priority */
  priority?: string | null | undefined;
  /** Remove items from blocked-by list */
  removeBlockedBy?: Array<string> | null | undefined;
  /** Remove items from blocking list */
  removeBlocking?: Array<string> | null | undefined;
  /** Remove tags from existing list */
  removeTags?: Array<string> | null | undefined;
  /** New status */
  status?: string | null | undefined;
  /** Replace all tags (nil preserves existing, mutually exclusive with addTags/removeTags) */
  tags?: Array<string> | null | undefined;
  /** New title */
  title?: string | null | undefined;
  /** New type */
  type?: string | null | undefined;
};

/** Status of a worktree's post-creation setup command */
export enum WorktreeSetupStatus {
  Done = 'DONE',
  Failed = 'FAILED',
  Running = 'RUNNING'
}

export type ItemFieldsFragment = { id: string, slug: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId: string | null, blockingIds: Array<string>, worktreeId: string | null };

export type WorktreeFieldsFragment = { id: string, name: string | null, description: string | null, branch: string, path: string, setupStatus: WorktreeSetupStatus | null, setupError: string | null, items: Array<{ id: string }>, pullRequest: { number: number, title: string, state: string, url: string, isDraft: boolean, checkStatus: string, reviewApproved: boolean, mergeable: boolean } | null };

export type AgentSessionFieldsFragment = { itemId: string, agentType: string, status: AgentSessionStatus, error: string | null, effort: string | null, planMode: boolean, actMode: boolean, systemStatus: string | null, workDir: string | null, quickReplies: Array<string>, messages: Array<{ role: AgentMessageRole, content: string, attachments: Array<string>, diff: string | null, images: Array<{ url: string, mediaType: string }> }>, pendingInteraction: { type: InteractionType, planContent: string | null, questions: Array<{ header: string, question: string, multiSelect: boolean, options: Array<{ label: string, description: string }> }> | null } | null, subagentActivities: Array<{ taskId: string, index: number, description: string, currentTool: string }> };

export type FileChangeFieldsFragment = { path: string, status: string, additions: number, deletions: number, staged: boolean };

export type AgentActionFieldsFragment = { id: string, label: string, description: string | null, disabled: boolean, disabledReason: string | null };

export type ItemChangedSubscriptionVariables = Exact<{
  includeInitial: boolean;
}>;


export type ItemChangedSubscription = { itemChanged: { type: ChangeType, itemId: string, item: { id: string, slug: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId: string | null, blockingIds: Array<string>, worktreeId: string | null } | null, items: Array<{ id: string, slug: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId: string | null, blockingIds: Array<string>, worktreeId: string | null }> | null } };

export type WorktreesChangedSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type WorktreesChangedSubscription = { worktreesChanged: Array<{ id: string, name: string | null, description: string | null, branch: string, path: string, setupStatus: WorktreeSetupStatus | null, setupError: string | null, items: Array<{ id: string }>, pullRequest: { number: number, title: string, state: string, url: string, isDraft: boolean, checkStatus: string, reviewApproved: boolean, mergeable: boolean } | null }> };

export type AgentSessionChangedSubscriptionVariables = Exact<{
  itemId: string | number;
}>;


export type AgentSessionChangedSubscription = { agentSessionChanged: { itemId: string, agentType: string, status: AgentSessionStatus, error: string | null, effort: string | null, planMode: boolean, actMode: boolean, systemStatus: string | null, workDir: string | null, quickReplies: Array<string>, messages: Array<{ role: AgentMessageRole, content: string, attachments: Array<string>, diff: string | null, images: Array<{ url: string, mediaType: string }> }>, pendingInteraction: { type: InteractionType, planContent: string | null, questions: Array<{ header: string, question: string, multiSelect: boolean, options: Array<{ label: string, description: string }> }> | null } | null, subagentActivities: Array<{ taskId: string, index: number, description: string, currentTool: string }> } };

export type ActiveAgentStatusesSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type ActiveAgentStatusesSubscription = { activeAgentStatuses: Array<{ itemId: string, status: AgentSessionStatus }> };

export type WorkspaceStatusesSubscriptionVariables = Exact<{ [key: string]: never; }>;


export type WorkspaceStatusesSubscription = { workspaceStatuses: Array<{ id: string, hasChanges: boolean, hasUnmergedCommits: boolean }> };

export type ConfigQueryVariables = Exact<{ [key: string]: never; }>;


export type ConfigQuery = { projectName: string, mainBranch: string, agentEnabled: boolean, worktreeBaseRef: string, worktreeRunCommand: string, worktreeIntegrateMode: string, theme: string };

export type WorktreesQueryVariables = Exact<{ [key: string]: never; }>;


export type WorktreesQuery = { worktrees: Array<{ id: string, hasChanges: boolean, hasUnmergedCommits: boolean }> };

export type FileChangesQueryVariables = Exact<{
  path?: string | null | undefined;
}>;


export type FileChangesQuery = { fileChanges: Array<{ path: string, status: string, additions: number, deletions: number, staged: boolean }> };

export type AllFileChangesQueryVariables = Exact<{
  path?: string | null | undefined;
}>;


export type AllFileChangesQuery = { allFileChanges: Array<{ path: string, status: string, additions: number, deletions: number, staged: boolean }> };

export type BranchStatusQueryVariables = Exact<{
  path?: string | null | undefined;
}>;


export type BranchStatusQuery = { branchStatus: { commitsBehind: number, hasConflicts: boolean } };

export type AgentActionsQueryVariables = Exact<{
  itemId: string | number;
  skipForge?: boolean | null | undefined;
}>;


export type AgentActionsQuery = { agentActions: Array<{ id: string, label: string, description: string | null, disabled: boolean, disabledReason: string | null }> };

export type FileDiffQueryVariables = Exact<{
  filePath: string;
  staged: boolean;
  path?: string | null | undefined;
}>;


export type FileDiffQuery = { fileDiff: string };

export type AllFileDiffQueryVariables = Exact<{
  filePath: string;
  path?: string | null | undefined;
}>;


export type AllFileDiffQuery = { allFileDiff: string };

export type CreateItemMutationVariables = Exact<{
  input: CreateItemInput;
}>;


export type CreateItemMutation = { createItem: { id: string, slug: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId: string | null, blockingIds: Array<string>, worktreeId: string | null } };

export type UpdateItemMutationVariables = Exact<{
  id: string | number;
  input: UpdateItemInput;
}>;


export type UpdateItemMutation = { updateItem: { id: string, slug: string | null, path: string, title: string, status: string, type: string, priority: string, tags: Array<string>, createdAt: string, updatedAt: string, body: string, order: string, parentId: string | null, blockingIds: Array<string>, worktreeId: string | null } };

export type UpdateItemStatusMutationVariables = Exact<{
  id: string | number;
  input: UpdateItemInput;
}>;


export type UpdateItemStatusMutation = { updateItem: { id: string, status: string } };

export type UpdateItemOrderMutationVariables = Exact<{
  id: string | number;
  input: UpdateItemInput;
}>;


export type UpdateItemOrderMutation = { updateItem: { id: string, status: string, order: string, parentId: string | null } };

export type DeleteItemMutationVariables = Exact<{
  id: string | number;
}>;


export type DeleteItemMutation = { deleteItem: boolean };

export type ArchiveItemMutationVariables = Exact<{
  id: string | number;
}>;


export type ArchiveItemMutation = { archiveItem: boolean };

export type CreateWorktreeMutationVariables = Exact<{
  name: string;
}>;


export type CreateWorktreeMutation = { createWorktree: { id: string, name: string | null, description: string | null, branch: string, path: string, setupStatus: WorktreeSetupStatus | null, setupError: string | null, items: Array<{ id: string }>, pullRequest: { number: number, title: string, state: string, url: string, isDraft: boolean, checkStatus: string, reviewApproved: boolean, mergeable: boolean } | null } };

export type RemoveWorktreeMutationVariables = Exact<{
  id: string | number;
}>;


export type RemoveWorktreeMutation = { removeWorktree: boolean };

export type SendAgentMessageMutationVariables = Exact<{
  itemId: string | number;
  message: string;
  images?: Array<ImageInput> | ImageInput | null | undefined;
  attachments?: Array<FileAttachmentInput> | FileAttachmentInput | null | undefined;
}>;


export type SendAgentMessageMutation = { sendAgentMessage: boolean };

export type ListFilesQueryVariables = Exact<{
  workspaceId?: string | number | null | undefined;
  prefix: string;
  limit?: number | null | undefined;
}>;


export type ListFilesQuery = { listFiles: Array<{ path: string }> };

export type StopAgentMutationVariables = Exact<{
  itemId: string | number;
}>;


export type StopAgentMutation = { stopAgent: boolean };

export type SetAgentPlanModeMutationVariables = Exact<{
  itemId: string | number;
  planMode: boolean;
}>;


export type SetAgentPlanModeMutation = { setAgentPlanMode: boolean };

export type SetAgentActModeMutationVariables = Exact<{
  itemId: string | number;
  actMode: boolean;
}>;


export type SetAgentActModeMutation = { setAgentActMode: boolean };

export type SetAgentEffortMutationVariables = Exact<{
  itemId: string | number;
  effort: string;
}>;


export type SetAgentEffortMutation = { setAgentEffort: boolean };

export type ClearAgentSessionMutationVariables = Exact<{
  itemId: string | number;
}>;


export type ClearAgentSessionMutation = { clearAgentSession: boolean };

export type ExecuteAgentActionMutationVariables = Exact<{
  itemId: string | number;
  actionId: string | number;
}>;


export type ExecuteAgentActionMutation = { executeAgentAction: boolean };

export type WriteTerminalInputMutationVariables = Exact<{
  sessionId: string;
  data: string;
}>;


export type WriteTerminalInputMutation = { writeTerminalInput: boolean };

export type DiscardFileChangeMutationVariables = Exact<{
  filePath: string;
  staged: boolean;
  path?: string | null | undefined;
}>;


export type DiscardFileChangeMutation = { discardFileChange: boolean };

export type OpenInEditorMutationVariables = Exact<{
  workspaceId: string | number;
}>;


export type OpenInEditorMutation = { openInEditor: boolean };

export type StartRunMutationVariables = Exact<{
  workspaceId: string | number;
}>;


export type StartRunMutation = { startRun: number };

export type StopRunMutationVariables = Exact<{
  workspaceId: string | number;
}>;


export type StopRunMutation = { stopRun: boolean };

export type IsRunningQueryVariables = Exact<{
  workspaceId: string | number;
}>;


export type IsRunningQuery = { isRunning: boolean };

export type WorkspacePortQueryVariables = Exact<{
  workspaceId: string | number;
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
export const ConfigDocument = {"kind":"Document","definitions":[{"kind":"OperationDefinition","operation":"query","name":{"kind":"Name","value":"Config"},"selectionSet":{"kind":"SelectionSet","selections":[{"kind":"Field","name":{"kind":"Name","value":"projectName"}},{"kind":"Field","name":{"kind":"Name","value":"mainBranch"}},{"kind":"Field","name":{"kind":"Name","value":"agentEnabled"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeBaseRef"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeRunCommand"}},{"kind":"Field","name":{"kind":"Name","value":"worktreeIntegrateMode"}},{"kind":"Field","name":{"kind":"Name","value":"theme"}}]}}]} as unknown as DocumentNode<ConfigQuery, ConfigQueryVariables>;
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