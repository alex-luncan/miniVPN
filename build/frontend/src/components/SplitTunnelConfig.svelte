<script>
  import { onMount } from 'svelte'
  import { SetTunneledPorts, GetTunneledPorts } from '../../wailsjs/go/main/App'

  let ports = $state([])
  let newPort = $state('')
  let mode = $state('include')
  let loading = $state(false)
  let error = $state(null)
  let saved = $state(false)

  onMount(async () => {
    try {
      const config = await GetTunneledPorts()
      ports = config.ports || []
      mode = config.mode || 'include'
    } catch (e) {
      // Ignore - will use defaults
    }
  })

  function addPort() {
    const port = parseInt(newPort)
    if (isNaN(port) || port < 1 || port > 65535) {
      error = 'Please enter a valid port (1-65535)'
      return
    }
    if (ports.includes(port)) {
      error = 'Port already added'
      return
    }
    error = null
    ports = [...ports, port]
    newPort = ''
  }

  function removePort(port) {
    ports = ports.filter(p => p !== port)
  }

  async function saveConfig() {
    loading = true
    error = null
    saved = false

    try {
      await SetTunneledPorts(ports, mode)
      saved = true
      setTimeout(() => saved = false, 2000)
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function handleKeyPress(event) {
    if (event.key === 'Enter') {
      addPort()
    }
  }

  const commonPorts = [
    { port: 3306, name: 'MySQL' },
    { port: 5432, name: 'PostgreSQL' },
    { port: 6379, name: 'Redis' },
    { port: 27017, name: 'MongoDB' },
    { port: 14700, name: 'Custom DB' },
    { port: 22, name: 'SSH' },
    { port: 3389, name: 'RDP' },
  ]
</script>

<div class="split-tunnel-config">
  <h3>Split Tunneling Configuration</h3>
  <p class="description">
    Select which ports should be routed through the VPN. All other traffic will use your normal network connection.
  </p>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <div class="mode-selector">
    <label class="mode-option">
      <input
        type="radio"
        name="tunnel-mode"
        value="include"
        bind:group={mode}
      />
      <span class="mode-label">
        <strong>Include Mode</strong>
        <small>Only selected ports go through VPN</small>
      </span>
    </label>
    <label class="mode-option">
      <input
        type="radio"
        name="tunnel-mode"
        value="exclude"
        bind:group={mode}
      />
      <span class="mode-label">
        <strong>Exclude Mode</strong>
        <small>All traffic except selected ports goes through VPN</small>
      </span>
    </label>
  </div>

  <div class="port-input">
    <input
      type="number"
      bind:value={newPort}
      placeholder="Enter port number"
      min="1"
      max="65535"
      onkeypress={handleKeyPress}
    />
    <button class="add-btn" onclick={addPort}>
      <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z"/>
      </svg>
      Add
    </button>
  </div>

  <div class="quick-add">
    <span>Quick add:</span>
    {#each commonPorts as { port, name }}
      <button
        class="quick-btn"
        class:added={ports.includes(port)}
        onclick={() => !ports.includes(port) && (ports = [...ports, port])}
        disabled={ports.includes(port)}
      >
        {name} ({port})
      </button>
    {/each}
  </div>

  {#if ports.length > 0}
    <div class="port-list">
      <h4>Configured Ports ({ports.length})</h4>
      <div class="ports">
        {#each ports as port}
          <div class="port-tag">
            <span>{port}</span>
            <button class="remove-btn" onclick={() => removePort(port)}>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
              </svg>
            </button>
          </div>
        {/each}
      </div>
    </div>
  {:else}
    <div class="empty-state">
      <p>No ports configured. Add ports to enable split tunneling.</p>
    </div>
  {/if}

  <button
    class="save-btn"
    onclick={saveConfig}
    disabled={loading}
  >
    {#if loading}
      <div class="spinner"></div>
      Saving...
    {:else if saved}
      <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
      </svg>
      Saved!
    {:else}
      <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M17 3H5c-1.11 0-2 .9-2 2v14c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2V7l-4-4zm-5 16c-1.66 0-3-1.34-3-3s1.34-3 3-3 3 1.34 3 3-1.34 3-3 3zm3-10H5V5h10v4z"/>
      </svg>
      Save Configuration
    {/if}
  </button>
</div>

<style>
  .split-tunnel-config {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 20px;
  }

  h3 {
    font-size: 1.1rem;
    margin-bottom: 8px;
    color: #4fc3f7;
  }

  .description {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.5);
    margin-bottom: 20px;
  }

  .error {
    background: rgba(244, 67, 54, 0.2);
    border: 1px solid rgba(244, 67, 54, 0.4);
    color: #ef5350;
    padding: 10px 14px;
    border-radius: 8px;
    margin-bottom: 16px;
    font-size: 0.9rem;
  }

  .mode-selector {
    display: flex;
    gap: 12px;
    margin-bottom: 20px;
  }

  .mode-option {
    flex: 1;
    display: flex;
    align-items: flex-start;
    gap: 12px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 12px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .mode-option:hover {
    background: rgba(255, 255, 255, 0.08);
  }

  .mode-option:has(input:checked) {
    border-color: rgba(79, 195, 247, 0.5);
    background: rgba(79, 195, 247, 0.1);
  }

  .mode-option input {
    margin-top: 3px;
  }

  .mode-label {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .mode-label strong {
    font-size: 0.9rem;
  }

  .mode-label small {
    font-size: 0.8rem;
    color: rgba(255, 255, 255, 0.5);
  }

  .port-input {
    display: flex;
    gap: 12px;
    margin-bottom: 16px;
  }

  .port-input input {
    flex: 1;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 12px 14px;
    color: #fff;
    font-size: 1rem;
  }

  .port-input input:focus {
    outline: none;
    border-color: rgba(79, 195, 247, 0.5);
  }

  .add-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    background: rgba(79, 195, 247, 0.2);
    border: 1px solid rgba(79, 195, 247, 0.3);
    border-radius: 8px;
    padding: 12px 20px;
    color: #4fc3f7;
    cursor: pointer;
    transition: all 0.2s;
  }

  .add-btn:hover {
    background: rgba(79, 195, 247, 0.3);
  }

  .quick-add {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-bottom: 20px;
  }

  .quick-add span {
    font-size: 0.85rem;
    color: rgba(255, 255, 255, 0.5);
  }

  .quick-btn {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 6px;
    padding: 6px 12px;
    font-size: 0.8rem;
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    transition: all 0.2s;
  }

  .quick-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
  }

  .quick-btn.added {
    background: rgba(76, 175, 80, 0.2);
    border-color: rgba(76, 175, 80, 0.3);
    color: #81c784;
  }

  .quick-btn:disabled {
    cursor: default;
  }

  .port-list {
    margin-bottom: 20px;
  }

  .port-list h4 {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.7);
    margin-bottom: 12px;
  }

  .ports {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .port-tag {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(79, 195, 247, 0.2);
    border: 1px solid rgba(79, 195, 247, 0.3);
    border-radius: 20px;
    padding: 6px 12px;
    color: #4fc3f7;
    font-family: 'Consolas', 'Monaco', monospace;
  }

  .remove-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.5);
    cursor: pointer;
    transition: color 0.2s;
  }

  .remove-btn:hover {
    color: #f44336;
  }

  .empty-state {
    text-align: center;
    padding: 24px;
    background: rgba(255, 255, 255, 0.02);
    border-radius: 8px;
    margin-bottom: 20px;
  }

  .empty-state p {
    color: rgba(255, 255, 255, 0.4);
    font-size: 0.9rem;
  }

  .save-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    width: 100%;
    padding: 12px;
    background: rgba(76, 175, 80, 0.2);
    border: 1px solid rgba(76, 175, 80, 0.3);
    border-radius: 8px;
    color: #81c784;
    font-size: 1rem;
    cursor: pointer;
    transition: all 0.2s;
  }

  .save-btn:hover:not(:disabled) {
    background: rgba(76, 175, 80, 0.3);
  }

  .save-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.2);
    border-top-color: currentColor;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
