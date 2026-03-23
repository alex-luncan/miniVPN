<script>
  import { SetMode } from '../../wailsjs/go/main/App'

  let { onselect } = $props()
  let loading = $state(false)
  let error = $state(null)

  async function selectMode(mode) {
    loading = true
    error = null

    try {
      await SetMode(mode)
      onselect({ detail: { mode } })
    } catch (e) {
      error = e.message || 'Failed to set mode'
    } finally {
      loading = false
    }
  }
</script>

<div class="mode-selector">
  <h1>Welcome to miniVPN</h1>
  <p class="subtitle">Select your connection mode to get started</p>

  {#if error}
    <div class="error">{error}</div>
  {/if}

  <div class="mode-cards">
    <button
      class="mode-card server"
      onclick={() => selectMode('server')}
      disabled={loading}
    >
      <div class="icon">
        <svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor">
          <path d="M4 1h16a1 1 0 011 1v4a1 1 0 01-1 1H4a1 1 0 01-1-1V2a1 1 0 011-1m0 8h16a1 1 0 011 1v4a1 1 0 01-1 1H4a1 1 0 01-1-1v-4a1 1 0 011-1m0 8h16a1 1 0 011 1v4a1 1 0 01-1 1H4a1 1 0 01-1-1v-4a1 1 0 011-1M9 5h1V3H9v2m0 8h1v-2H9v2m0 8h1v-2H9v2M5 3v2h2V3H5m0 8v2h2v-2H5m0 8v2h2v-2H5z"/>
        </svg>
      </div>
      <h2>Server Mode</h2>
      <p>Host a VPN server for others to connect. You'll receive a secret code to share with clients.</p>
      <div class="features">
        <span>Generate secret codes</span>
        <span>Configure server settings</span>
        <span>Monitor connections</span>
      </div>
    </button>

    <button
      class="mode-card client"
      onclick={() => selectMode('client')}
      disabled={loading}
    >
      <div class="icon">
        <svg viewBox="0 0 24 24" width="48" height="48" fill="currentColor">
          <path d="M4 6h16v2H4zm0 5h16v2H4zm0 5h16v2H4z"/>
          <circle cx="20" cy="12" r="2"/>
          <path d="M20 8v-3a1 1 0 00-1-1H5a1 1 0 00-1 1v3"/>
        </svg>
      </div>
      <h2>Client Mode</h2>
      <p>Connect to an existing VPN server using the server IP and secret code provided to you.</p>
      <div class="features">
        <span>Split tunneling</span>
        <span>Port-based routing</span>
        <span>Secure connection</span>
      </div>
    </button>
  </div>

  {#if loading}
    <div class="loading">
      <div class="spinner"></div>
      <span>Initializing...</span>
    </div>
  {/if}
</div>

<style>
  .mode-selector {
    text-align: center;
    max-width: 800px;
    width: 100%;
  }

  h1 {
    font-size: 2.5rem;
    margin-bottom: 8px;
    background: linear-gradient(135deg, #4fc3f7, #81d4fa);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .subtitle {
    color: rgba(255, 255, 255, 0.6);
    margin-bottom: 40px;
    font-size: 1.1rem;
  }

  .error {
    background: rgba(244, 67, 54, 0.2);
    border: 1px solid rgba(244, 67, 54, 0.4);
    color: #ef5350;
    padding: 12px 16px;
    border-radius: 8px;
    margin-bottom: 24px;
  }

  .mode-cards {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 24px;
  }

  .mode-card {
    background: rgba(255, 255, 255, 0.05);
    border: 2px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    padding: 32px;
    cursor: pointer;
    transition: all 0.3s ease;
    text-align: left;
    color: inherit;
  }

  .mode-card:hover:not(:disabled) {
    transform: translateY(-4px);
    border-color: rgba(79, 195, 247, 0.5);
    background: rgba(255, 255, 255, 0.08);
  }

  .mode-card:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .mode-card.server:hover:not(:disabled) {
    border-color: rgba(129, 199, 132, 0.5);
  }

  .mode-card.client:hover:not(:disabled) {
    border-color: rgba(79, 195, 247, 0.5);
  }

  .icon {
    width: 72px;
    height: 72px;
    border-radius: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 20px;
  }

  .server .icon {
    background: linear-gradient(135deg, rgba(129, 199, 132, 0.2), rgba(76, 175, 80, 0.3));
    color: #81c784;
  }

  .client .icon {
    background: linear-gradient(135deg, rgba(79, 195, 247, 0.2), rgba(41, 182, 246, 0.3));
    color: #4fc3f7;
  }

  .mode-card h2 {
    font-size: 1.5rem;
    margin-bottom: 12px;
    color: #fff;
  }

  .mode-card p {
    color: rgba(255, 255, 255, 0.6);
    line-height: 1.5;
    margin-bottom: 20px;
  }

  .features {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .features span {
    background: rgba(255, 255, 255, 0.1);
    padding: 6px 12px;
    border-radius: 20px;
    font-size: 0.85rem;
    color: rgba(255, 255, 255, 0.7);
  }

  .loading {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    margin-top: 32px;
    color: rgba(255, 255, 255, 0.7);
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 3px solid rgba(255, 255, 255, 0.2);
    border-top-color: #4fc3f7;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  @media (max-width: 700px) {
    .mode-cards {
      grid-template-columns: 1fr;
    }
  }
</style>
