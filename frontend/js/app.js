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
        mainChart: document.getElementById('main-chart')
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
                    if (forexVal && item.seriesName !== 'USD/CNY 汇率' && val) {
                        const diff = ((forexVal - val) / forexVal * 100).toFixed(2);
                        extra = ` (差价: ${diff}%)`;
                    }
                    result += `${item.marker} ${item.seriesName}: ${val}${extra}<br/>`;
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
        el.refreshBtn.addEventListener('click', loadChartData);
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

async function loadChartData() {
    if (!state.currentAmount) return;
    
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
        return list.map(item => [new Date(item.t * 1000), item.v]);
    };

    const binanceData = processData(data.binance);
    const okxData = processData(data.okx);
    const forexData = processData(data.forex);

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