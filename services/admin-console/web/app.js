let activeView = 'dashboard';
let eventSource = null;
const expandedQueues = new Set();
const peekStatusHideTimers = new Map();
const attributeStatusHideTimers = new Map();
const queueAttributesCache = new Map();
const queueAttributeEditMode = new Set();

const editableQueueAttributeKeys = [
  'VisibilityTimeout',
  'MessageRetentionPeriod',
  'MaximumMessageSize',
  'DelaySeconds',
  'ReceiveMessageWaitTimeSeconds',
];

const queueAttributeRanges = {
  VisibilityTimeout: { min: 0, max: 43200 },
  MessageRetentionPeriod: { min: 60, max: 1209600 },
  MaximumMessageSize: { min: 1024, max: 262144 },
  DelaySeconds: { min: 0, max: 900 },
  ReceiveMessageWaitTimeSeconds: { min: 0, max: 20 },
};

const defaultCreateQueueAttributes = {
  VisibilityTimeout: 30,
  MessageRetentionPeriod: 345600,
  MaximumMessageSize: 262144,
  DelaySeconds: 0,
  ReceiveMessageWaitTimeSeconds: 0,
};

function setAlert(message, tone = 'error') {
  const container = document.getElementById('alerts');
  if (!message) {
    container.innerHTML = '';
    return;
  }
  const klass = tone === 'error'
    ? 'bg-red-50 text-red-800 border-red-200'
    : 'bg-emerald-50 text-emerald-800 border-emerald-200';
  container.innerHTML = `<div class="border rounded px-3 py-2 ${klass}">${message}</div>`;
}

function setStreamStatus(text) {
  document.getElementById('stream-status').textContent = `Stream: ${text}`;
}

function setActiveMenu(view) {
  document.querySelectorAll('.menu-btn').forEach(btn => btn.classList.remove('bg-slate-700'));
  const activeBtn = document.getElementById(`menu-${view}`);
  if (activeBtn) activeBtn.classList.add('bg-slate-700');
}

function switchView(view) {
  activeView = view;
  setActiveMenu(view);
  setAlert('');

  const title = document.getElementById('view-title');
  const subtitle = document.getElementById('view-subtitle');
  if (view === 'dashboard') {
    title.textContent = 'Dashboard';
    subtitle.textContent = 'Live status of active emulator surface';
    loadDashboard();
  } else {
    title.textContent = 'ess-queue-ess';
    subtitle.textContent = 'Queue operations and non-mutating message inspection';
    loadQueues();
  }

  connectSSE(view);
}

async function apiGet(path) {
  const response = await fetch(path);
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(payload.error || `HTTP ${response.status}`);
  }
  return response.json();
}

async function apiPost(path, body) {
  const response = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
    throw new Error(payload.error || `HTTP ${response.status}`);
  }
  return response.json().catch(() => ({}));
}

async function loadDashboard() {
  try {
    const data = await apiGet('/api/dashboard/summary');
    renderDashboard(data);
  } catch (error) {
    setAlert(error.message);
  }
}

function renderDashboard(data) {
  const serviceRows = (data.services || []).map((service) => {
    const badge = service.status === 'online'
      ? '<span class="px-2 py-1 rounded text-xs bg-emerald-100 text-emerald-800">online</span>'
      : '<span class="px-2 py-1 rounded text-xs bg-red-100 text-red-800">offline</span>';

    const stats = (service.stats || []).map((stat) => (
      `<li class="text-sm text-slate-700">${stat.label}: <span class="font-semibold">${stat.value}</span></li>`
    )).join('');

    return `
      <div class="bg-white rounded border p-4 flex items-start gap-4">
        <div class="w-48 shrink-0">
          <div class="font-medium">${service.name}</div>
          <div class="mt-1">${badge}</div>
        </div>
        <div class="flex-1">
          <ul class="space-y-1">
            ${stats || '<li class="text-sm text-slate-500">No stats available.</li>'}
          </ul>
        </div>
        <div class="shrink-0 ml-auto">
          <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Export service configuration" aria-label="Export service configuration" onclick="exportServiceConfig('${service.name}')">Export Config</button>
        </div>
      </div>
    `;
  }).join('');

  document.getElementById('view-content').innerHTML = `
    <div class="space-y-3">
      ${serviceRows || '<div class="bg-white rounded border p-4 text-sm text-slate-500">No service data.</div>'}
    </div>
    <div class="text-xs text-slate-500">Updated: ${new Date(data.updated_at).toLocaleString()}</div>
  `;
}

function exportServiceConfig(serviceName) {
  const anchor = document.createElement('a');
  anchor.href = `/api/services/${encodeURIComponent(serviceName)}/config/export`;
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

async function loadQueues() {
  try {
    const data = await apiGet('/api/services/ess-queue-ess/queues');
    renderQueuesIncremental(data.queues || []);
  } catch (error) {
    setAlert(error.message);
  }
}

function queueRowTemplate(queue) {
  const redriveButton = queue.is_dlq
    ? `<button class="px-3 py-1 rounded bg-purple-700 text-white text-sm" title="Move messages from this DLQ back to its source queue" aria-label="Start redrive" onclick="event.stopPropagation(); startRedrive('${queue.queue_url}', '${queue.queue_id}')">Start Redrive</button>`
    : '';

  const fifoControls = queue.is_fifo
    ? `<input id="group-${queue.queue_id}" class="w-full border rounded px-2 py-1 text-sm" placeholder="Message Group ID (required for FIFO)" />
       <input id="dedup-${queue.queue_id}" class="w-full border rounded px-2 py-1 text-sm" placeholder="Deduplication ID (optional)" />`
    : '';

  return `
    <div class="bg-white rounded border" data-queue-url="${queue.queue_url}" id="queue-${queue.queue_id}">
      <div class="px-4 py-3 flex items-start justify-between gap-3">
        <button class="flex-1 text-left" title="Expand or collapse queue details" aria-label="Toggle queue details" onclick="toggleQueue('${queue.queue_id}')">
          <div class="font-medium">${queue.queue_name}</div>
          <div class="text-xs text-slate-500">${queue.queue_url}</div>
        </button>
        <div class="flex flex-col items-end gap-2">
          <div class="flex items-center gap-2">
            <span class="px-2 py-1 text-xs rounded bg-slate-100">Visible: <span data-stat="visible">${queue.visible_count}</span></span>
            <span class="px-2 py-1 text-xs rounded bg-slate-100">In Flight: <span data-stat="in-flight">${queue.not_visible_count}</span></span>
            <span class="px-2 py-1 text-xs rounded bg-slate-100">Delayed: <span data-stat="delayed">${queue.delayed_count}</span></span>
          </div>
          <div class="flex items-center gap-2">
            <button class="px-3 py-1 rounded bg-amber-600 text-white text-sm" title="Remove all messages from this queue" aria-label="Purge queue" onclick="event.stopPropagation(); purgeQueue('${queue.queue_url}')">Purge</button>
            <button class="h-8 w-8 rounded bg-red-600 text-white leading-none flex items-center justify-center" title="Delete this queue" aria-label="Delete queue" onclick="event.stopPropagation(); deleteQueue('${queue.queue_url}')">
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18M8 6V4h8v2m-9 0l1 14h6l1-14" />
              </svg>
            </button>
          </div>
        </div>
      </div>
      <div class="border-t px-4 py-3 hidden" id="panel-${queue.queue_id}">
        <div class="grid gap-2 mb-3">
          <textarea id="body-${queue.queue_id}" class="w-full border rounded px-2 py-1 text-sm" rows="2" placeholder="Message body"></textarea>
          ${fifoControls}
          <div class="flex gap-2">
            <button class="px-3 py-1 rounded bg-blue-600 text-white text-sm" title="Send a message to this queue" aria-label="Send message" onclick="event.stopPropagation(); sendMessage('${queue.queue_url}', '${queue.queue_id}', ${queue.is_fifo})">Send</button>
            ${redriveButton}
          </div>
        </div>
        <div class="mb-3">
          <div class="text-sm font-medium mb-2 flex items-center justify-between">
            <span>Queue Attributes</span>
            <button class="h-7 w-7 rounded bg-indigo-700 text-white text-sm" title="Refresh attributes" aria-label="Refresh attributes" onclick="event.stopPropagation(); loadQueueAttributes('${queue.queue_id}')">↻</button>
          </div>
          <div class="flex gap-2 mb-2">
            <button id="attr-toggle-${queue.queue_id}" class="px-3 py-1 rounded bg-emerald-700 text-white text-sm" title="Edit queue attributes" aria-label="Edit queue attributes" onclick="event.stopPropagation(); toggleEditAttributes('${queue.queue_url}', '${queue.queue_id}')">Edit Attributes</button>
          </div>
          <div data-role="attr-status" class="hidden mb-2 text-xs px-2 py-1 rounded"></div>
          <div data-role="attributes-preview"><p class="text-sm text-slate-500">Use ↻ to fetch current SQS attributes.</p></div>
        </div>
        <div class="text-sm font-medium mb-2 flex items-center justify-between">
          <span>Messages (peek)</span>
          <button class="h-7 w-7 rounded bg-slate-700 text-white text-sm" title="Refresh peek" aria-label="Refresh peek" onclick="event.stopPropagation(); loadPeekMessages('${queue.queue_id}')">↻</button>
        </div>
        <div data-role="peek-status" class="hidden mb-2 text-xs px-2 py-1 rounded"></div>
        <div data-role="message-preview"><p class="text-sm text-slate-500">Use ↻ to load messages.</p></div>
      </div>
    </div>
  `;
}

function renderMessagePreview(messages) {
  if (!messages.length) return '<p class="text-sm text-slate-500">No visible messages.</p>';
  const rows = messages.slice(0, 10).map(msg => `
    <tr class="border-b">
      <td class="py-2 pr-2 text-xs">${msg.message_id || '-'}</td>
      <td class="py-2 pr-2 text-xs">${msg.body || ''}</td>
      <td class="py-2 text-xs">${msg.receive_count ?? 0}</td>
    </tr>
  `).join('');
  return `<div class="overflow-x-auto"><table class="w-full"><thead><tr class="text-xs text-slate-500"><th class="text-left py-1">Message ID</th><th class="text-left py-1">Body</th><th class="text-left py-1">Receive Count</th></tr></thead><tbody>${rows}</tbody></table></div>`;
}

function escapeHTML(input) {
  return String(input)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function validateQueueAttributeValue(name, value) {
  if (!Number.isFinite(value)) {
    return `${name} must be a number.`;
  }

  const range = queueAttributeRanges[name];
  if (!range) {
    return null;
  }

  if (value < range.min || value > range.max) {
    return `${name} must be between ${range.min} and ${range.max}.`;
  }

  return null;
}

function renderAttributesPreview(queueId, attributes) {
  if (!attributes || Object.keys(attributes).length === 0) {
    return '<p class="text-sm text-slate-500">No queue attributes found.</p>';
  }

  const visibleKeys = [
    'VisibilityTimeout',
    'MessageRetentionPeriod',
    'MaximumMessageSize',
    'DelaySeconds',
    'ReceiveMessageWaitTimeSeconds',
    'FifoQueue',
    'ContentBasedDeduplication',
    'RedrivePolicy',
    'RedriveAllowPolicy',
    'ApproximateNumberOfMessages',
    'ApproximateNumberOfMessagesNotVisible',
    'ApproximateNumberOfMessagesDelayed',
  ];

  const isEditing = queueAttributeEditMode.has(queueId);

  const rows = visibleKeys
    .filter((key) => Object.prototype.hasOwnProperty.call(attributes, key))
    .map((key) => {
      const rawValue = attributes[key] ?? '';
      const escapedValue = escapeHTML(rawValue);
      const isEditable = editableQueueAttributeKeys.includes(key);

      const valueCell = (isEditing && isEditable)
        ? `<input data-role="attr-input" data-attr-key="${key}" class="w-full border rounded px-2 py-1 text-xs" min="${queueAttributeRanges[key]?.min ?? ''}" max="${queueAttributeRanges[key]?.max ?? ''}" value="${escapedValue}" />`
        : `<span class="${isEditable ? 'font-medium' : ''}">${escapedValue}</span>`;

      return `
      <tr class="border-b">
        <td class="py-1 pr-3 text-xs font-medium">${key}</td>
        <td class="py-1 text-xs break-all">${valueCell}</td>
      </tr>
    `;
    })
    .join('');

  return `<div class="overflow-x-auto"><table class="w-full"><tbody>${rows}</tbody></table></div>`;
}

function renderQueuesIncremental(queues) {
  const content = document.getElementById('view-content');
  if (!content.querySelector('#queue-list')) {
    content.innerHTML = `
      <div class="bg-white rounded border p-4 mb-4">
        <h3 class="font-semibold mb-2">Create Queue</h3>
        <div class="grid md:grid-cols-3 gap-2 items-center mb-2">
          <input id="create-queue-name" class="border rounded px-2 py-1 text-sm" placeholder="Queue name" />
          <label class="text-sm flex items-center gap-2"><input type="checkbox" id="create-queue-fifo" /> FIFO</label>
          <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Create a new queue with these settings" aria-label="Create queue" onclick="createQueue()">Create Queue</button>
        </div>
        <div class="grid md:grid-cols-2 gap-2 items-center mb-2">
          <label class="text-sm flex items-center gap-2"><input type="checkbox" id="create-queue-with-dlq" onchange="toggleCreateDLQFields()" /> Create DLQ with queue</label>
          <label class="text-xs">DLQ Max Receive Count <span class="text-slate-400">(1+)</span>
            <input id="create-dlq-max-receive" class="w-full border rounded px-2 py-1 text-sm" type="number" min="1" value="3" disabled />
          </label>
        </div>
        <details class="mt-2 border rounded p-2">
          <summary class="text-sm font-medium cursor-pointer">Advanced Attributes</summary>
          <div class="mt-2">
            <button class="text-xs text-slate-600 underline" title="Reset advanced queue attributes to defaults" aria-label="Reset advanced defaults" onclick="event.stopPropagation(); resetCreateQueueAdvancedDefaults()">Reset to defaults</button>
          </div>
          <div class="grid md:grid-cols-2 lg:grid-cols-5 gap-2 mt-2">
            <label class="text-xs">VisibilityTimeout <span class="text-slate-400">(0-43200)</span>
              <input id="create-visibility-timeout" class="w-full border rounded px-2 py-1 text-sm" type="number" min="0" max="43200" value="${defaultCreateQueueAttributes.VisibilityTimeout}" />
            </label>
            <label class="text-xs">MessageRetentionPeriod <span class="text-slate-400">(60-1209600)</span>
              <input id="create-message-retention" class="w-full border rounded px-2 py-1 text-sm" type="number" min="60" max="1209600" value="${defaultCreateQueueAttributes.MessageRetentionPeriod}" />
            </label>
            <label class="text-xs">MaximumMessageSize <span class="text-slate-400">(1024-262144)</span>
              <input id="create-maximum-size" class="w-full border rounded px-2 py-1 text-sm" type="number" min="1024" max="262144" value="${defaultCreateQueueAttributes.MaximumMessageSize}" />
            </label>
            <label class="text-xs">DelaySeconds <span class="text-slate-400">(0-900)</span>
              <input id="create-delay-seconds" class="w-full border rounded px-2 py-1 text-sm" type="number" min="0" max="900" value="${defaultCreateQueueAttributes.DelaySeconds}" />
            </label>
            <label class="text-xs">ReceiveMessageWaitTimeSeconds <span class="text-slate-400">(0-20)</span>
              <input id="create-receive-wait" class="w-full border rounded px-2 py-1 text-sm" type="number" min="0" max="20" value="${defaultCreateQueueAttributes.ReceiveMessageWaitTimeSeconds}" />
            </label>
          </div>
        </details>
      </div>
      <div id="queue-list" class="space-y-3"></div>
    `;
  }

  const list = document.getElementById('queue-list');
  const incoming = new Map(queues.map(queue => [queue.queue_url, queue]));

  // Remove queues that no longer exist
  list.querySelectorAll('[data-queue-url]').forEach(node => {
    if (!incoming.has(node.dataset.queueUrl)) {
      node.remove();
    }
  });

  // Upsert rows
  queues.forEach(queue => {
    let row = list.querySelector(`[data-queue-url="${CSS.escape(queue.queue_url)}"]`);
    if (!row) {
      list.insertAdjacentHTML('beforeend', queueRowTemplate(queue));
      row = list.querySelector(`[data-queue-url="${CSS.escape(queue.queue_url)}"]`);
    } else {
      row.querySelector('[data-stat="visible"]').textContent = String(queue.visible_count);
      row.querySelector('[data-stat="in-flight"]').textContent = String(queue.not_visible_count);
      row.querySelector('[data-stat="delayed"]').textContent = String(queue.delayed_count);
    }

    const panel = document.getElementById(`panel-${queue.queue_id}`);
    if (!panel) {
      return;
    }
    if (expandedQueues.has(queue.queue_id)) {
      panel.classList.remove('hidden');
    } else {
      panel.classList.add('hidden');
    }
  });

  if (!queues.length) {
    list.innerHTML = '<div class="bg-white border rounded p-4 text-sm text-slate-500">No queues found.</div>';
  }
}

function setAttributeStatus(queueId, state, message = '') {
  const panel = document.getElementById(`panel-${queueId}`);
  if (!panel) return;

  const status = panel.querySelector('[data-role="attr-status"]');
  if (!status) return;

  const existingTimer = attributeStatusHideTimers.get(queueId);
  if (existingTimer) {
    clearTimeout(existingTimer);
    attributeStatusHideTimers.delete(queueId);
  }

  status.classList.remove('hidden', 'bg-slate-100', 'text-slate-700', 'bg-red-100', 'text-red-700', 'bg-emerald-100', 'text-emerald-700');
  if (state === 'idle') {
    status.classList.add('hidden');
    status.textContent = '';
    return;
  }

  if (state === 'loading') {
    status.classList.add('bg-slate-100', 'text-slate-700');
    status.textContent = message || 'Loading queue attributes...';
    return;
  }

  if (state === 'error') {
    status.classList.add('bg-red-100', 'text-red-700');
    status.textContent = message || 'Failed to load attributes.';
    return;
  }

  status.classList.add('bg-emerald-100', 'text-emerald-700');
  status.textContent = message || 'Attributes updated.';
  const hideTimer = setTimeout(() => {
    status.classList.add('hidden');
    status.textContent = '';
    attributeStatusHideTimers.delete(queueId);
  }, 3000);
  attributeStatusHideTimers.set(queueId, hideTimer);
}

async function loadQueueAttributes(queueId) {
  try {
    const panel = document.getElementById(`panel-${queueId}`);
    if (!panel) return;

    const preview = panel.querySelector('[data-role="attributes-preview"]');
    if (!preview) return;

    setAttributeStatus(queueId, 'loading');
    preview.innerHTML = '<p class="text-sm text-slate-500">Loading...</p>';
    const data = await apiGet(`/api/services/ess-queue-ess/queues/${encodeURIComponent(queueId)}/attributes`);
    queueAttributesCache.set(queueId, data.attributes || {});
    renderQueueAttributesPanel(queueId);
    setAttributeStatus(queueId, 'success');
  } catch (error) {
    setAttributeStatus(queueId, 'error', error.message);
    const panel = document.getElementById(`panel-${queueId}`);
    if (panel) {
      const preview = panel.querySelector('[data-role="attributes-preview"]');
      if (preview) {
        preview.innerHTML = '<p class="text-sm text-red-600">Unable to load queue attributes.</p>';
      }
    }
  }
}

function renderQueueAttributesPanel(queueId) {
  const panel = document.getElementById(`panel-${queueId}`);
  if (!panel) return;

  const preview = panel.querySelector('[data-role="attributes-preview"]');
  if (!preview) return;

  const cachedAttributes = queueAttributesCache.get(queueId) || {};
  preview.innerHTML = renderAttributesPreview(queueId, cachedAttributes);

  const toggleButton = document.getElementById(`attr-toggle-${queueId}`);
  if (toggleButton) {
    const editing = queueAttributeEditMode.has(queueId);
    toggleButton.textContent = editing ? 'Save Attributes' : 'Edit Attributes';
    toggleButton.title = editing ? 'Save queue attributes' : 'Edit queue attributes';
    toggleButton.setAttribute('aria-label', editing ? 'Save queue attributes' : 'Edit queue attributes');
    toggleButton.classList.remove('bg-emerald-700', 'bg-amber-700');
    toggleButton.classList.add(editing ? 'bg-amber-700' : 'bg-emerald-700');
  }
}

async function toggleEditAttributes(queueUrl, queueId) {
  if (!queueAttributeEditMode.has(queueId)) {
    if (!queueAttributesCache.has(queueId)) {
      await loadQueueAttributes(queueId);
    }
    queueAttributeEditMode.add(queueId);
    renderQueueAttributesPanel(queueId);
    return;
  }

  try {
    const panel = document.getElementById(`panel-${queueId}`);
    if (!panel) return;

    const inputs = panel.querySelectorAll('[data-role="attr-input"]');
    const values = {};
    inputs.forEach((input) => {
      const key = input.getAttribute('data-attr-key');
      if (key) {
        values[key] = Number(input.value);
      }
    });

    const visibility = values.VisibilityTimeout;
    const retention = values.MessageRetentionPeriod;
    const maxSize = values.MaximumMessageSize;
    const delay = values.DelaySeconds;
    const wait = values.ReceiveMessageWaitTimeSeconds;

    const inlineValidationError =
      validateQueueAttributeValue('VisibilityTimeout', visibility) ||
      validateQueueAttributeValue('MessageRetentionPeriod', retention) ||
      validateQueueAttributeValue('MaximumMessageSize', maxSize) ||
      validateQueueAttributeValue('DelaySeconds', delay) ||
      validateQueueAttributeValue('ReceiveMessageWaitTimeSeconds', wait);

    if (inlineValidationError) {
      setAttributeStatus(queueId, 'error', inlineValidationError);
      return;
    }

    await apiPost('/api/services/ess-queue-ess/actions/update-attributes', {
      queue_url: queueUrl,
      visibility_timeout: visibility,
      message_retention_period: retention,
      maximum_message_size: maxSize,
      delay_seconds: delay,
      receive_message_wait_time_seconds: wait,
    });

    queueAttributeEditMode.delete(queueId);

    const cached = queueAttributesCache.get(queueId) || {};
    cached.VisibilityTimeout = String(visibility);
    cached.MessageRetentionPeriod = String(retention);
    cached.MaximumMessageSize = String(maxSize);
    cached.DelaySeconds = String(delay);
    cached.ReceiveMessageWaitTimeSeconds = String(wait);
    queueAttributesCache.set(queueId, cached);
    renderQueueAttributesPanel(queueId);

    setAttributeStatus(queueId, 'success', 'Queue attributes saved.');
    await loadQueueAttributes(queueId);
  } catch (error) {
    setAttributeStatus(queueId, 'error', error.message);
  }
}

async function startRedrive(queueUrl, queueId) {
  try {
    await apiPost('/api/services/ess-queue-ess/actions/start-redrive', {
      queue_url: queueUrl,
      max_messages_per_second: 100,
    });
    setAttributeStatus(queueId, 'success', 'Redrive task started.');
    await Promise.all([loadPeekMessages(queueId), loadQueueAttributes(queueId)]);
  } catch (error) {
    setAttributeStatus(queueId, 'error', error.message);
  }
}

async function createQueue() {
  try {
    const queueNameInput = document.getElementById('create-queue-name');
    const isFifoInput = document.getElementById('create-queue-fifo');
    const visibilityInput = document.getElementById('create-visibility-timeout');
    const retentionInput = document.getElementById('create-message-retention');
    const maxSizeInput = document.getElementById('create-maximum-size');
    const delayInput = document.getElementById('create-delay-seconds');
    const waitInput = document.getElementById('create-receive-wait');
    const withDLQInput = document.getElementById('create-queue-with-dlq');
    const dlqMaxReceiveInput = document.getElementById('create-dlq-max-receive');

    const queueName = queueNameInput?.value?.trim();
    const isFIFO = !!isFifoInput?.checked;
    const createDLQ = !!withDLQInput?.checked;
    const visibilityTimeout = Number(visibilityInput?.value || 30);
    const messageRetentionPeriod = Number(retentionInput?.value || 345600);
    const maximumMessageSize = Number(maxSizeInput?.value || 262144);
    const delaySeconds = Number(delayInput?.value || 0);
    const receiveMessageWaitTimeSeconds = Number(waitInput?.value || 0);
    const dlqMaxReceiveCount = Number(dlqMaxReceiveInput?.value || 3);

    if (!queueName) {
      setAlert('Queue name is required');
      return;
    }

    if ([visibilityTimeout, messageRetentionPeriod, maximumMessageSize, delaySeconds, receiveMessageWaitTimeSeconds].some((value) => !Number.isFinite(value))) {
      setAlert('Advanced attributes must be numeric values');
      return;
    }

    if (createDLQ && (!Number.isFinite(dlqMaxReceiveCount) || dlqMaxReceiveCount < 1)) {
      setAlert('DLQ max receive count must be 1 or greater');
      return;
    }

    const createValidationError =
      validateQueueAttributeValue('VisibilityTimeout', visibilityTimeout) ||
      validateQueueAttributeValue('MessageRetentionPeriod', messageRetentionPeriod) ||
      validateQueueAttributeValue('MaximumMessageSize', maximumMessageSize) ||
      validateQueueAttributeValue('DelaySeconds', delaySeconds) ||
      validateQueueAttributeValue('ReceiveMessageWaitTimeSeconds', receiveMessageWaitTimeSeconds);

    if (createValidationError) {
      setAlert(createValidationError);
      return;
    }

    await apiPost('/api/services/ess-queue-ess/actions/create-queue', {
      queue_name: queueName,
      is_fifo: isFIFO,
      content_based_deduplication: isFIFO,
      create_dlq: createDLQ,
      dlq_max_receive_count: dlqMaxReceiveCount,
      visibility_timeout: visibilityTimeout,
      message_retention_period: messageRetentionPeriod,
      maximum_message_size: maximumMessageSize,
      delay_seconds: delaySeconds,
      receive_message_wait_time_seconds: receiveMessageWaitTimeSeconds,
    });

    setAlert(`Queue created: ${queueName}${createDLQ ? ' (with DLQ)' : ''}`, 'info');
    queueNameInput.value = '';
    if (isFifoInput) isFifoInput.checked = false;
    if (withDLQInput) withDLQInput.checked = false;
    if (dlqMaxReceiveInput) {
      dlqMaxReceiveInput.value = '3';
      dlqMaxReceiveInput.disabled = true;
    }
    await loadQueues();
  } catch (error) {
    setAlert(error.message);
  }
}

function toggleCreateDLQFields() {
  const withDLQInput = document.getElementById('create-queue-with-dlq');
  const dlqMaxReceiveInput = document.getElementById('create-dlq-max-receive');
  if (!withDLQInput || !dlqMaxReceiveInput) return;

  dlqMaxReceiveInput.disabled = !withDLQInput.checked;
  if (!withDLQInput.checked) {
    dlqMaxReceiveInput.value = '3';
  }
}

function resetCreateQueueAdvancedDefaults() {
  const visibilityInput = document.getElementById('create-visibility-timeout');
  const retentionInput = document.getElementById('create-message-retention');
  const maxSizeInput = document.getElementById('create-maximum-size');
  const delayInput = document.getElementById('create-delay-seconds');
  const waitInput = document.getElementById('create-receive-wait');
  const withDLQInput = document.getElementById('create-queue-with-dlq');
  const dlqMaxReceiveInput = document.getElementById('create-dlq-max-receive');

  if (visibilityInput) visibilityInput.value = String(defaultCreateQueueAttributes.VisibilityTimeout);
  if (retentionInput) retentionInput.value = String(defaultCreateQueueAttributes.MessageRetentionPeriod);
  if (maxSizeInput) maxSizeInput.value = String(defaultCreateQueueAttributes.MaximumMessageSize);
  if (delayInput) delayInput.value = String(defaultCreateQueueAttributes.DelaySeconds);
  if (waitInput) waitInput.value = String(defaultCreateQueueAttributes.ReceiveMessageWaitTimeSeconds);
  if (withDLQInput) withDLQInput.checked = false;
  if (dlqMaxReceiveInput) {
    dlqMaxReceiveInput.value = '3';
    dlqMaxReceiveInput.disabled = true;
  }

  setAlert('Advanced attributes reset to defaults.', 'info');
}

async function sendMessage(queueUrl, queueId, isFIFO) {
  try {
    const bodyInput = document.getElementById(`body-${queueId}`);
    const groupInput = document.getElementById(`group-${queueId}`);
    const dedupInput = document.getElementById(`dedup-${queueId}`);

    const messageBody = bodyInput?.value?.trim();
    const messageGroupId = groupInput?.value?.trim() || '';
    const messageDeduplicationId = dedupInput?.value?.trim() || '';

    if (!messageBody) {
      setAlert('Message body is required');
      return;
    }
    if (isFIFO && !messageGroupId) {
      setAlert('Message Group ID is required for FIFO queues');
      return;
    }

    await apiPost('/api/services/ess-queue-ess/actions/send-message', {
      queue_url: queueUrl,
      message_body: messageBody,
      message_group_id: messageGroupId,
      message_deduplication_id: messageDeduplicationId,
      delay_seconds: 0,
    });

    setAlert('Message sent', 'info');
    if (bodyInput) bodyInput.value = '';
    await loadQueues();
  } catch (error) {
    setAlert(error.message);
  }
}

async function purgeQueue(queueUrl) {
  try {
    if (!window.confirm(`Purge all messages from ${queueUrl}?`)) return;
    await apiPost('/api/services/ess-queue-ess/actions/purge-queue', { queue_url: queueUrl });
    setAlert(`Queue purged: ${queueUrl}`, 'info');
    await loadQueues();
  } catch (error) {
    setAlert(error.message);
  }
}

async function deleteQueue(queueUrl) {
  try {
    if (!window.confirm(`Delete queue ${queueUrl}?`)) return;
    await apiPost('/api/services/ess-queue-ess/actions/delete-queue', { queue_url: queueUrl });
    setAlert(`Queue deleted: ${queueUrl}`, 'info');
    await loadQueues();
  } catch (error) {
    setAlert(error.message);
  }
}

function toggleQueue(queueId) {
  const panel = document.getElementById(`panel-${queueId}`);
  if (!panel) return;
  const expanded = !panel.classList.contains('hidden');
  if (expanded) {
    panel.classList.add('hidden');
    expandedQueues.delete(queueId);
  } else {
    panel.classList.remove('hidden');
    expandedQueues.add(queueId);
    loadQueueAttributes(queueId);
    loadPeekMessages(queueId);
  }
}

async function loadPeekMessages(queueId) {
  const setPeekStatus = (panel, state, message = '') => {
    const status = panel.querySelector('[data-role="peek-status"]');
    if (!status) return;

    const existingTimer = peekStatusHideTimers.get(queueId);
    if (existingTimer) {
      clearTimeout(existingTimer);
      peekStatusHideTimers.delete(queueId);
    }

    status.classList.remove('hidden', 'bg-slate-100', 'text-slate-700', 'bg-red-100', 'text-red-700', 'bg-emerald-100', 'text-emerald-700');
    if (state === 'idle') {
      status.classList.add('hidden');
      status.textContent = '';
      return;
    }

    if (state === 'loading') {
      status.classList.add('bg-slate-100', 'text-slate-700');
      status.textContent = message || 'Loading latest peek messages...';
      return;
    }

    if (state === 'error') {
      status.classList.add('bg-red-100', 'text-red-700');
      status.textContent = message || 'Failed to load peek messages.';
      return;
    }

    status.classList.add('bg-emerald-100', 'text-emerald-700');
    status.textContent = message || 'Peek messages updated.';

    const hideTimer = setTimeout(() => {
      status.classList.add('hidden');
      status.textContent = '';
      peekStatusHideTimers.delete(queueId);
    }, 3000);
    peekStatusHideTimers.set(queueId, hideTimer);
  };

  try {
    const panel = document.getElementById(`panel-${queueId}`);
    if (!panel) return;

    const preview = panel.querySelector('[data-role="message-preview"]');
    if (!preview) return;

    setPeekStatus(panel, 'loading');
    preview.innerHTML = '<p class="text-sm text-slate-500">Loading...</p>';
    const data = await apiGet(`/api/services/ess-queue-ess/queues/${encodeURIComponent(queueId)}/messages/peek?limit=10`);
    preview.innerHTML = renderMessagePreview(data.messages || []);
    setPeekStatus(panel, 'success');
  } catch (error) {
    const panel = document.getElementById(`panel-${queueId}`);
    if (panel) {
      setPeekStatus(panel, 'error', error.message);
      const preview = panel.querySelector('[data-role="message-preview"]');
      if (preview) {
        preview.innerHTML = '<p class="text-sm text-red-600">Unable to load peek messages for this queue.</p>';
      }
    }
  }
}

function connectSSE(view) {
  if (eventSource) {
    eventSource.close();
  }
  eventSource = new EventSource(`/api/events?view=${encodeURIComponent(view)}`);
  setStreamStatus(`connecting (${view})`);

  eventSource.onopen = () => {
    setStreamStatus(`connected (${view})`);
  };

  eventSource.addEventListener('state', (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (view !== activeView) return;
      if (view === 'dashboard') {
        renderDashboard(payload);
      } else {
        renderQueuesIncremental(payload.queues || []);
      }
    } catch (error) {
      setAlert(`Failed to parse stream data: ${error.message}`);
    }
  });

  eventSource.onerror = () => {
    setStreamStatus(`retrying (${view})`);
  };
}

document.getElementById('menu-dashboard').addEventListener('click', () => switchView('dashboard'));
document.getElementById('menu-ess-queue-ess').addEventListener('click', () => switchView('ess-queue-ess'));

switchView('dashboard');
