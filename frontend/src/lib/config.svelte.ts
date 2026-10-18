import { client } from './graphqlClient';
import { ConfigDocument } from './graphql/generated';

class ConfigStore {
  projectName = $state('');
  mainBranch = $state('main');
  agentEnabled = $state(true);
  worktreeBaseRef = $state('main');
  worktreeRunCommand = $state('');
  worktreeIntegrateMode = $state<'local' | 'pr'>('local');
  /** display.theme from .lif.yml; empty until the config has loaded. */
  theme = $state('');

  async load(): Promise<void> {
    const result = await client.query(ConfigDocument, {}).toPromise();
    if (result.error) {
      console.warn('Failed to load config:', result.error.message);
      return;
    }
    if (result.data) {
      this.projectName = result.data.projectName;
      this.mainBranch = result.data.mainBranch;
      this.agentEnabled = result.data.agentEnabled;
      this.worktreeBaseRef = result.data.worktreeBaseRef;
      this.worktreeRunCommand = result.data.worktreeRunCommand;
      this.worktreeIntegrateMode = result.data.worktreeIntegrateMode as 'local' | 'pr';
      this.theme = result.data.theme;
    }
  }
}

export const configStore = new ConfigStore();
