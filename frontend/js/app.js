// Global State
const state = {
    config: {
        c2c_interval_minutes: 3,
        forex_interval_hours: 1,
        forex_max_age_hours: 6,
        target_amounts: []
    },
    version: 'unknown',
    supportedExchanges: [],
    historyKeys: {},
    currentAmount: null,
    currentRange: '1d',
    chartInstance: null
};

const FOREX_SERIES_NAME = 'USD/CNY 汇率';

// Initialization
document.addEventListener('DOMContentLoaded', () => {
    bindEvents();
    initTabs();
    initChart();
    
    Promise.all([loadMeta(), loadConfig()]).then(() => {
        // After config loaded, load initial data
        if (state.config.target_amounts && state.config.target_amounts.length > 0) {
            state.currentAmount = state.config.target_amounts[0];
            loadChartData();
        }
        loadActiveAlerts();
        loadSystemStatus();
    });
});

function getElements() {
    return {
        tabs: document.querySelectorAll('.tab-btn'),
        tabContents: document.querySelectorAll('.tab-content'),
        amountSelect: document.getElementById('amount-select'),
        rangeBtns: document.querySelectorAll('.range-btn'),
        refreshBtn: document.getElementById('refresh-btn'),
        c2cIntervalInput: document.getElementById('c2c-interval'),
        forexIntervalInput: document.getElementById('forex-interval'),
        forexMaxAgeInput: document.getElementById('forex-max-age'),
        adminTokenInput: document.getElementById('admin-token'),
        amountTagsContainer: document.getElementById('amount-tags'),
        newAmountInput: document.getElementById('new-amount'),
        addAmountBtn: document.getElementById('add-amount-btn'),
        saveConfigBtn: document.getElementById('save-config-btn'),
        saveStatus: document.getElementById('save-status'),
        mainChart: document.getElementById('main-chart'),
        alertStatusTableBody: document.querySelector('#alert-status-table tbody'),
        systemStatusIndicator: document.getElementById('system-status-indicator'),
        statusDetailsTooltip: document.querySelector('.status-details-tooltip'),
        appVersionBadge: document.getElementById('app-version-badge')
    };
}

// Tab Switching
function initTabs() {
    const el = getElements();
    el.tabs.forEach(btn => {
        btn.addEventListener('click', () => {
            const target = btn.dataset.tab;
            
            // Toggle Buttons
            el.tabs.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');

            // Toggle Content
            el.tabContents.forEach(c => {
                c.classList.remove('active');
                if (c.id === target) c.classList.add('active');
            });
            
            // Resize chart if showing dashboard
            if (target === 'dashboard' && state.chartInstance) {
                setTimeout(() => state.chartInstance.resize(), 100);
            }
            
            if (target === 'dashboard') {
                loadActiveAlerts();
                loadSystemStatus();
            }
        });
    });
}

// Chart Initialization
function initChart() {
    const el = getElements();
    state.chartInstance = echarts.init(el.mainChart);
    const option = {
        title: { text: 'C2C 价格趋势 vs 汇率' },
        tooltip: {
            trigger: 'axis',
            formatter: function (params) {
                if (!params || params.length === 0) return '';

                let result = `${escapeHTML(params[0].axisValueLabel)}<br/>`;
                let forexVal = null;
                
                // Find Forex Value first
                params.forEach(item => {
                    if (item.seriesName === FOREX_SERIES_NAME) {
                        forexVal = item.value[1];
                    }
                });

                params.forEach(item => {
                    const val = item.value[1];
                    let extra = '';
                    
                    // Forex just shows value
                    if (item.seriesName === FOREX_SERIES_NAME) {
                        result += `${item.marker} ${escapeHTML(item.seriesName)}: ${formatNumber(val)}<br/>`;
                        return;
                    }

                    // C2C Series
                    if (val) {
                         // value array: [date, price, merchant, min, max, pay, available]
                        const merchant = escapeHTML(item.value[2] || 'Unknown');
                        const min = item.value[3] || 0;
                        const max = item.value[4] || 0;
                        const pay = escapeHTML(item.value[5] || '-');
                        const avail = item.value[6] || 0;

                        if (forexVal) {
                            const diff = ((forexVal - val) / forexVal * 100).toFixed(2);
                            extra += ` <span style="font-weight:bold">(差价: ${diff}%)</span>`;
                        }
                        
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">商家: ${merchant}</span>`;
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">限额: ${formatNumber(min)} - ${formatNumber(max)} CNY</span>`;
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">可用: ${formatNumber(avail)} CNY</span>`;
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">支付: ${pay}</span>`;
                    }
                    result += `${item.marker} ${escapeHTML(item.seriesName)}: ${formatNumber(val)}${extra}<br/><br/>`;
                });
                return result;
            }
        },
        legend: { data: [] },
        grid: { left: '3%', right: '4%', top: 72, bottom: '3%', containLabel: true },
        xAxis: { type: 'time', boundaryGap: false },
        yAxis: { type: 'value', scale: true }, 
        series: [],
        media: [
            {
                query: { maxWidth: 480 },
                option: {
                    title: {
                        top: 8,
                        left: 8,
                        textStyle: { fontSize: 16 }
                    },
                    legend: {
                        top: 38,
                        left: 8,
                        right: 8,
                        type: 'scroll'
                    },
                    grid: {
                        left: 8,
                        right: 12,
                        top: 92,
                        bottom: 16,
                        containLabel: true
                    }
                }
            }
        ]
    };
    state.chartInstance.setOption(option);
    
    // Click Event
        state.chartInstance.on('click', function(params) {
        if (params.componentType === 'series' && params.seriesName !== FOREX_SERIES_NAME) {
             const val = params.value;
             // val: [date, price, merchant, min, max, pay, available]
             const date = val[0].toLocaleString();
             const price = val[1];
             const merchant = val[2];
             const min = val[3];
             const max = val[4];
             const pay = val[5];
             const avail = val[6];
             
             alert(`详细信息:\n\n时间: ${date}\n价格: ${price} CNY\n商家: ${merchant}\n限额: ${min} - ${max} CNY\n可用: ${avail} CNY\n支付: ${pay}`);
        }
    });
    
    // Responsive
    window.addEventListener('resize', () => {
        state.chartInstance.resize();
    });
}

// Event Bindings
function bindEvents() {
    const el = getElements();

    // Dashboard Controls
    if (el.amountSelect) {
        el.amountSelect.addEventListener('change', (e) => {
            state.currentAmount = Number(e.target.value);
            loadChartData();
        });
    }

    if (el.rangeBtns) {
        el.rangeBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                el.rangeBtns.forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                state.currentRange = btn.dataset.range;
                loadChartData();
            });
        });
    }

    if (el.refreshBtn) {
        el.refreshBtn.addEventListener('click', () => {
            loadChartData();
            loadActiveAlerts();
            loadSystemStatus();
        });
    }

    // Settings Controls
    if (el.addAmountBtn) {
        el.addAmountBtn.addEventListener('click', addAmountTag);
    }
    if (el.saveConfigBtn) {
        el.saveConfigBtn.addEventListener('click', saveConfig);
    }
    if (el.adminTokenInput) {
        el.adminTokenInput.value = sessionStorage.getItem('c2cAdminToken') || '';
        el.adminTokenInput.addEventListener('input', () => {
            const token = el.adminTokenInput.value.trim();
            if (token) {
                sessionStorage.setItem('c2cAdminToken', token);
            } else {
                sessionStorage.removeItem('c2cAdminToken');
            }
        });
    }
}

// API Calls
async function loadConfig() {
    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/config`);
        if (!response.ok) throw new Error('Failed to fetch config');

        const data = await response.json();
        const config = data.data || data;

        // Map backend field names (PascalCase) to frontend field names (snake_case)
        state.config = {
            c2c_interval_minutes: config.C2CIntervalMinutes || config.c2c_interval_minutes || 3,
            forex_interval_hours: config.ForexIntervalHours || config.forex_interval_hours || 1,
            forex_max_age_hours: config.ForexMaxAgeHours || config.forex_max_age_hours || 6,
            target_amounts: config.TargetAmounts || config.target_amounts || []
        };

        renderConfigUI();
    } catch (error) {
        console.error('Error loading config:', error);
    }
}

async function loadMeta() {
    const el = getElements();

    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/meta`);
        if (!response.ok) throw new Error('Failed to fetch app metadata');

        const data = await response.json();
        state.version = data.version || 'unknown';
        state.supportedExchanges = Array.isArray(data.supported_exchanges) ? data.supported_exchanges : [];
        state.historyKeys = data.history_keys || {};
        if (el.appVersionBadge) {
            el.appVersionBadge.textContent = state.version;
            if (data.summary) {
                el.appVersionBadge.title = data.summary;
            }
        }
    } catch (error) {
        console.error('Error loading app metadata:', error);
        if (el.appVersionBadge) {
            el.appVersionBadge.textContent = 'version unavailable';
        }
    }
}

async function loadActiveAlerts() {
    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/alerts/status`);
        if (!response.ok) throw new Error('Failed to fetch alert status');
        const json = await response.json();
        renderActiveAlerts(json.data);
    } catch (error) {
        console.error("Error loading alert status:", error);
    }
}

async function resetAlert(key) {
    if (!confirm('Are you sure you want to reset this dynamic threshold?')) return;

    const parts = parseAlertKey(key);
    if (!parts) {
        alert('Invalid alert key');
        return;
    }
    const token = getAdminToken();
    if (!token) return;

    const payload = {
        exchange: parts.exchange,
        side: parts.side,
        amount: parts.amount
    };

    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/alerts/reset`, {
            method: 'POST',
            headers: adminHeaders(token),
            body: JSON.stringify(payload)
        });
        
        if (response.ok) {
            loadActiveAlerts();
        } else if (response.status === 401) {
            clearAdminToken();
            alert('Admin token rejected');
        } else {
            const error = await readErrorResponse(response);
            alert(error || 'Failed to reset alert');
        }
    } catch (error) {
        console.error('Error resetting alert:', error);
    }
}

function renderActiveAlerts(states) {
    const el = getElements();
    if (!el.alertStatusTableBody) return;
    
    el.alertStatusTableBody.replaceChildren();
    
    if (!states || Object.keys(states).length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 3;
        td.className = 'empty-table-state';
        td.textContent = 'No active dynamic thresholds. (Using default percentage alerts)';
        tr.appendChild(td);
        el.alertStatusTableBody.appendChild(tr);
        return;
    }

    for (const [key, price] of Object.entries(states)) {
        const tr = document.createElement('tr');
        const keyCell = document.createElement('td');
        keyCell.textContent = key;
        const priceCell = document.createElement('td');
        const priceValue = document.createElement('strong');
        priceValue.textContent = Number(price).toFixed(4);
        priceCell.appendChild(priceValue);
        const actionCell = document.createElement('td');
        const resetButton = document.createElement('button');
        resetButton.type = 'button';
        resetButton.className = 'danger-btn compact-btn';
        resetButton.textContent = 'Reset';
        resetButton.addEventListener('click', () => resetAlert(key));
        actionCell.appendChild(resetButton);
        tr.append(keyCell, priceCell, actionCell);
        el.alertStatusTableBody.appendChild(tr);
    }
}

async function loadChartData() {
    if (state.currentAmount === null) return;
    
    state.chartInstance.showLoading();
    try {
        const url = `${AppConfig.apiBaseUrl}/api/v1/history?amount=${state.currentAmount}&range=${state.currentRange}`;
        const response = await fetch(url);
        if (!response.ok) throw new Error('Failed to fetch history');

        const json = await response.json();
        const data = json.data;

        updateChart(data);
    } catch (error) {
        console.error('Error loading history:', error);
        state.chartInstance.hideLoading();
    }
}

async function loadSystemStatus() {
    const el = getElements();
    if (!el.systemStatusIndicator) return;
    
    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/status`);
        if (!response.ok) throw new Error('Failed to fetch status');
        const json = await response.json();
        updateSystemStatusUI(json.data);
    } catch (error) {
        console.error("Error loading system status:", error);
        el.systemStatusIndicator.classList.remove('ok', 'loading');
        el.systemStatusIndicator.classList.add('error');
        el.systemStatusIndicator.querySelector('.status-text').textContent = "Connection Error";
    }
}

function updateSystemStatusUI(statusMap) {
    const el = getElements();
    if (!el.systemStatusIndicator || !el.statusDetailsTooltip) return;

    let hasError = false;
    let hasDegraded = false;

    if (!statusMap || Object.keys(statusMap).length === 0) {
        el.systemStatusIndicator.className = 'status-indicator loading';
        el.systemStatusIndicator.querySelector('.status-text').textContent = 'Waiting for data...';
        el.statusDetailsTooltip.replaceChildren(createTextElement('p', 'No status data available yet.', 'status-empty'));
        return;
    }

    const fragment = document.createDocumentFragment();
    fragment.appendChild(createTextElement('h4', 'Service Health'));

    for (const [key, val] of Object.entries(statusMap)) {
        if (val.status === 'Degraded') {
            hasDegraded = true;
        } else if (val.status !== 'OK') {
            hasError = true;
        }

        const lastCheck = new Date(val.last_check).toLocaleTimeString();
        const normalizedStatus = val.status === 'OK' ? 'ok' : (val.status === 'Degraded' ? 'degraded' : 'error');
        const statusText = val.status === 'OK' ? 'Operational' : val.status;

        const item = document.createElement('div');
        item.className = 'status-item';
        const details = document.createElement('div');
        details.appendChild(createTextElement('div', key, 'status-item-name'));
        details.appendChild(createTextElement('div', `Last check: ${lastCheck}`, 'status-item-time'));
        if (val.message) {
            details.appendChild(createTextElement('div', val.message, `status-item-message ${normalizedStatus}`));
        }
        const value = createTextElement('div', statusText, `status-item-val ${normalizedStatus}`);
        item.append(details, value);
        fragment.appendChild(item);
    }

    if (hasError) {
        el.systemStatusIndicator.className = 'status-indicator error';
        el.systemStatusIndicator.querySelector('.status-text').textContent = 'System Issues Detected';
    } else if (hasDegraded) {
        el.systemStatusIndicator.className = 'status-indicator degraded';
        el.systemStatusIndicator.querySelector('.status-text').textContent = 'Partial Data Degradation';
    } else {
        el.systemStatusIndicator.className = 'status-indicator ok';
        el.systemStatusIndicator.querySelector('.status-text').textContent = 'All Systems Operational';
    }

    el.statusDetailsTooltip.replaceChildren(fragment);
}

async function saveConfig() {
    const el = getElements();
    const c2cInterval = readPositiveInteger(el.c2cIntervalInput.value);
    const forexInterval = readPositiveInteger(el.forexIntervalInput.value);
    const forexMaxAge = readPositiveInteger(el.forexMaxAgeInput.value);
    if (c2cInterval === null || forexInterval === null || forexMaxAge === null) {
        el.saveStatus.textContent = 'Intervals and Forex maximum age must be positive integers';
        el.saveStatus.style.color = 'red';
        return;
    }
    if (state.config.target_amounts.length === 0) {
        el.saveStatus.textContent = 'Add at least one target amount';
        el.saveStatus.style.color = 'red';
        return;
    }

    const newConfig = {
        ...state.config,
        c2c_interval_minutes: c2cInterval,
        forex_interval_hours: forexInterval,
        forex_max_age_hours: forexMaxAge,
        target_amounts: state.config.target_amounts
    };
    const token = getAdminToken();
    if (!token) return;

    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/config`, {
            method: 'POST',
            headers: adminHeaders(token),
            body: JSON.stringify(newConfig)
        });
        
        if (response.ok) {
            el.saveStatus.textContent = 'Config Saved!';
            el.saveStatus.style.color = '';
            setTimeout(() => el.saveStatus.textContent = '', 3000);
            state.config = newConfig;
            renderConfigUI();
        } else if (response.status === 401) {
            clearAdminToken();
            el.saveStatus.textContent = 'Admin token rejected';
            el.saveStatus.style.color = 'red';
        } else {
            el.saveStatus.textContent = await readErrorResponse(response) || 'Save Failed';
            el.saveStatus.style.color = 'red';
        }
    } catch (error) {
        console.error('Error saving config:', error);
        el.saveStatus.textContent = 'Save Error';
    }
}

// UI Rendering
function renderConfigUI() {
    const el = getElements();
    
    // Update Settings Inputs
    if (el.c2cIntervalInput) el.c2cIntervalInput.value = state.config.c2c_interval_minutes;
    if (el.forexIntervalInput) el.forexIntervalInput.value = state.config.forex_interval_hours;
    if (el.forexMaxAgeInput) el.forexMaxAgeInput.value = state.config.forex_max_age_hours;

    // Render Tags in Settings
    if (el.amountTagsContainer) {
        el.amountTagsContainer.replaceChildren();
        const sortedAmounts = [...state.config.target_amounts].sort((a,b) => a-b);
        
        sortedAmounts.forEach(amt => {
            const tag = document.createElement('div');
            tag.className = 'tag';
            const label = amt === 0 ? "Lowest" : `${amt} CNY`;
            tag.appendChild(document.createTextNode(label));
            const removeButton = document.createElement('button');
            removeButton.type = 'button';
            removeButton.className = 'remove-tag';
            removeButton.setAttribute('aria-label', `Remove ${label}`);
            removeButton.textContent = '\u00d7';
            removeButton.addEventListener('click', () => removeAmountTag(amt));
            tag.appendChild(removeButton);
            el.amountTagsContainer.appendChild(tag);
        });
    }

    // Update Dashboard Selector
    if (el.amountSelect) {
        el.amountSelect.replaceChildren();
        const sortedAmounts = [...state.config.target_amounts].sort((a,b) => a-b);
        
        sortedAmounts.forEach(amt => {
            const option = document.createElement('option');
            option.value = amt;
            option.textContent = amt === 0 ? "Lowest (No Limit)" : `${amt} CNY`;
            if (amt === state.currentAmount) option.selected = true;
            el.amountSelect.appendChild(option);
        });
        
        // If current amount is not in list (e.g. deleted), pick first
        if (!state.config.target_amounts.includes(state.currentAmount) && state.config.target_amounts.length > 0) {
            state.currentAmount = state.config.target_amounts[0];
            el.amountSelect.value = state.currentAmount;
            loadChartData();
        }
    }
}

function updateChart(data) {
    const processData = (list) => {
        if (!list) return [];
        return list.map(item => [
            new Date(item.t * 1000), 
            item.v,
            item.merchant,
            item.min_amount,
            item.max_amount,
            item.pay_methods,
            item.available_amount
        ]);
    };

    const exchangeEntries = getExchangeEntries(data);
    const forexData = (data.forex || []).map(item => [new Date(item.t * 1000), item.v]);

    const buildSeries = (name, data, extra = {}) => ({
        name,
        type: 'line',
        data,
        showSymbol: data.length <= 1,
        symbolSize: data.length <= 1 ? 10 : 4,
        lineStyle: { width: 2 },
        ...extra
    });

    const exchangeSeries = exchangeEntries.map(({ name, historyKey }) => (
        buildSeries(name, processData(data[historyKey]))
    ));

    state.chartInstance.setOption({
        legend: { data: [...exchangeEntries.map(entry => entry.name), FOREX_SERIES_NAME] },
        series: [
            ...exchangeSeries,
            buildSeries(FOREX_SERIES_NAME, forexData, {
                itemStyle: { color: '#dc3545' },
                lineStyle: { type: 'dashed', width: 2 }
            })
        ]
    });
    state.chartInstance.hideLoading();
}

function getExchangeEntries(data) {
    if (state.supportedExchanges.length > 0) {
        return state.supportedExchanges.map(name => ({
            name,
            historyKey: state.historyKeys[name] || name.toLowerCase()
        }));
    }

    return Object.keys(data || {})
        .filter(key => key !== 'forex')
        .sort()
        .map(key => ({
            name: key.toUpperCase(),
            historyKey: key
        }));
}

// Logic for Settings Tags
function addAmountTag() {
    const el = getElements();
    const rawValue = el.newAmountInput.value.trim();
    if (rawValue === '') return;

    const val = Number(rawValue);
    if (Number.isFinite(val) && val >= 0 && !state.config.target_amounts.includes(val)) {
        state.config.target_amounts.push(val);
        el.newAmountInput.value = '';
        renderConfigUI();
    }
}

function removeAmountTag(amt) {
    state.config.target_amounts = state.config.target_amounts.filter(a => a !== amt);
    renderConfigUI();
}

function getAdminToken() {
    let token = (sessionStorage.getItem('c2cAdminToken') || '').trim();
    if (!token) {
        token = (window.prompt('Admin token') || '').trim();
        if (token) {
            sessionStorage.setItem('c2cAdminToken', token);
            const el = getElements();
            if (el.adminTokenInput) el.adminTokenInput.value = token;
        }
    }
    return token;
}

function clearAdminToken() {
    sessionStorage.removeItem('c2cAdminToken');
    const el = getElements();
    if (el.adminTokenInput) el.adminTokenInput.value = '';
}

function adminHeaders(token) {
    return {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json'
    };
}

async function readErrorResponse(response) {
    try {
        const payload = await response.json();
        return payload.error || '';
    } catch {
        return '';
    }
}

function parseAlertKey(key) {
    const match = /^(.+)-(BUY|SELL)-(.+)$/.exec(key);
    if (!match) return null;
    const amount = Number(match[3]);
    if (!Number.isFinite(amount) || amount < 0) return null;
    return { exchange: match[1], side: match[2], amount };
}

function createTextElement(tagName, text, className = '') {
    const element = document.createElement(tagName);
    if (className) element.className = className;
    element.textContent = text;
    return element;
}

function formatNumber(value) {
    if (value === null || value === undefined || value === '') return '-';
    const number = Number(value);
    return Number.isFinite(number) ? String(number) : '-';
}

function readPositiveInteger(value) {
    const rawValue = String(value).trim();
    if (rawValue === '') return null;
    const number = Number(rawValue);
    return Number.isInteger(number) && number > 0 ? number : null;
}

function escapeHTML(value) {
    return String(value)
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;')
        .replaceAll("'", '&#39;');
}
