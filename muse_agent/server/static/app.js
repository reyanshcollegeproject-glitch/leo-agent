// Meta Muse Autonomous Agent Web Dashboard Controller

const liveScreen = document.getElementById('live-screen');
const screenCoords = document.getElementById('screen-coords');
const btnRefresh = document.getElementById('btn-refresh-screen');
const goalInput = document.getElementById('goal-input');
const chkMock = document.getElementById('chk-mock');
const btnRun = document.getElementById('btn-run');
const btnPause = document.getElementById('btn-pause');
const btnResume = document.getElementById('btn-resume');
const btnStop = document.getElementById('btn-stop');
const agentStateEl = document.getElementById('agent-state');
const agentStateBadge = document.getElementById('agent-state-badge');
const currentThoughtEl = document.getElementById('current-thought');
const historyList = document.getElementById('history-list');
const stepCounter = document.getElementById('step-counter');
const actionPointer = document.getElementById('action-pointer');

let stepCount = 0;
let ws = null;

// Track cursor coordinates on viewport
liveScreen.addEventListener('mousemove', (e) => {
  const rect = liveScreen.getBoundingClientRect();
  const normX = Math.round(((e.clientX - rect.left) / rect.width) * 1000);
  const normY = Math.round(((e.clientY - rect.top) / rect.height) * 1000);
  screenCoords.textContent = `Norm Coords: (${normX}, ${normY})`;
});

// Refresh screen manually
btnRefresh.addEventListener('click', () => {
  liveScreen.src = `/api/screenshot?t=${Date.now()}`;
});

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws`;

  ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    console.log('Connected to Meta Muse Agent Telemetry Stream');
  };

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    handleServerEvent(msg);
  };

  ws.onclose = () => {
    console.warn('WebSocket disconnected. Retrying in 2 seconds...');
    setTimeout(connectWebSocket, 2000);
  };
}

function updateState(state) {
  agentStateEl.textContent = state.toUpperCase();
  const isRunning = (state === 'running');
  btnRun.disabled = isRunning;
  btnPause.disabled = !isRunning;
  btnResume.disabled = (state !== 'paused');
  btnStop.disabled = (state === 'idle' || state === 'completed' || state === 'stopped');

  if (state === 'running') {
    agentStateBadge.style.borderColor = 'var(--accent-cyan)';
  } else if (state === 'completed') {
    agentStateBadge.style.borderColor = 'var(--accent-green)';
  } else if (state === 'stopped' || state === 'failed') {
    agentStateBadge.style.borderColor = 'var(--accent-red)';
  }
}

function handleServerEvent(msg) {
  if (msg.type === 'initial_state' || msg.type === 'state_change') {
    updateState(msg.state);
  } else if (msg.type === 'step_telemetry') {
    const data = msg.data;
    stepCount = data.step;
    stepCounter.textContent = `Steps: ${stepCount}`;
    updateState(data.state);

    // Update thought
    if (data.action && data.action.thought) {
      currentThoughtEl.textContent = data.action.thought;
    }

    // Pulse action pointer on screen
    if (data.action && data.action.x !== null && data.action.y !== null) {
      showActionPulse(data.action.x, data.action.y);
    }

    // Refresh viewport image
    liveScreen.src = `/api/screenshot?t=${Date.now()}`;

    // Append to history list
    appendHistoryItem(data);
  } else if (msg.type === 'task_finished') {
    updateState(msg.data.state);
    liveScreen.src = `/api/screenshot?t=${Date.now()}`;
    const statusMsg = msg.data.success ? 'Goal Completed Successfully! 🎉' : 'Task Terminated / Failed ⚠️';
    currentThoughtEl.innerHTML = `<strong>${statusMsg}</strong> Execution time: ${msg.data.elapsed_seconds}s across ${msg.data.total_steps} steps.`;
  }
}

function showActionPulse(normX, normY) {
  const rect = liveScreen.getBoundingClientRect();
  const px = (normX / 1000.0) * rect.width;
  const py = (normY / 1000.0) * rect.height;

  actionPointer.style.left = `${px}px`;
  actionPointer.style.top = `${py}px`;
  actionPointer.classList.add('active');

  setTimeout(() => {
    actionPointer.classList.remove('active');
  }, 900);
}

function appendHistoryItem(data) {
  const emptyEl = historyList.querySelector('.empty');
  if (emptyEl) {
    emptyEl.remove();
  }

  const item = document.createElement('div');
  item.className = 'history-item';

  const action = data.action;
  const result = data.result;
  const diff = data.visual_diff;

  let actionDetail = '';
  if (action.x !== null && action.y !== null) {
    actionDetail = `coord: (${action.x}, ${action.y})`;
  } else if (action.text) {
    actionDetail = `text: "${action.text}"`;
  } else if (action.app_name) {
    actionDetail = `app: "${action.app_name}"`;
  } else if (action.hotkey) {
    actionDetail = `hotkey: "${action.hotkey}"`;
  }

  const diffTag = diff && diff.has_changed
    ? `<div class="result-box diff-changed">🔍 Delta: ${(diff.diff_ratio * 100).toFixed(2)}% screen changed</div>`
    : '';

  item.innerHTML = `
    <div class="history-step-head">
      <span class="step-tag">Step #${data.step}</span>
      <span class="action-pill">${action.action.toUpperCase()}</span>
    </div>
    <div class="action-detail">${actionDetail}</div>
    <div class="result-box">✓ ${result.message || 'Executed'}</div>
    ${diffTag}
  `;

  historyList.prepend(item);
}

// User Actions
btnRun.addEventListener('click', async () => {
  const goal = goalInput.value.trim();
  if (!goal) {
    alert('Please enter a goal for Meta Muse.');
    return;
  }

  historyList.innerHTML = '';
  stepCount = 0;
  stepCounter.textContent = 'Steps: 0';
  updateState('running');
  currentThoughtEl.textContent = 'Analyzing objective and planning tactical execution...';

  try {
    const res = await fetch('/api/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        goal: goal,
        use_mock: chkMock.checked,
        max_steps: 25
      })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Failed to start: ${err.detail}`);
      updateState('idle');
    }
  } catch (e) {
    alert(`Network error: ${e}`);
    updateState('idle');
  }
});

btnPause.addEventListener('click', async () => {
  await fetch('/api/pause', { method: 'POST' });
  updateState('paused');
});

btnResume.addEventListener('click', async () => {
  await fetch('/api/resume', { method: 'POST' });
  updateState('running');
});

btnStop.addEventListener('click', async () => {
  if (confirm('Trigger Sentinel Emergency Kill Switch?')) {
    await fetch('/api/stop', { method: 'POST' });
    updateState('stopped');
  }
});

// Init
connectWebSocket();

