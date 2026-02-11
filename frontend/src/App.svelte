<!-- frontend/src/App.svelte -->
<script>
    import { onMount } from 'svelte';
    import { 
        ActivateWithKey, Connect, Disconnect, GetStatus, 
        SaveKey, LoadKey, DeleteKey 
    } from '../wailsjs/go/main/App';
    import { BrowserOpenURL, WindowSetSize } from '../wailsjs/runtime';

    let uiState = 'loading';
    let activationKey = "";
    let rememberDevice = true;
    let vlessLink = "";
    let serverName = "Сервер";
    let isConnected = false;
    let isLoading = false;
    let errorMessage = "";

    const boostyURL = 'https://boosty.to/justmyvpnclient/purchase/3432442?ssource=DIRECT&share=subscription_link';
    const boostyURL2 = 'https://boosty.to/justmyvpnclient';

    const windowWidth = 520;
    const activateWindowHeight = 376;
    const activateWindowHeightWithError = 430;
    const mainWindowHeight = 340;

     $: if (uiState === 'activate') {
        if (errorMessage) {
            WindowSetSize(windowWidth, activateWindowHeightWithError);
        } else {
            WindowSetSize(windowWidth, activateWindowHeight);
        }
    }

    onMount(() => {
        LoadKey().then(savedKey => {
            if (savedKey) {
                activationKey = savedKey;
                handleActivation(true);
            } else {
                switchToActivateScreen();
            }
        }).catch(() => {
            switchToActivateScreen();
        });
    });

    function switchToActivateScreen() {
        WindowSetSize(windowWidth, activateWindowHeight); // Устанавливаем стандартный размер
        uiState = 'activate';
    }

    function switchToMainScreen() {
        WindowSetSize(windowWidth, mainWindowHeight);
        uiState = 'main_app';
    }

    async function handleActivation(isAutoActivation = false) {
        if (!activationKey) return;
        if (!isAutoActivation) isLoading = true;
        errorMessage = ""; // Сбрасываем ошибку перед новой попыткой
        try {
            const link = await ActivateWithKey(activationKey);
            vlessLink = link;
            serverName = "Нидерланды";
            if (rememberDevice) {
                await SaveKey(activationKey);
            } else {
                await DeleteKey();
            }
            switchToMainScreen();
        } catch (error) {
            errorMessage = error;
            // switchToActivateScreen() больше не нужен здесь, реактивный блок сам изменит размер
        } finally {
            if (!isAutoActivation) isLoading = false;
        }
    }

    async function handleLogout() {
        isLoading = true;
        errorMessage = "";
        try {
            if (isConnected) {
                await Disconnect();
                isConnected = false;
            }
            await DeleteKey();
            activationKey = "";
            vlessLink = "";
            switchToActivateScreen();
        } catch (error) {
            errorMessage = "Ошибка при выходе: " + error;
        } finally {
            isLoading = false;
        }
    }
    
    async function handleToggleConnection() {
        isLoading = true;
        errorMessage = "";
        try {
            if (isConnected) {
                await Disconnect();
                isConnected = false;
            } else {
                await Connect(vlessLink);
                isConnected = true;
            }
        } catch (error) {
            errorMessage = error;
            isConnected = await GetStatus();
        } finally {
            isLoading = false;
        }
    }
</script>

<main>
    {#if uiState === 'loading'}
        <div class="layout-wrapper">Загрузка...</div>

    {:else if uiState === 'activate'}
        <div class="layout-wrapper">
            <h1>Активация доступа</h1>
            <input 
                type="text" 
                placeholder="XXX-XXX-XXX" 
                bind:value={activationKey} 
                disabled={isLoading}
                on:input={() => errorMessage = ''}
            >
            <div class="checkbox-container" on:click={() => rememberDevice = !rememberDevice}>
                <input type="checkbox" id="remember" bind:checked={rememberDevice}>
                <label for="remember">Запомнить на этом устройстве</label>
            </div>
            <button class="btn action-btn" on:click={() => handleActivation(false)} disabled={isLoading || !activationKey}>
                {isLoading ? 'Проверка...' : 'Активировать'}
            </button>
            
            <div class="footer-buttons activate-footer">
                <button class="btn server-btn" on:click={() => BrowserOpenURL(boostyURL)}>
                    Купить ключ
                </button>
            </div>
        </div>

    {:else if uiState === 'main_app'}
        <div class="layout-wrapper">
            <div class="display">
                <div class="status-text">
                    {#if isConnected} Подключено {:else} Отключено {/if}
                </div>
                <div class="server-name">{serverName}</div>
            </div>
            <button class="btn action-btn" on:click={handleToggleConnection} disabled={isLoading}>
                {#if isLoading} ... {:else if isConnected} Отключить {:else} Подключить {/if}
            </button>
            
            <div class="footer-buttons">
                <button class="btn server-btn" on:click={handleLogout} disabled={isLoading}>Выйти</button>
                <button class="btn server-btn" on:click={() => BrowserOpenURL(boostyURL2)} disabled={isLoading}>Boosty</button>
            </div>
        </div>
    {/if}

    {#if errorMessage}
        <div class="error">{errorMessage}</div>
    {/if}
</main>

<style>
    :global(body) {
        background-color: #2d2d2d;
        color: white;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen,
            Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif;
        margin: 0;
        overflow: hidden;
        user-select: none;
    }
    main {
        padding: 1em;
        display: flex;
        flex-direction: column;
        height: 100vh;
        box-sizing: border-box;
        justify-content: flex-start;
        align-items: center;
    }
    .layout-wrapper {
        width: 100%;
        display: flex;
        flex-direction: column;
        gap: 10px; 
    }
    .display {
        background-color: #1e1e1e;
        padding: 20px;
        text-align: right;
        border: 1px solid #444;
    }
    .status-text {
        font-size: 1.2em;
        color: #aaa;
    }
    .server-name {
        font-size: 2.5em;
        font-weight: bold;
        min-height: 1.2em;
    }
    .btn {
        height: 60px;
        border: none;
        font-size: 1.2em;
        font-weight: 500;
        cursor: pointer;
        transition: background-color 0.2s ease;
        color: white;
    }
    .btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    .action-btn {
        background-color: #ff6f00;
    }
    .action-btn:hover:not(:disabled) {
        background-color: #ff8f00;
    }
    .error {
        margin-top: 15px;
        padding: 10px;
        color: #ffcdd2;
        background-color: #c62828;
        text-align: center;
        word-break: break-word;
        width: 100%;
        box-sizing: border-box;
    }
    .checkbox-container {
        display: flex;
        justify-content: center;
        align-items: center;
        gap: 8px;
        color: #ccc;
        cursor: pointer;
    }
    .checkbox-container input {
        width: auto;
        height: auto;
    }
    .footer-buttons {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 10px;
        margin-top: auto;
        padding-top: 10px;
    }
    .activate-footer {
        grid-template-columns: 1fr;
    }
    input[type="text"] {
        height: 50px;
        background: #1e1e1e;
        border: 1px solid #444;
        color: white;
        text-align: center;
        font-size: 1.5em;
        padding: 10px;
        box-sizing: border-box;
    }
    .server-btn {
        background-color: #00897b;
    }
    .server-btn:hover:not(:disabled) {
        background-color: #00a99d;
    }
    h1 {
        margin-bottom: 0;
    }
    p {
        color: #ccc;
        margin-top: 0;
    }
</style>