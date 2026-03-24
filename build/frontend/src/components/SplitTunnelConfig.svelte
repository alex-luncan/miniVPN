<script>
  import { onMount } from 'svelte'
  import { GetRunningApps, SetSplitTunnelApps, GetSplitTunnelApps } from '../../wailsjs/go/main/App'

  let runningApps = $state([])
  let selectedApps = $state([])
  let mode = $state('include')
  let loading = $state(false)
  let loadingApps = $state(false)
  let error = $state(null)
  let saved = $state(false)
  let searchQuery = $state('')

  onMount(async () => {
    await loadConfig()
    await loadRunningApps()
  })

  async function loadConfig() {
    try {
      const config = await GetSplitTunnelApps()
      selectedApps = config.apps || []
      mode = config.mode || 'include'
    } catch (e) {
      // Ignore - will use defaults
    }
  }

  async function loadRunningApps() {
    loadingApps = true
    try {
      const apps = await GetRunningApps()
      runningApps = apps.sort((a, b) => a.name.localeCompare(b.name))
    } catch (e) {
      error = 'Failed to load running applications: ' + e.message
    } finally {
      loadingApps = false
    }
  }

  function isAppSelected(app) {
    return selectedApps.some(a => a.path.toLowerCase() === app.path.toLowerCase())
  }

  function toggleApp(app) {
    if (isAppSelected(app)) {
      selectedApps = selectedApps.filter(a => a.path.toLowerCase() !== app.path.toLowerCase())
    } else {
      selectedApps = [...selectedApps, {
        path: app.path,
        name: app.name,
        exeName: app.exeName
      }]
    }
    error = null
  }

  function removeApp(app) {
    selectedApps = selectedApps.filter(a => a.path.toLowerCase() !== app.path.toLowerCase())
  }

  async function saveConfig() {
    loading = true
    error = null
    saved = false

    try {
      await SetSplitTunnelApps(selectedApps, mode)
      saved = true
      setTimeout(() => saved = false, 2000)
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function getFilteredApps() {
    if (!searchQuery.trim()) {
      return runningApps
    }
    const query = searchQuery.toLowerCase()
    return runningApps.filter(app =>
      app.name.toLowerCase().includes(query) ||
      app.exeName.toLowerCase().includes(query)
    )
  }

  function getAppIcon(app) {
    // Return appropriate icon based on app category
    const name = app.exeName.toLowerCase()
    if (name.includes('chrome') || name.includes('firefox') || name.includes('edge') || name.includes('brave') || name.includes('opera')) {
      return 'browser'
    }
    if (name.includes('discord') || name.includes('slack') || name.includes('teams') || name.includes('zoom') || name.includes('skype')) {
      return 'chat'
    }
    if (name.includes('steam') || name.includes('epic') || name.includes('battle') || name.includes('origin')) {
      return 'game'
    }
    if (name.includes('code') || name.includes('idea') || name.includes('studio') || name.includes('sublime') || name.includes('notepad')) {
      return 'code'
    }
    if (name.includes('torrent') || name.includes('deluge') || name.includes('transmission')) {
      return 'download'
    }
    return 'app'
  }
</script>

<div class="split-tunnel-config">
  <h3>Split Tunneling - Applications</h3>
  <p class="description">
    Select which applications should have their traffic routed through the VPN.
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
        <small>Only selected apps go through VPN</small>
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
        <small>All traffic except selected apps goes through VPN</small>
      </span>
    </label>
  </div>

  {#if selectedApps.length > 0}
    <div class="selected-apps">
      <h4>Selected Applications ({selectedApps.length})</h4>
      <div class="apps-list">
        {#each selectedApps as app}
          <div class="app-tag">
            <span class="app-name">{app.name}</span>
            <button class="remove-btn" onclick={() => removeApp(app)} title="Remove">
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
              </svg>
            </button>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <div class="app-picker">
    <div class="picker-header">
      <h4>Running Applications</h4>
      <button class="refresh-btn" onclick={loadRunningApps} disabled={loadingApps} title="Refresh">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor" class:spinning={loadingApps}>
          <path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/>
        </svg>
      </button>
    </div>

    <div class="search-box">
      <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
        <path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
      </svg>
      <input
        type="text"
        bind:value={searchQuery}
        placeholder="Search applications..."
      />
    </div>

    {#if loadingApps}
      <div class="loading-apps">
        <div class="spinner"></div>
        <span>Loading applications...</span>
      </div>
    {:else if getFilteredApps().length === 0}
      <div class="empty-apps">
        {#if searchQuery}
          <p>No applications match "{searchQuery}"</p>
        {:else}
          <p>No running applications found</p>
        {/if}
      </div>
    {:else}
      <div class="apps-grid">
        {#each getFilteredApps() as app}
          <button
            class="app-item"
            class:selected={isAppSelected(app)}
            onclick={() => toggleApp(app)}
          >
            <div class="app-icon" data-type={getAppIcon(app)}>
              {#if getAppIcon(app) === 'browser'}
                <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
                  <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                </svg>
              {:else if getAppIcon(app) === 'chat'}
                <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
                  <path d="M20 2H4c-1.1 0-1.99.9-1.99 2L2 22l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zM6 9h12v2H6V9zm8 5H6v-2h8v2zm4-6H6V6h12v2z"/>
                </svg>
              {:else if getAppIcon(app) === 'game'}
                <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
                  <path d="M21 6H3c-1.1 0-2 .9-2 2v8c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm-10 7H8v3H6v-3H3v-2h3V8h2v3h3v2zm4.5 2c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm4-3c-.83 0-1.5-.67-1.5-1.5S18.67 9 19.5 9s1.5.67 1.5 1.5-.67 1.5-1.5 1.5z"/>
                </svg>
              {:else if getAppIcon(app) === 'code'}
                <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
                  <path d="M9.4 16.6L4.8 12l4.6-4.6L8 6l-6 6 6 6 1.4-1.4zm5.2 0l4.6-4.6-4.6-4.6L16 6l6 6-6 6-1.4-1.4z"/>
                </svg>
              {:else if getAppIcon(app) === 'download'}
                <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
                  <path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z"/>
                </svg>
              {:else}
                <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
                  <path d="M4 8h4V4H4v4zm6 12h4v-4h-4v4zm-6 0h4v-4H4v4zm0-6h4v-4H4v4zm6 0h4v-4h-4v4zm6-10v4h4V4h-4zm-6 4h4V4h-4v4zm6 6h4v-4h-4v4zm0 6h4v-4h-4v4z"/>
                </svg>
              {/if}
            </div>
            <div class="app-info">
              <span class="app-name">{app.name}</span>
              <span class="app-exe">{app.exeName}</span>
            </div>
            {#if isAppSelected(app)}
              <div class="check-mark">
                <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                  <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
                </svg>
              </div>
            {/if}
          </button>
        {/each}
      </div>
    {/if}
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

  h4 {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.7);
    margin: 0;
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

  .selected-apps {
    margin-bottom: 20px;
    background: rgba(79, 195, 247, 0.05);
    border: 1px solid rgba(79, 195, 247, 0.2);
    border-radius: 8px;
    padding: 12px;
  }

  .selected-apps h4 {
    margin-bottom: 12px;
    color: #4fc3f7;
  }

  .apps-list {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .app-tag {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(79, 195, 247, 0.2);
    border: 1px solid rgba(79, 195, 247, 0.3);
    border-radius: 20px;
    padding: 6px 12px;
    color: #4fc3f7;
  }

  .app-tag .app-name {
    font-size: 0.85rem;
  }

  .remove-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.5);
    cursor: pointer;
    padding: 0;
    transition: color 0.2s;
  }

  .remove-btn:hover {
    color: #f44336;
  }

  .app-picker {
    margin-bottom: 20px;
  }

  .picker-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .refresh-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 6px;
    padding: 6px;
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    transition: all 0.2s;
  }

  .refresh-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
  }

  .refresh-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .refresh-btn .spinning {
    animation: spin 1s linear infinite;
  }

  .search-box {
    display: flex;
    align-items: center;
    gap: 10px;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 10px 14px;
    margin-bottom: 12px;
  }

  .search-box svg {
    color: rgba(255, 255, 255, 0.4);
  }

  .search-box input {
    flex: 1;
    background: none;
    border: none;
    color: #fff;
    font-size: 0.9rem;
    outline: none;
  }

  .search-box input::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }

  .loading-apps, .empty-apps {
    text-align: center;
    padding: 24px;
    background: rgba(255, 255, 255, 0.02);
    border-radius: 8px;
  }

  .loading-apps {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: rgba(255, 255, 255, 0.6);
  }

  .empty-apps p {
    color: rgba(255, 255, 255, 0.4);
    font-size: 0.9rem;
    margin: 0;
  }

  .apps-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 8px;
    max-height: 300px;
    overflow-y: auto;
    padding: 4px;
  }

  .apps-grid::-webkit-scrollbar {
    width: 6px;
  }

  .apps-grid::-webkit-scrollbar-track {
    background: rgba(255, 255, 255, 0.05);
    border-radius: 3px;
  }

  .apps-grid::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
  }

  .app-item {
    display: flex;
    align-items: center;
    gap: 10px;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 10px 12px;
    cursor: pointer;
    transition: all 0.2s;
    text-align: left;
    position: relative;
  }

  .app-item:hover {
    background: rgba(255, 255, 255, 0.08);
  }

  .app-item.selected {
    background: rgba(76, 175, 80, 0.15);
    border-color: rgba(76, 175, 80, 0.4);
  }

  .app-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    color: rgba(255, 255, 255, 0.7);
    flex-shrink: 0;
  }

  .app-icon[data-type="browser"] { color: #4fc3f7; background: rgba(79, 195, 247, 0.2); }
  .app-icon[data-type="chat"] { color: #ba68c8; background: rgba(186, 104, 200, 0.2); }
  .app-icon[data-type="game"] { color: #81c784; background: rgba(129, 199, 132, 0.2); }
  .app-icon[data-type="code"] { color: #ffb74d; background: rgba(255, 183, 77, 0.2); }
  .app-icon[data-type="download"] { color: #f48fb1; background: rgba(244, 143, 177, 0.2); }

  .app-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .app-info .app-name {
    font-size: 0.85rem;
    color: #fff;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .app-info .app-exe {
    font-size: 0.75rem;
    color: rgba(255, 255, 255, 0.4);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .check-mark {
    position: absolute;
    top: 8px;
    right: 8px;
    color: #81c784;
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
