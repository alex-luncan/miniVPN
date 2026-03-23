<script>
  import ModeSelector from './components/ModeSelector.svelte'
  import ServerDashboard from './components/ServerDashboard.svelte'
  import ClientConnect from './components/ClientConnect.svelte'

  let currentMode = $state(null)

  function handleModeSelect(event) {
    currentMode = event.detail.mode
  }

  function handleBack() {
    currentMode = null
  }
</script>

<main>
  <header>
    <div class="logo">
      <svg viewBox="0 0 24 24" width="32" height="32" fill="currentColor">
        <path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm0 2.18l7 3.12v5.7c0 4.47-3.02 8.65-7 9.82-3.98-1.17-7-5.35-7-9.82V6.3l7-3.12z"/>
        <path d="M12 7c-1.1 0-2 .9-2 2v2H9c-.55 0-1 .45-1 1v4c0 .55.45 1 1 1h6c.55 0 1-.45 1-1v-4c0-.55-.45-1-1-1h-1V9c0-1.1-.9-2-2-2zm0 1.5c.28 0 .5.22.5.5v2h-1V9c0-.28.22-.5.5-.5z"/>
      </svg>
      <span>miniVPN</span>
    </div>
    {#if currentMode}
      <button class="back-btn" onclick={handleBack}>
        <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
          <path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z"/>
        </svg>
        Change Mode
      </button>
    {/if}
  </header>

  <div class="content">
    {#if !currentMode}
      <ModeSelector onselect={handleModeSelect} />
    {:else if currentMode === 'server'}
      <ServerDashboard />
    {:else if currentMode === 'client'}
      <ClientConnect />
    {/if}
  </div>

  <footer>
    <span>miniVPN v1.0.0</span>
    <span class="separator">|</span>
    <span>Secure Split-Tunnel VPN</span>
  </footer>
</main>

<style>
  main {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;
    background: rgba(0, 0, 0, 0.2);
    backdrop-filter: blur(10px);
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }

  .logo {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 1.5rem;
    font-weight: 600;
    color: #4fc3f7;
  }

  .back-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 8px;
    color: #e0e0e0;
    cursor: pointer;
    transition: all 0.2s;
  }

  .back-btn:hover {
    background: rgba(255, 255, 255, 0.15);
    border-color: rgba(255, 255, 255, 0.3);
  }

  .content {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
    overflow-y: auto;
  }

  footer {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 8px;
    padding: 12px;
    background: rgba(0, 0, 0, 0.2);
    border-top: 1px solid rgba(255, 255, 255, 0.1);
    font-size: 0.85rem;
    color: rgba(255, 255, 255, 0.5);
  }

  .separator {
    color: rgba(255, 255, 255, 0.2);
  }
</style>
