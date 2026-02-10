// Global State
const state = {
    config: {
        c2c_interval_minutes: 3,
        forex_interval_hours: 1,
        target_amounts: []
    },
    currentAmount: null,
    currentRange: '1d',
    chartInstance: null
};

// DOM Elements
const elements = {
    tabs: document.querySelectorAll('.tab-btn'),
    tabContents: document.querySelectorAll('.tab-content'),
    amountSelect: document.getElementById('amount-select'),
    rangeBtns: document.querySelectorAll('.range-btn'),
    refreshBtn: document.getElementById('refresh-btn'),
    c2cIntervalInput: document.getElementById('c2c-interval'),
    forexIntervalInput: document.getElementById('forex-interval'),
    amountTagsContainer: document.getElementById('amount-tags'),
    newAmountInput: document.getElementById('new-amount'),
    addAmountBtn: document.getElementById('add-amount-btn'),
    saveConfigBtn: document.getElementById('save-config-btn'),
    saveStatus: document.getElementById('save-status'),
    mainChart: document.getElementById('main-chart')
};

// Initialization
document.addEventListener('DOMContentLoaded', () => {
    // Re-select elements on load in case of dynamic issues, though usually const elements defined above is fine if script runs after DOM
    // But since we use 'defer' or put script at bottom, it's fine.
    // Actually, to be safe against script placement, we should re-query inside DOMContentLoaded or move `elements` definition inside.
    // Let's move them inside to be safe.
    
    bindEvents();
    initTabs();
    initChart();
    
    loadConfig().then(() => {
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
        amountTagsContainer: document.getElementById('amount-tags'),
        newAmountInput: document.getElementById('new-amount'),
        addAmountBtn: document.getElementById('add-amount-btn'),
        saveConfigBtn: document.getElementById('save-config-btn'),
        saveStatus: document.getElementById('save-status'),
        mainChart: document.getElementById('main-chart'),
        alertStatusTableBody: document.querySelector('#alert-status-table tbody'),
        systemStatusIndicator: document.getElementById('system-status-indicator'),
        statusDetailsTooltip: document.querySelector('.status-details-tooltip')
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
                let result = params[0].axisValueLabel + '<br/>';
                let forexVal = null;
                
                // Find Forex Value first
                params.forEach(item => {
                    if (item.seriesName === 'USD/CNY 汇率') {
                        forexVal = item.value[1];
                    }
                });

                params.forEach(item => {
                    const val = item.value[1];
                    let extra = '';
                    
                    // Forex just shows value
                    if (item.seriesName === 'USD/CNY 汇率') {
                        result += `${item.marker} ${item.seriesName}: ${val}<br/>`;
                        return;
                    }

                    // C2C Series
                    if (val) {
                         // value array: [date, price, merchant, min, max, pay, available]
                        const merchant = item.value[2] || 'Unknown';
                        const min = item.value[3] || 0;
                        const max = item.value[4] || 0;
                        const pay = item.value[5] || '-';
                        const avail = item.value[6] || 0;

                        if (forexVal) {
                            const diff = ((forexVal - val) / forexVal * 100).toFixed(2);
                            extra += ` <span style="font-weight:bold">(差价: ${diff}%)</span>`;
                        }
                        
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">商家: ${merchant}</span>`;
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">限额: ${min} - ${max} CNY</span>`;
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">可用: ${avail} CNY</span>`;
                        extra += `<br/><span style="font-size:12px;color:#666;margin-left:14px">支付: ${pay}</span>`;
                    }
                    result += `${item.marker} ${item.seriesName}: ${val}${extra}<br/><br/>`;
                });
                return result;
            }
        },
        legend: { data: ['Binance', 'OKX', 'USD/CNY 汇率'] },
        grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
        xAxis: { type: 'time', boundaryGap: false },
        yAxis: { type: 'value', scale: true }, 
        series: []
    };
    state.chartInstance.setOption(option);
    
    // Click Event
    state.chartInstance.on('click', function(params) {
        if (params.componentType === 'series' && params.seriesName !== 'USD/CNY 汇率') {
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
            state.currentAmount = parseInt(e.target.value);
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
            target_amounts: config.TargetAmounts || config.target_amounts || []
        };

        renderConfigUI();
    } catch (error) {
        console.error('Error loading config:', error);
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
    
    // Key format: Exchange-Side-Amount (e.g., Binance-BUY-1000)
    const parts = key.split('-');
    if (parts.length !== 3) return;

    const payload = {
        exchange: parts[0],
        side: parts[1],
        amount: parseFloat(parts[2])
    };

    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/alerts/reset`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        
        if (response.ok) {
            loadActiveAlerts();
        } else {
            alert('Failed to reset alert');
        }
    } catch (error) {
        console.error('Error resetting alert:', error);
    }
}

function renderActiveAlerts(states) {
    const el = getElements();
    if (!el.alertStatusTableBody) return;
    
    el.alertStatusTableBody.innerHTML = '';
    
    if (!states || Object.keys(states).length === 0) {
        el.alertStatusTableBody.innerHTML = '<tr><td colspan="3" style="text-align:center; padding: 15px; color: #888;">No active dynamic thresholds. (Using default percentage alerts)</td></tr>';
        return;
    }

    for (const [key, price] of Object.entries(states)) {
        const tr = document.createElement('tr');
        tr.innerHTML = `
            <td style="padding: 10px; border-bottom: 1px solid #eee;">${key}</td>
            <td style="padding: 10px; border-bottom: 1px solid #eee;"><strong>${price.toFixed(4)}</strong></td>
            <td style="padding: 10px; border-bottom: 1px solid #eee;">
                <button class="primary-btn" style="padding: 5px 10px; font-size: 12px; background: #dc3545;" onclick="resetAlert('${key}')">Reset</button>
            </td>
        `;
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

    // Check if any error exists
    let allOk = true;
    let detailsHtml = '<h4>Service Health</h4>';

    // If map is empty, it means no data yet
    if (!statusMap || Object.keys(statusMap).length === 0) {
        el.systemStatusIndicator.className = 'status-indicator loading';
        el.systemStatusIndicator.querySelector('.status-text').textContent = 'Waiting for data...';
        el.statusDetailsTooltip.innerHTML = '<p style="font-size:12px; color:#666;">No status data available yet.</p>';
        return;
    }

    for (const [key, val] of Object.entries(statusMap)) {
        if (val.status !== 'OK') allOk = false;
        
        const lastCheck = new Date(val.last_check).toLocaleTimeString();
        const statusClass = val.status === 'OK' ? 'ok' : 'error';
        const statusText = val.status === 'OK' ? 'Operational' : 'Failed';
        
        detailsHtml += `
            <div class="status-item">
                <div>
                    <div class="status-item-name">${key}</div>
                    <div style="font-size:10px; color:#999;">Last check: ${lastCheck}</div>
                    ${val.message ? `<div style="font-size:10px; color:#dc3545; margin-top:2px;">${val.message}</div>` : ''}
                </div>
                <div class="status-item-val ${statusClass}">${statusText}</div>
            </div>
        `;
    }

    if (allOk) {
        el.systemStatusIndicator.className = 'status-indicator ok';
        el.systemStatusIndicator.querySelector('.status-text').textContent = 'All Systems Operational';
    } else {
        el.systemStatusIndicator.className = 'status-indicator error';
        el.systemStatusIndicator.querySelector('.status-text').textContent = 'System Issues Detected';
    }

    el.statusDetailsTooltip.innerHTML = detailsHtml;
}

async function saveConfig() {
    const el = getElements();
    const newConfig = {
        c2c_interval_minutes: parseInt(el.c2cIntervalInput.value),
        forex_interval_hours: parseInt(el.forexIntervalInput.value),
        target_amounts: state.config.target_amounts
    };

    try {
        const response = await fetch(`${AppConfig.apiBaseUrl}/api/config`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(newConfig)
        });
        
        if (response.ok) {
            el.saveStatus.textContent = 'Config Saved!';
            setTimeout(() => el.saveStatus.textContent = '', 3000);
            state.config = newConfig;
            renderConfigUI(); 
        } else {
            el.saveStatus.textContent = 'Save Failed';
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

    // Render Tags in Settings
    if (el.amountTagsContainer) {
        el.amountTagsContainer.innerHTML = '';
        const sortedAmounts = [...state.config.target_amounts].sort((a,b) => a-b);
        
        sortedAmounts.forEach(amt => {
            const tag = document.createElement('div');
            tag.className = 'tag';
            const label = amt === 0 ? "Lowest" : `${amt} CNY`;
            tag.innerHTML = `
                ${label} 
                <span class="remove-tag" onclick="removeAmountTag(${amt})">&times;</span>
            `;
            el.amountTagsContainer.appendChild(tag);
        });
    }

    // Update Dashboard Selector
    if (el.amountSelect) {
        el.amountSelect.innerHTML = '';
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

    const binanceData = processData(data.binance);
    const okxData = processData(data.okx);
    // Forex data only has [time, value]
    const forexData = (data.forex || []).map(item => [new Date(item.t * 1000), item.v]);

    state.chartInstance.setOption({
        series: [
            {
                name: 'Binance',
                type: 'line',
                data: binanceData,
                showSymbol: false,
                lineStyle: { width: 2 }
            },
            {
                name: 'OKX',
                type: 'line',
                data: okxData,
                showSymbol: false,
                lineStyle: { width: 2 }
            },
            {
                name: 'USD/CNY 汇率',
                type: 'line',
                data: forexData,
                showSymbol: false,
                itemStyle: { color: '#dc3545' },
                lineStyle: { type: 'dashed', width: 2 }
            }
        ]
    });
    state.chartInstance.hideLoading();
}

// Logic for Settings Tags
function addAmountTag() {
    const el = getElements();
    const val = parseInt(el.newAmountInput.value);
    if (!isNaN(val) && val >= 0 && !state.config.target_amounts.includes(val)) {
        state.config.target_amounts.push(val);
        el.newAmountInput.value = '';
        renderConfigUI();
    }
}

window.removeAmountTag = function(amt) {
    state.config.target_amounts = state.config.target_amounts.filter(a => a !== amt);
    renderConfigUI();
};