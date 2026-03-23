<script>
  import { onMount, onDestroy } from 'svelte'
  import { GetSecretCode, RegenerateSecretCode, StartServer, StopServer, IsConnected, GetLocalIP, GetPublicIP, GetClientCount } from '../../wailsjs/go/main/App'

  let secretCode = $state('')
  let serverRunning = $state(false)
  let localIP = $state('Loading...')
  let publicIP = $state('Loading...')
  let port = $state(51820)
  let loading = $state(false)
  let error = $state(null)
  let copied = $state(false)
  let clientCount = $state(0)
  let pollInterval = null

  onMount(async () => {
    try {
      secretCode = await GetSecretCode()
      serverRunning = await IsConnected()
      localIP = await GetLocalIP()

      // Fetch public IP (async, may take a moment)
      GetPublicIP().then(ip => publicIP = ip).catch(() => publicIP = 'Unable to determine')

      // Poll for client count when server is running
      pollInterval = setInterval(async () => {
        if (serverRunning) {
          try {
            clientCount = await GetClientCount()
          } catch (e) {}
        }
      }, 2000)
    } catch (e) {
      error = e.message
    }
  })

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval)
  })

  async function regenerateCode() {
    try {
      secretCode = await RegenerateSecretCode()
    } catch (e) {
      error = e.message
    }
  }

  async function toggleServer() {
    loading = true
    error = null

    try {
      if (serverRunning) {
        await StopServer()
        serverRunning = false
      } else {
        await StartServer(port)
        serverRunning = true
      }
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function copyToClipboard() {
    navigator.clipboard.writeText(secretCode)
    copied = true
    setTimeout(() => copied = false, 2000)
  }
</script>

<div class="server-dashboard">
  <div class="status-card">
    <div class="status-indicator" class:active={serverRunning}></div>
    <div class="status-text">
      <h3>Server Status</h3>
      <span>{serverRunning ? 'Running' : 'Stopped'}</span>
    </div>
    {#if serverRunning}
      <div class="client-count">
        <span class="count">{clientCount}</span>
        <span class="label">client{clientCount !== 1 ? 's' : ''}</span>
      </div>
    {/if}
  </div>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <div class="info-grid">
    <div class="info-card">
      <label>Public IP (for external clients)</label>
      <div class="info-value">{publicIP}</div>
    </div>

    <div class="info-card">
      <label>Local IP (for LAN clients)</label>
      <div class="info-value">{localIP}</div>
    </div>

    <div class="info-card">
      <label>Server Port</label>
      <input
        type="number"
        bind:value={port}
        min="1024"
        max="65535"
        disabled={serverRunning}
      />
    </div>
  </div>

  <div class="secret-section">
    <div class="secret-header">
      <h3>Secret Code</h3>
      <button class="icon-btn" onclick={regenerateCode} disabled={serverRunning} title="Regenerate Code">
        <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
          <path d="M17.65 6.35A7.958 7.958 0 0012 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08A5.99 5.99 0 0112 18c-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/>
        </svg>
      </button>
    </div>

    <div class="secret-code">
      <code>{secretCode}</code>
      <button class="copy-btn" onclick={copyToClipboard}>
        {#if copied}
          <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
            <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
          </svg>
          Copied!
        {:else}
          <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
            <path d="M16 1H4c-1.1 0-2 .9-2 2v14h2V3h12V1zm3 4H8c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h11c1.1 0 2-.9 2-2V7c0-1.1-.9-2-2-2zm0 16H8V7h11v14z"/>
          </svg>
          Copy
        {/if}
      </button>
    </div>
    <p class="secret-hint">Share this code with clients who need to connect. Code changes when regenerated.</p>
  </div>

  <div class="controls">
    <button
      class="control-btn"
      class:start={!serverRunning}
      class:stop={serverRunning}
      onclick={toggleServer}
      disabled={loading}
    >
      {#if loading}
        <div class="spinner"></div>
        {serverRunning ? 'Stopping...' : 'Starting...'}
      {:else if serverRunning}
        <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
          <path d="M6 6h12v12H6z"/>
        </svg>
        Stop Server
      {:else}
        <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
          <path d="M8 5v14l11-7z"/>
        </svg>
        Start Server
      {/if}
    </button>
  </div>

  {#if serverRunning}
    <div class="connection-info">
      <h4>Connection Details for Clients</h4>
      <p><strong>Public IP:</strong> {publicIP}</p>
      <p><strong>Local IP:</strong> {localIP}</p>
      <p><strong>Port:</strong> {port}</p>
      <p><strong>Secret:</strong> {secretCode}</p>
    </div>
  {/if}
</div>

<style>
  .server-dashboard {
    width: 100%;
    max-width: 600px;
  }

  .status-card {
    display: flex;
    align-items: center;
    gap: 16px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 24px;
  }

  .status-indicator {
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: #f44336;
    box-shadow: 0 0 12px rgba(244, 67, 54, 0.5);
  }

  .status-indicator.active {
    background: #4caf50;
    box-shadow: 0 0 12px rgba(76, 175, 80, 0.5);
    animation: pulse 2s infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.7; }
  }

  .status-text h3 {
    font-size: 0.85rem;
    color: rgba(255, 255, 255, 0.5);
    margin-bottom: 4px;
  }

  .status-text span {
    font-size: 1.25rem;
    font-weight: 600;
  }

  .client-count {
    margin-left: auto;
    text-align: center;
    padding: 8px 16px;
    background: rgba(79, 195, 247, 0.1);
    border-radius: 8px;
  }

  .client-count .count {
    display: block;
    font-size: 1.5rem;
    font-weight: 700;
    color: #4fc3f7;
  }

  .client-count .label {
    font-size: 0.75rem;
    color: rgba(255, 255, 255, 0.5);
    text-transform: uppercase;
  }

  .error {
    background: rgba(244, 67, 54, 0.2);
    border: 1px solid rgba(244, 67, 54, 0.4);
    color: #ef5350;
    padding: 12px 16px;
    border-radius: 8px;
    margin-bottom: 24px;
  }

  .info-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
    margin-bottom: 24px;
  }

  .info-card {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 16px;
  }

  .info-card label {
    display: block;
    font-size: 0.85rem;
    color: rgba(255, 255, 255, 0.5);
    margin-bottom: 8px;
  }

  .info-value {
    font-size: 1.1rem;
    font-weight: 500;
    font-family: 'Consolas', 'Monaco', monospace;
  }

  .info-card input {
    width: 100%;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 10px;
    color: #fff;
    font-size: 1.1rem;
    font-family: 'Consolas', 'Monaco', monospace;
  }

  .info-card input:disabled {
    opacity: 0.5;
  }

  .secret-section {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 24px;
  }

  .secret-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
  }

  .secret-header h3 {
    font-size: 1rem;
    color: rgba(255, 255, 255, 0.7);
  }

  .icon-btn {
    background: rgba(255, 255, 255, 0.1);
    border: none;
    border-radius: 8px;
    padding: 8px;
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    transition: all 0.2s;
  }

  .icon-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.2);
    color: #fff;
  }

  .icon-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .secret-code {
    display: flex;
    align-items: center;
    gap: 12px;
    background: rgba(0, 0, 0, 0.3);
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 12px;
  }

  .secret-code code {
    flex: 1;
    font-size: 1.3rem;
    font-family: 'Consolas', 'Monaco', monospace;
    letter-spacing: 2px;
    color: #81c784;
  }

  .copy-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(79, 195, 247, 0.2);
    border: 1px solid rgba(79, 195, 247, 0.3);
    border-radius: 8px;
    padding: 8px 16px;
    color: #4fc3f7;
    cursor: pointer;
    transition: all 0.2s;
  }

  .copy-btn:hover {
    background: rgba(79, 195, 247, 0.3);
  }

  .secret-hint {
    font-size: 0.85rem;
    color: rgba(255, 255, 255, 0.4);
  }

  .controls {
    margin-bottom: 24px;
  }

  .control-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    width: 100%;
    padding: 16px;
    border: none;
    border-radius: 12px;
    font-size: 1.1rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .control-btn.start {
    background: linear-gradient(135deg, #4caf50, #66bb6a);
    color: #fff;
  }

  .control-btn.start:hover:not(:disabled) {
    background: linear-gradient(135deg, #66bb6a, #81c784);
  }

  .control-btn.stop {
    background: linear-gradient(135deg, #f44336, #ef5350);
    color: #fff;
  }

  .control-btn.stop:hover:not(:disabled) {
    background: linear-gradient(135deg, #ef5350, #e57373);
  }

  .control-btn:disabled {
    opacity: 0.7;
    cursor: not-allowed;
  }

  .spinner {
    width: 20px;
    height: 20px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-top-color: #fff;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .connection-info {
    background: rgba(76, 175, 80, 0.1);
    border: 1px solid rgba(76, 175, 80, 0.3);
    border-radius: 12px;
    padding: 20px;
  }

  .connection-info h4 {
    color: #81c784;
    margin-bottom: 12px;
  }

  .connection-info p {
    font-family: 'Consolas', 'Monaco', monospace;
    margin-bottom: 8px;
    color: rgba(255, 255, 255, 0.8);
  }
</style>
