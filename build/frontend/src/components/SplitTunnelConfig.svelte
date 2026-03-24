<script>
  import { onMount } from 'svelte'
  import { SetSplitTunnelApps, GetSplitTunnelApps } from '../../wailsjs/go/main/App'

  let mode = $state('exclude')
  let loading = $state(false)
  let error = $state(null)
  let saved = $state(false)

  onMount(async () => {
    try {
      const config = await GetSplitTunnelApps()
      mode = config.mode || 'exclude'
    } catch (e) {
      // Ignore - will use defaults
    }
  })

  async function saveConfig() {
    loading = true
    error = null
    saved = false

    try {
      // Pass empty apps array - only mode matters for routing
      await SetSplitTunnelApps([], mode)
      saved = true
      setTimeout(() => saved = false, 2000)
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }
</script>

<div class="split-tunnel-config">
  <h3>Split Tunneling</h3>
  <p class="description">
    Control how your traffic is routed when connected to VPN.
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
        <strong>Split Tunnel Mode</strong>
        <small>Only VPN network (10.0.0.x) uses tunnel</small>
        <span class="mode-detail">Your public IP remains unchanged for internet traffic</span>
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
        <strong>Full VPN Mode</strong>
        <small>All traffic goes through VPN tunnel</small>
        <span class="mode-detail">Your public IP shows the VPN server's IP</span>
      </span>
    </label>
  </div>

  <div class="info-box">
    <strong>When to use each mode:</strong>
    <ul>
      <li><strong>Split Tunnel:</strong> Access VPN resources (10.0.0.x) while keeping normal internet speed and your real IP</li>
      <li><strong>Full VPN:</strong> Route all traffic through VPN for privacy/security (slower, but hides your real IP)</li>
    </ul>
  </div>

  <div class="reconnect-notice">
    <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
    </svg>
    <span>Changes take effect on next connection</span>
  </div>

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
    flex-direction: column;
    gap: 12px;
    margin-bottom: 20px;
  }

  .mode-option {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 14px;
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
    font-size: 0.95rem;
  }

  .mode-label small {
    font-size: 0.8rem;
    color: rgba(255, 255, 255, 0.5);
  }

  .mode-detail {
    font-size: 0.8rem;
    color: #81c784;
    margin-top: 4px;
  }

  .info-box {
    background: rgba(79, 195, 247, 0.08);
    border: 1px solid rgba(79, 195, 247, 0.2);
    border-radius: 8px;
    padding: 12px;
    margin-bottom: 16px;
    font-size: 0.85rem;
  }

  .info-box strong {
    color: #4fc3f7;
    display: block;
    margin-bottom: 8px;
  }

  .info-box ul {
    margin: 0;
    padding-left: 20px;
  }

  .info-box li {
    margin: 6px 0;
    color: rgba(255, 255, 255, 0.7);
  }

  .info-box li strong {
    display: inline;
    color: rgba(255, 255, 255, 0.9);
  }

  .reconnect-notice {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(255, 183, 77, 0.1);
    border: 1px solid rgba(255, 183, 77, 0.3);
    border-radius: 8px;
    padding: 10px 14px;
    margin-bottom: 16px;
    font-size: 0.85rem;
    color: #ffb74d;
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
