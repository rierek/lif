import { pipe, subscribe } from 'wonka';
import { client } from './graphqlClient';
import {
  AgentSessionChangedDocument,
  SendAgentMessageDocument,
  StopAgentDocument,
  SetAgentPlanModeDocument,
  SetAgentActModeDocument,
  SetAgentEffortDocument,
  ClearAgentSessionDocument,
  type AgentSessionFieldsFragment,
  AgentMessageRole,
  type InteractionType,
  type ImageInput,
  type FileAttachmentInput,
} from './graphql/generated';

// Derived from the fragment: typescript-operations 6 no longer emits schema object types.
export type AgentSession = AgentSessionFieldsFragment;
export type AgentMessage = AgentSession['messages'][number];
export type AgentMessageImage = AgentMessage['images'][number];
export type { InteractionType };
export type PendingInteraction = NonNullable<AgentSession['pendingInteraction']>;
export type AskUserQuestionData = NonNullable<PendingInteraction['questions']>[number];
export type AskUserOption = AskUserQuestionData['options'][number];
export type SubagentActivity = AgentSession['subagentActivities'][number];
export type ImageUploadInput = ImageInput;
export type FileAttachment = FileAttachmentInput;

export class AgentChatStore {
  session = $state<AgentSession | null>(null);
  sending = $state(false);
  error = $state<string | null>(null);

  #itemId: string | null = null;
  #unsubscribe: (() => void) | null = null;

  subscribe(itemId: string): void {
    // If already subscribed to the same item, skip
    if (this.#unsubscribe && this.#itemId === itemId) return;

    // Clean up previous subscription
    this.unsubscribe();
    this.#itemId = itemId;

    const { unsubscribe } = pipe(
      client.subscription(AgentSessionChangedDocument, { itemId: itemId }),
      subscribe((result) => {
        if (result.error) {
          console.error('Agent session subscription error:', result.error);
          this.error = result.error.message;
          return;
        }

        const session = result.data?.agentSessionChanged;
        if (session) {
          const prev = this.session;

          // Log new messages (user/tool appear instantly, so count-based works)
          const prevLen = prev?.messages.length ?? 0;
          for (let i = prevLen; i < session.messages.length; i++) {
            const msg = session.messages[i];
            if (msg.role !== 'ASSISTANT') {
              console.debug(`[agent:${msg.role}]`, msg.content);
            }
          }

          // Log completed assistant messages when turn finishes
          if (prev?.status === 'RUNNING' && session.status === 'IDLE') {
            for (const msg of session.messages.slice(prevLen > 0 ? prevLen - 1 : 0)) {
              if (msg.role === 'ASSISTANT' && msg.content) {
                console.debug('[agent:ASSISTANT]', msg.content);
              }
            }
          }

          if (session.systemStatus && session.systemStatus !== prev?.systemStatus) {
            console.debug('[agent:system]', session.systemStatus);
          }
          if (session.error && session.error !== prev?.error) {
            console.debug('[agent:error]', session.error);
          }

          this.session = session;
          this.error = null;
        }
      })
    );

    this.#unsubscribe = unsubscribe;
  }

  unsubscribe(): void {
    if (this.#unsubscribe) {
      this.#unsubscribe();
      this.#unsubscribe = null;
    }
    this.#itemId = null;
  }

  async sendMessage(
    itemId: string,
    message: string,
    images?: ImageUploadInput[],
    attachments?: FileAttachment[]
  ): Promise<boolean> {
    this.sending = true;
    this.error = null;

    // Optimistically append user message so it appears instantly
    if (this.session) {
      this.session = {
        ...this.session,
        messages: [
          ...this.session.messages,
          {
            role: AgentMessageRole.User,
            content: message,
            images: [],
            attachments: attachments?.map(a => a.path) ?? [],
            diff: null
          }
        ]
      };
    }

    const result = await client
      .mutation(SendAgentMessageDocument, {
        itemId: itemId,
        message,
        images: images ?? null,
        attachments: attachments?.length ? attachments : null
      })
      .toPromise();

    this.sending = false;

    if (result.error) {
      this.error = result.error.message;
      return false;
    }

    return true;
  }

  async stop(itemId: string): Promise<boolean> {
    const result = await client.mutation(StopAgentDocument, { itemId: itemId }).toPromise();

    if (result.error) {
      this.error = result.error.message;
      return false;
    }

    return true;
  }

  async setPlanMode(itemId: string, planMode: boolean): Promise<boolean> {
    const result = await client.mutation(SetAgentPlanModeDocument, { itemId: itemId, planMode }).toPromise();

    if (result.error) {
      this.error = result.error.message;
      return false;
    }

    return true;
  }

  async setActMode(itemId: string, actMode: boolean): Promise<boolean> {
    const result = await client.mutation(SetAgentActModeDocument, { itemId: itemId, actMode }).toPromise();

    if (result.error) {
      this.error = result.error.message;
      return false;
    }

    return true;
  }

  async setEffort(itemId: string, effort: string): Promise<boolean> {
    const result = await client
      .mutation(SetAgentEffortDocument, { itemId: itemId, effort })
      .toPromise();

    if (result.error) {
      this.error = result.error.message;
      return false;
    }

    return true;
  }

  async clearSession(itemId: string): Promise<boolean> {
    const result = await client.mutation(ClearAgentSessionDocument, { itemId: itemId }).toPromise();

    if (result.error) {
      this.error = result.error.message;
      return false;
    }

    return true;
  }
}
