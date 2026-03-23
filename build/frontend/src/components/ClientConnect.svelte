<script>
  import { ConnectToServer, Disconnect, IsConnected, SetTunneledPorts, GetTunneledPorts } from '../../wailsjs/go/main/App'
  import SplitTunnelConfig from './SplitTunnelConfig.svelte'

  let serverIP = $state('')
  let serverPort = $state(51820)
  let secretCode = $state('')
  let connected = $state(false)
  let loading = $state(false)
  let error = $state(null)
  let showSplitTunnel = $state(false)

  async function connect() {
    if (!serverIP || !secretCode) {
      error = 'Please enter server IP and secret code'
      return
    }
    if (!serverPort || serverPort < 1 || serverPort > 65535) {
      error = 'Please enter a valid port (1-65535)'
      return
    }

    loading = true
    error = null

    try {
      await ConnectToServer(serverIP, serverPort, secretCode)
      connected = true
    } catch (e) {
      error = e.message || 'Connection failed'
    } finally {
      loading = false
    }
  }

  async function disconnect() {
    loading = true
    error = null

    try {
      await Disconnect()
      connected = false
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function formatSecretCode(event) {
    let value = event.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '')
    let formatted = ''
    for (let i = 0; i < value.length && i < 20; i++) {
      if (i > 0 && i % 4 === 0) formatted += '-'
      formatted += value[i]
    }
    secretCode = formatted
  }
</script>

<div class="client-connect">
  {#if !connected}
    <div class="connect-form">
      <h2>Connect to VPN Server</h2>
      <p class="subtitle">Enter the server details provided by your VPN administrator</p>

      {#if error}
        <div class="error">{error}</div>
      {/if}

      <div class="form-row">
        <div class="form-group flex-grow">
          <label for="serverIP">Server IP Address</label>
          <input
            id="serverIP"
            type="text"
            bind:value={serverIP}
            placeholder="192.168.1.100 or hostname"
            disabled={loading}
          />
        </div>
        <div class="form-group port-field">
          <label for="serverPort">Port</label>
          <input
            id="serverPort"
            type="number"
            bind:value={serverPort}
            min="1"
            max="65535"
            disabled={loading}
          />
        </div>
      </div>

      <div class="form-group">
        <label for="secretCode">Secret Code</label>
        <input
          id="secretCode"
          type="text"
          value={secretCode}
          oninput={formatSecretCode}
          placeholder="XXXX-XXXX-XXXX-XXXX-XXXX"
          disabled={loading}
        />
      </div>

      <button class="toggle-split" onclick={() => showSplitTunnel = !showSplitTunnel}>
        <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
          <path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/>
        </svg>
        {showSplitTunnel ? 'Hide' : 'Configure'} Split Tunneling
        <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" class:rotated={showSplitTunnel}>
          <path d="M7 10l5 5 5-5z"/>
        </svg>
      </button>

      {#if showSplitTunnel}
        <SplitTunnelConfig />
      {/if}

      <button
        class="connect-btn"
        onclick={connect}
        disabled={loading || !serverIP || !secretCode}
      >
        {#if loading}
          <div class="spinner"></div>
          Connecting...
        {:else}
          <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
            <path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4z"/>
          </svg>
          Connect to VPN
        {/if}
      </button>
    </div>
  {:else}
    <div class="connected-view">
      <div class="connected-status">
        <div class="status-icon">
          <svg viewBox="0 0 24 24" width="64" height="64" fill="currentColor">
            <path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm-2 16l-4-4 1.41-1.41L10 14.17l6.59-6.59L18 9l-8 8z"/>
          </svg>
        </div>
        <h2>Connected</h2>
        <p>Your connection is secure</p>
      </div>

      <div class="connection-details">
        <div class="detail-item">
          <span class="label">Server</span>
          <span class="value">{serverIP}</span>
        </div>
        <div class="detail-item">
          <span class="label">Status</span>
          <span class="value connected-indicator">
            <span class="dot"></span>
            Active
          </span>
        </div>
      </div>

      <button class="toggle-split connected" onclick={() => showSplitTunnel = !showSplitTunnel}>
        <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
          <path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/>
        </svg>
        {showSplitTunnel ? 'Hide' : 'Manage'} Split Tunneling
      </button>

      {#if showSplitTunnel}
        <SplitTunnelConfig />
      {/if}

      <button
        class="disconnect-btn"
        onclick={disconnect}
        disabled={loading}
      >
        {#if loading}
          <div class="spinner"></div>
          Disconnecting...
        {:else}
          <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
            <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
          </svg>
          Disconnect
        {/if}
      </button>
    </div>
  {/if}
</div>

<style>
  .client-connect {
    width: 100%;
    max-width: 500px;
  }

  .connect-form, .connected-view {
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    padding: 32px;
  }

  h2 {
    font-size: 1.75rem;
    margin-bottom: 8px;
    text-align: center;
  }

  .subtitle {
    text-align: center;
    color: rgba(255, 255, 255, 0.5);
    margin-bottom: 32px;
  }

  .error {
    background: rgba(244, 67, 54, 0.2);
    border: 1px solid rgba(244, 67, 54, 0.4);
    color: #ef5350;
    padding: 12px 16px;
    border-radius: 8px;
    margin-bottom: 24px;
  }

  .form-row {
    display: flex;
    gap: 12px;
    margin-bottom: 20px;
  }

  .form-group {
    margin-bottom: 20px;
  }

  .form-row .form-group {
    margin-bottom: 0;
  }

  .form-group.flex-grow {
    flex: 1;
  }

  .form-group.port-field {
    width: 100px;
  }

  .form-group.port-field input {
    text-align: center;
  }

  .form-group label {
    display: block;
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.7);
    margin-bottom: 8px;
  }

  .form-group input {
    width: 100%;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 14px 16px;
    color: #fff;
    font-size: 1rem;
    font-family: 'Consolas', 'Monaco', monospace;
    transition: border-color 0.2s;
  }

  .form-group input:focus {
    outline: none;
    border-color: rgba(79, 195, 247, 0.5);
  }

  .form-group input::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }

  .form-group input:disabled {
    opacity: 0.5;
  }

  .toggle-split {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    width: 100%;
    padding: 12px;
    margin-bottom: 20px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    transition: all 0.2s;
  }

  .toggle-split:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
  }

  .toggle-split.connected {
    margin-top: 24px;
  }

  .toggle-split svg:last-child {
    transition: transform 0.2s;
  }

  .toggle-split svg:last-child.rotated {
    transform: rotate(180deg);
  }

  .connect-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    width: 100%;
    padding: 16px;
    background: linear-gradient(135deg, #4fc3f7, #29b6f6);
    border: none;
    border-radius: 12px;
    color: #fff;
    font-size: 1.1rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .connect-btn:hover:not(:disabled) {
    background: linear-gradient(135deg, #29b6f6, #03a9f4);
    transform: translateY(-2px);
  }

  .connect-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    transform: none;
  }

  .connected-view {
    text-align: center;
  }

  .connected-status {
    margin-bottom: 32px;
  }

  .status-icon {
    width: 100px;
    height: 100px;
    margin: 0 auto 20px;
    background: rgba(76, 175, 80, 0.2);
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #4caf50;
    animation: pulse 2s infinite;
  }

  @keyframes pulse {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.05); }
  }

  .connected-status h2 {
    color: #4caf50;
  }

  .connected-status p {
    color: rgba(255, 255, 255, 0.5);
  }

  .connection-details {
    background: rgba(0, 0, 0, 0.2);
    border-radius: 12px;
    padding: 16px;
    margin-bottom: 24px;
  }

  .detail-item {
    display: flex;
    justify-content: space-between;
    padding: 12px 0;
  }

  .detail-item:not(:last-child) {
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }

  .detail-item .label {
    color: rgba(255, 255, 255, 0.5);
  }

  .detail-item .value {
    font-family: 'Consolas', 'Monaco', monospace;
  }

  .connected-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #4caf50;
  }

  .connected-indicator .dot {
    width: 8px;
    height: 8px;
    background: #4caf50;
    border-radius: 50%;
    animation: blink 1.5s infinite;
  }

  @keyframes blink {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }

  .disconnect-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    width: 100%;
    padding: 16px;
    margin-top: 16px;
    background: rgba(244, 67, 54, 0.2);
    border: 1px solid rgba(244, 67, 54, 0.3);
    border-radius: 12px;
    color: #f44336;
    font-size: 1.1rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.2s;
  }

  .disconnect-btn:hover:not(:disabled) {
    background: rgba(244, 67, 54, 0.3);
  }

  .disconnect-btn:disabled {
    opacity: 0.5;
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
</style>
