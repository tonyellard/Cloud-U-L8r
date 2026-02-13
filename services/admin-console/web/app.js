let activeView = 'dashboard';
let eventSource = null;
const expandedQueues = new Set();
const peekStatusHideTimers = new Map();
const attributeStatusHideTimers = new Map();
const queueAttributesCache = new Map();
const queueAttributeEditMode = new Set();
let subscriptionQueueTargets = [];
let activeTopicActivityARN = '';
let activeTopicActivityName = '';
let expandedPubSubTopicARN = '';
let latestPubSubState = null;

function displayServiceName(name) {
  if (name === 'essthree') return 'ess-three';
  return name;
}

const repoBaseURL = 'https://github.com/tonyellard/Cloud-U-L8r';

function getServiceReadmeURL(serviceName) {
  const readmePaths = {
    'essthree': 'services/essthree/README.md',
    'cloudfauxnt': 'services/cloudfauxnt/README.md',
    'ess-queue-ess': 'services/ess-queue-ess/README.md',
    'ess-enn-ess': 'services/ess-enn-ess/README.md',
    'admin-console': 'services/admin-console/README.md',
  };

  const readmePath = readmePaths[serviceName];
  if (!readmePath) return '';
  return `${repoBaseURL}/blob/main/${readmePath}`;
}

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
  } else if (view === 'ess-queue-ess') {
    title.textContent = 'ess-queue-ess';
    subtitle.textContent = 'Queue operations and non-mutating message inspection';
    loadQueues();
  } else if (view === 'ess-enn-ess') {
    title.textContent = 'ess-enn-ess';
    subtitle.textContent = 'Topic, subscription, and publish operations';
    loadPubSubState();
  } else if (view === 'essthree') {
    title.textContent = 'ess-three';
    subtitle.textContent = 'Informational S3 surface summary (more admin actions coming soon)';
    loadEssThreeSummary();
  } else {
    title.textContent = 'cloudfauxnt';
    subtitle.textContent = 'Informational CDN/origin overview (more admin actions coming soon)';
    loadCloudfauxntSummary();
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
    const serviceReadmeURL = getServiceReadmeURL(service.name);
    const badge = service.status === 'online'
      ? '<span class="px-2 py-1 rounded text-xs bg-emerald-100 text-emerald-800">online</span>'
      : '<span class="px-2 py-1 rounded text-xs bg-red-100 text-red-800">offline</span>';

    const stats = (service.stats || []).map((stat) => (
      `<li class="text-sm text-slate-700">${stat.label}: <span class="font-semibold">${stat.value}</span></li>`
    )).join('');

    return `
      <div class="bg-white rounded border p-4 flex items-start gap-4">
        <div class="w-48 shrink-0">
          <div class="font-medium">${displayServiceName(service.name)}</div>
          <div class="mt-1">${badge}</div>
        </div>
        <div class="flex-1">
          <ul class="space-y-1">
            ${stats || '<li class="text-sm text-slate-500">No stats available.</li>'}
          </ul>
        </div>
        <div class="shrink-0 ml-auto">
          <div class="flex items-center gap-2">
            ${serviceReadmeURL ? `<a class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm hover:bg-slate-50" title="Open service README on GitHub" aria-label="Open service README on GitHub" href="${serviceReadmeURL}" target="_blank" rel="noopener noreferrer">README</a>` : ''}
            <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Export service configuration" aria-label="Export service configuration" onclick="exportServiceConfig('${service.name}')">Export Config</button>
          </div>
        </div>
      </div>
    `;
  }).join('');

  document.getElementById('view-content').innerHTML = `
    <div class="bg-white rounded border p-3 flex items-center justify-between">
      <div class="text-sm text-slate-600">Repository documentation</div>
      <a class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm hover:bg-slate-50" title="Open main repository README on GitHub" aria-label="Open main repository README on GitHub" href="${repoBaseURL}/blob/main/README.md" target="_blank" rel="noopener noreferrer">Main README</a>
    </div>
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

async function loadPubSubState() {
  try {
    const data = await apiGet('/api/services/ess-enn-ess/state');
    renderPubSubState(data);
  } catch (error) {
    setAlert(error.message);
  }
}

async function loadEssThreeSummary() {
  try {
    const data = await apiGet('/api/services/essthree/summary');
    renderEssThreeSummary(data);
  } catch (error) {
    setAlert(error.message);
  }
}

async function loadCloudfauxntSummary() {
  try {
    const data = await apiGet('/api/services/cloudfauxnt/summary');
    renderCloudfauxntSummary(data);
  } catch (error) {
    setAlert(error.message);
  }
}

function renderFutureBanner(text) {
  return `<div class="border border-amber-200 bg-amber-50 text-amber-800 rounded px-3 py-2 text-sm">${text}</div>`;
}

function renderEssThreeSummary(data) {
  const buckets = data.buckets || [];
  const rows = buckets.map((bucket) => `
    <tr class="border-b">
      <td class="py-2 pr-2 text-sm">${escapeHTML(bucket.name || '')}</td>
      <td class="py-2 text-sm text-right">${Number(bucket.object_count || 0)}</td>
    </tr>
  `).join('');

  document.getElementById('view-content').innerHTML = `
    ${renderFutureBanner('ess-three admin is currently informational. Additional admin actions will be added in a future update.')}
    <div class="grid grid-cols-2 gap-4">
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Buckets</div>
        <div class="text-2xl font-semibold">${Number(data.stats?.buckets || 0)}</div>
      </div>
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Objects</div>
        <div class="text-2xl font-semibold">${Number(data.stats?.objects || 0)}</div>
      </div>
    </div>
    <div class="bg-white rounded border p-4">
      <div class="flex items-center justify-between mb-2">
        <h3 class="font-semibold">Bucket Overview</h3>
        <button class="h-7 w-7 rounded bg-slate-700 text-white text-sm" title="Refresh bucket summary" aria-label="Refresh bucket summary" onclick="loadEssThreeSummary()">↻</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="text-xs text-slate-500 border-b">
              <th class="text-left py-1 pr-2">Bucket</th>
              <th class="text-right py-1">Object Count</th>
            </tr>
          </thead>
          <tbody>
            ${rows || '<tr><td colspan="2" class="py-2 text-sm text-slate-500">No buckets found.</td></tr>'}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

function renderCloudfauxntSummary(data) {
  const origins = data.origins || [];
  const rows = origins.map((origin) => `
    <tr class="border-b align-top">
      <td class="py-2 pr-2 text-sm">${escapeHTML(origin.name || '')}</td>
      <td class="py-2 pr-2 text-sm break-all">${escapeHTML(origin.url || '')}</td>
      <td class="py-2 pr-2 text-sm">${(origin.path_patterns || []).map((pattern) => `<div>${escapeHTML(pattern)}</div>`).join('')}</td>
      <td class="py-2 pr-2 text-sm">${origin.require_signature ? 'required' : 'not required'}</td>
      <td class="py-2 text-sm">${escapeHTML(origin.default_root_object || data.server?.default_root_object || '-')}</td>
    </tr>
  `).join('');

  document.getElementById('view-content').innerHTML = `
    ${renderFutureBanner('cloudfauxnt admin is currently informational. Additional admin actions will be added in a future update.')}
    <div class="grid grid-cols-3 gap-4">
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Origins</div>
        <div class="text-2xl font-semibold">${Number(data.stats?.origins || 0)}</div>
      </div>
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Behaviors</div>
        <div class="text-2xl font-semibold">${Number(data.stats?.behaviors || 0)}</div>
      </div>
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Signing</div>
        <div class="text-2xl font-semibold">${data.signing?.enabled ? 'On' : 'Off'}</div>
      </div>
    </div>
    <div class="bg-white rounded border p-4">
      <div class="flex items-center justify-between mb-2">
        <h3 class="font-semibold">Origin & Behavior Overview</h3>
        <button class="h-7 w-7 rounded bg-slate-700 text-white text-sm" title="Refresh cloudfauxnt overview" aria-label="Refresh cloudfauxnt overview" onclick="loadCloudfauxntSummary()">↻</button>
      </div>
      <p class="text-xs text-slate-500 mb-2">Server: ${escapeHTML(data.server?.host || '')}:${Number(data.server?.port || 0)}</p>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="text-xs text-slate-500 border-b">
              <th class="text-left py-1 pr-2">Origin</th>
              <th class="text-left py-1 pr-2">URL</th>
              <th class="text-left py-1 pr-2">Behaviors</th>
              <th class="text-left py-1 pr-2">Signature</th>
              <th class="text-left py-1">Default Root</th>
            </tr>
          </thead>
          <tbody>
            ${rows || '<tr><td colspan="5" class="py-2 text-sm text-slate-500">No origins configured.</td></tr>'}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

function renderPubSubState(data) {
  latestPubSubState = data;
  const content = document.getElementById('view-content');
  if (!content.querySelector('#pubsub-shell')) {
    content.innerHTML = `
      <div id="pubsub-shell" class="space-y-4">
        <div class="grid lg:grid-cols-2 gap-4">
          <div class="bg-white rounded border p-4">
            <h3 class="font-semibold mb-2">Create Topic</h3>
            <div class="flex gap-2">
              <input id="topic-create-name" class="flex-1 border rounded px-2 py-1 text-sm" placeholder="Topic name" />
              <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Create topic" aria-label="Create topic" onclick="createTopic()">Create</button>
            </div>
          </div>
          <div class="bg-white rounded border p-4">
            <h3 class="font-semibold mb-2">Publish Message</h3>
            <div class="grid gap-2">
              <select id="publish-topic-arn" class="border rounded px-2 py-1 text-sm"></select>
              <input id="publish-subject" class="border rounded px-2 py-1 text-sm" placeholder="Subject (optional)" />
              <textarea id="publish-message" class="border rounded px-2 py-1 text-sm" rows="2" placeholder="Message body"></textarea>
              <div class="flex justify-end">
                <button class="px-3 py-1 rounded bg-blue-600 text-white text-sm" title="Publish message to selected topic" aria-label="Publish message" onclick="publishTopicMessage()">Publish</button>
              </div>
            </div>
          </div>
        </div>

        <div class="bg-white rounded border p-4">
          <div class="flex items-center justify-between mb-2">
            <h3 class="font-semibold">Topics</h3>
            <div class="flex items-center gap-2">
              <span id="topics-count" class="text-xs text-slate-500">0 topics</span>
              <button class="h-7 w-7 rounded bg-slate-700 text-white text-sm" title="Refresh topics and subscriptions" aria-label="Refresh topics and subscriptions" onclick="loadPubSubState()">↻</button>
            </div>
          </div>
          <div class="overflow-x-auto">
            <table class="w-full">
              <thead>
                <tr class="text-xs text-slate-500 border-b">
                  <th class="text-left py-1 pr-2">Expand</th>
                  <th class="text-left py-1 pr-2">Topic ARN</th>
                  <th class="text-left py-1 pr-2">Display Name</th>
                  <th class="text-left py-1 pr-2">Subscriptions</th>
                  <th class="text-left py-1 pr-2">Type</th>
                  <th class="text-right py-1">Actions</th>
                </tr>
              </thead>
              <tbody id="topics-body">
                <tr><td colspan="6" class="py-2 text-sm text-slate-500">No topics found.</td></tr>
              </tbody>
            </table>
          </div>
        </div>

        <div id="topic-activity-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50">
          <div class="bg-white w-full max-w-5xl rounded border shadow-lg">
            <div class="px-4 py-3 border-b flex items-center justify-between">
              <div>
                <h3 class="font-semibold">Topic Activity</h3>
                <p id="topic-activity-subtitle" class="text-xs text-slate-500"></p>
              </div>
              <div class="flex items-center gap-2">
                <button class="h-7 w-7 rounded bg-slate-700 text-white text-sm" title="Refresh topic activity" aria-label="Refresh topic activity" onclick="refreshTopicActivityModal()">↻</button>
                <button class="h-7 w-7 rounded bg-slate-200 text-slate-700 text-sm" title="Close activity modal" aria-label="Close activity modal" onclick="closeTopicActivityModal()">✕</button>
              </div>
            </div>
            <div class="p-4">
              <div id="topic-activity-status" class="hidden mb-2 text-xs px-2 py-1 rounded"></div>
              <div id="topic-activity-body" class="max-h-[60vh] overflow-auto">
                <p class="text-sm text-slate-500">No topic selected.</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    `;
  }

  const topics = data.topics || [];
  const subscriptions = data.subscriptions || [];
  const subscriptionsByTopic = groupSubscriptionsByTopic(subscriptions);
  const subscriptionCounts = buildSubscriptionCountMap(subscriptionsByTopic);

  renderTopicsTable(topics, subscriptionsByTopic, subscriptionCounts);
  syncTopicSelector('publish-topic-arn', topics);
  loadSubscriptionQueueTargets();

  const topicsCount = document.getElementById('topics-count');
  if (topicsCount) {
    topicsCount.textContent = `${topics.length} topic${topics.length === 1 ? '' : 's'}`;
  }
}

async function loadSubscriptionQueueTargets() {
  try {
    const data = await apiGet('/api/services/ess-queue-ess/queues');
    subscriptionQueueTargets = data.queues || [];
    const queueTargetSelects = document.querySelectorAll('[id^="subscription-queue-target-"]');
    queueTargetSelects.forEach((select) => {
      const topicKey = select.id.replace('subscription-queue-target-', '');
      syncSubscriptionQueueTargetsForTopic(topicKey);
    });
  } catch {
    subscriptionQueueTargets = [];
    const queueTargetSelects = document.querySelectorAll('[id^="subscription-queue-target-"]');
    queueTargetSelects.forEach((select) => {
      const topicKey = select.id.replace('subscription-queue-target-', '');
      syncSubscriptionQueueTargetsForTopic(topicKey);
    });
  }
}

function groupSubscriptionsByTopic(subscriptions) {
  const grouped = new Map();
  subscriptions.forEach((subscription) => {
    const topicArn = subscription.topic_arn || '';
    if (!grouped.has(topicArn)) {
      grouped.set(topicArn, []);
    }
    grouped.get(topicArn).push(subscription);
  });
  return grouped;
}

function buildSubscriptionCountMap(subscriptionsByTopic) {
  const counts = new Map();
  subscriptionsByTopic.forEach((list, topicArn) => {
    counts.set(topicArn, list.length);
  });
  return counts;
}

function getTopicDomKey(topicArn) {
  return btoa(topicArn).replaceAll('=', '');
}

function syncSubscriptionQueueTargetsForTopic(topicKey) {
  const select = document.getElementById(`subscription-queue-target-${topicKey}`);
  if (!select) return;

  const existingValue = select.value;
  const options = subscriptionQueueTargets.map((queue) => {
    const queueName = queue.queue_name || queue.queue_url || '';
    const queueURL = queue.queue_url || '';
    return `<option value="${escapeHTML(queueURL)}">${escapeHTML(queueName)}</option>`;
  }).join('');

  select.innerHTML = options || '<option value="">No queues available</option>';
  if (existingValue && subscriptionQueueTargets.some((queue) => queue.queue_url === existingValue)) {
    select.value = existingValue;
  }
}

function onTopicSubscriptionProtocolChange(topicKey) {
  const protocolSelect = document.getElementById(`subscription-protocol-${topicKey}`);
  const endpointInput = document.getElementById(`subscription-endpoint-${topicKey}`);
  const queueTargetSelect = document.getElementById(`subscription-queue-target-${topicKey}`);
  if (!protocolSelect || !endpointInput || !queueTargetSelect) return;

  const useQueueTarget = protocolSelect.value === 'ess-queue-ess';
  endpointInput.classList.toggle('hidden', useQueueTarget);
  queueTargetSelect.classList.toggle('hidden', !useQueueTarget);

  if (useQueueTarget) {
    syncSubscriptionQueueTargetsForTopic(topicKey);
  }
}

function renderTopicSubscriptionPanel(topicArn, topicKey, subscriptions) {
  const rows = subscriptions.map((subscription) => `
    <tr class="border-b">
      <td class="py-2 pr-2 text-xs break-all">${escapeHTML(subscription.subscription_arn || '')}</td>
      <td class="py-2 pr-2 text-xs">${escapeHTML(subscription.protocol || '')}</td>
      <td class="py-2 pr-2 text-xs break-all">${escapeHTML(subscription.endpoint || '')}</td>
      <td class="py-2 pr-2 text-xs">${escapeHTML(subscription.status || '')}</td>
      <td class="py-2 text-right">
        <button class="h-8 w-8 rounded bg-red-600 text-white leading-none inline-flex items-center justify-center" title="Delete subscription" aria-label="Delete subscription" onclick="deleteSubscription('${encodeURIComponent(subscription.subscription_arn || '')}')">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18M8 6V4h8v2m-9 0l1 14h6l1-14" />
          </svg>
        </button>
      </td>
    </tr>
  `).join('');

  return `
    <div class="bg-slate-50 border rounded p-3">
      <div class="grid md:grid-cols-4 gap-2 mb-3">
        <select id="subscription-protocol-${topicKey}" class="border rounded px-2 py-1 text-sm" onchange="onTopicSubscriptionProtocolChange('${topicKey}')">
          <option value="http">http</option>
          <option value="ess-queue-ess">Ess-Queue-Ess</option>
        </select>
        <input id="subscription-endpoint-${topicKey}" class="border rounded px-2 py-1 text-sm" placeholder="HTTP endpoint" />
        <select id="subscription-queue-target-${topicKey}" class="border rounded px-2 py-1 text-sm hidden"></select>
        <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Create subscription for this topic" aria-label="Create subscription for this topic" onclick="createSubscriptionForTopic('${encodeURIComponent(topicArn)}', '${topicKey}')">Add Subscription</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="text-xs text-slate-500 border-b">
              <th class="text-left py-1 pr-2">Subscription ARN</th>
              <th class="text-left py-1 pr-2">Protocol</th>
              <th class="text-left py-1 pr-2">Endpoint</th>
              <th class="text-left py-1 pr-2">Status</th>
              <th class="text-right py-1">Actions</th>
            </tr>
          </thead>
          <tbody>
            ${rows || '<tr><td colspan="5" class="py-2 text-sm text-slate-500">No subscriptions for this topic.</td></tr>'}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

function renderTopicsTable(topics, subscriptionsByTopic, subscriptionCounts) {
  const body = document.getElementById('topics-body');
  if (!body) return;

  if (!topics.length) {
    body.innerHTML = '<tr><td colspan="6" class="py-2 text-sm text-slate-500">No topics found.</td></tr>';
    return;
  }

  const rows = topics.map((topic) => {
    const topicArn = topic.topic_arn || '';
    const topicKey = getTopicDomKey(topicArn);
    const isExpanded = expandedPubSubTopicARN === topicArn;
    const topicSubscriptions = subscriptionsByTopic.get(topicArn) || [];
    const subscriptionCount = subscriptionCounts.get(topicArn) ?? topicSubscriptions.length;
    const typeLabel = topic.fifo_topic ? 'FIFO' : 'Standard';
    const encodedTopicArn = encodeURIComponent(topicArn);
    const displayName = (topic.display_name || '').replaceAll("'", '&#39;');
    return `
      <tr class="border-b">
        <td class="py-2 pr-2 text-xs">
          <button class="h-7 w-7 rounded bg-slate-700 text-white text-sm" title="${isExpanded ? 'Collapse subscriptions' : 'Expand subscriptions'}" aria-label="${isExpanded ? 'Collapse subscriptions' : 'Expand subscriptions'}" onclick="toggleTopicSubscriptions('${encodedTopicArn}')">${isExpanded ? '−' : '+'}</button>
        </td>
        <td class="py-2 pr-2 text-xs break-all">${escapeHTML(topicArn)}</td>
        <td class="py-2 pr-2 text-xs">${escapeHTML(topic.display_name || '')}</td>
        <td class="py-2 pr-2 text-xs">${subscriptionCount}</td>
        <td class="py-2 pr-2 text-xs">${typeLabel}</td>
        <td class="py-2 text-right">
          <button class="px-2 py-1 rounded bg-indigo-700 text-white text-xs mr-2" title="View activity details for this topic" aria-label="View topic activity" onclick="openTopicActivityModal('${encodedTopicArn}', '${displayName}')">Activity</button>
          <button class="h-8 w-8 rounded bg-red-600 text-white leading-none inline-flex items-center justify-center" title="Delete topic" aria-label="Delete topic" onclick="deleteTopic('${encodedTopicArn}')">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18M8 6V4h8v2m-9 0l1 14h6l1-14" />
            </svg>
          </button>
        </td>
      </tr>
      <tr class="${isExpanded ? '' : 'hidden'}" id="topic-subscriptions-${topicKey}">
        <td colspan="6" class="py-2 px-1">
          ${renderTopicSubscriptionPanel(topicArn, topicKey, topicSubscriptions)}
        </td>
      </tr>
    `;
  }).join('');

  body.innerHTML = rows;

  if (expandedPubSubTopicARN) {
    const expandedKey = getTopicDomKey(expandedPubSubTopicARN);
    onTopicSubscriptionProtocolChange(expandedKey);
  }
}

function toggleTopicSubscriptions(encodedTopicArn) {
  const topicArn = decodeURIComponent(encodedTopicArn || '').trim();
  if (!topicArn) return;

  if (expandedPubSubTopicARN === topicArn) {
    expandedPubSubTopicARN = '';
  } else {
    expandedPubSubTopicARN = topicArn;
  }

  if (latestPubSubState) {
    renderPubSubState(latestPubSubState);
  }
}

function renderTopicActivityTable(activities) {
  if (!activities.length) {
    return '<p class="text-sm text-slate-500">No activity found for this topic.</p>';
  }

  const rows = activities.map((entry) => {
    const detailsText = entry.details ? escapeHTML(JSON.stringify(entry.details)) : '-';
    return `
      <tr class="border-b align-top">
        <td class="py-2 pr-2 text-xs whitespace-nowrap">${escapeHTML(new Date(entry.timestamp).toLocaleString())}</td>
        <td class="py-2 pr-2 text-xs">${escapeHTML(entry.event_type || '')}</td>
        <td class="py-2 pr-2 text-xs">${escapeHTML(entry.status || '')}</td>
        <td class="py-2 pr-2 text-xs break-all">${escapeHTML(entry.message_id || '-')}</td>
        <td class="py-2 pr-2 text-xs break-all">${escapeHTML(entry.subscription_arn || '-')}</td>
        <td class="py-2 pr-2 text-xs break-all">${detailsText}</td>
        <td class="py-2 text-xs break-all text-red-700">${escapeHTML(entry.error || '-')}</td>
      </tr>
    `;
  }).join('');

  return `
    <div class="overflow-x-auto">
      <table class="w-full">
        <thead>
          <tr class="text-xs text-slate-500 border-b">
            <th class="text-left py-1 pr-2">Timestamp</th>
            <th class="text-left py-1 pr-2">Event</th>
            <th class="text-left py-1 pr-2">Status</th>
            <th class="text-left py-1 pr-2">Message ID</th>
            <th class="text-left py-1 pr-2">Subscription</th>
            <th class="text-left py-1 pr-2">Details</th>
            <th class="text-left py-1">Error</th>
          </tr>
        </thead>
        <tbody>${rows}</tbody>
      </table>
    </div>
  `;
}

async function openTopicActivityModal(encodedTopicARN, displayName) {
  const topicARN = decodeURIComponent(encodedTopicARN || '').trim();
  if (!topicARN) return;

  activeTopicActivityARN = topicARN;
  activeTopicActivityName = displayName || topicARN;

  const modal = document.getElementById('topic-activity-modal');
  const subtitle = document.getElementById('topic-activity-subtitle');
  const body = document.getElementById('topic-activity-body');
  const status = document.getElementById('topic-activity-status');

  if (subtitle) subtitle.textContent = activeTopicActivityName;
  if (body) body.innerHTML = '<p class="text-sm text-slate-500">Loading activity...</p>';
  if (status) {
    status.className = 'mb-2 text-xs px-2 py-1 rounded bg-slate-100 text-slate-700';
    status.textContent = 'Loading topic activity...';
    status.classList.remove('hidden');
  }
  if (modal) modal.classList.remove('hidden');

  await refreshTopicActivityModal();
}

async function refreshTopicActivityModal() {
  if (!activeTopicActivityARN) return;

  const body = document.getElementById('topic-activity-body');
  const status = document.getElementById('topic-activity-status');

  try {
    const data = await apiGet(`/api/services/ess-enn-ess/topics/${encodeURIComponent(activeTopicActivityARN)}/activities`);
    const activities = data.activities || [];
    if (body) body.innerHTML = renderTopicActivityTable(activities);
    if (status) {
      status.className = 'mb-2 text-xs px-2 py-1 rounded bg-emerald-100 text-emerald-700';
      status.textContent = `Loaded ${activities.length} activity entr${activities.length === 1 ? 'y' : 'ies'}.`;
      status.classList.remove('hidden');
    }
  } catch (error) {
    if (body) {
      body.innerHTML = '<p class="text-sm text-red-600">Unable to load topic activity.</p>';
    }
    if (status) {
      status.className = 'mb-2 text-xs px-2 py-1 rounded bg-red-100 text-red-700';
      status.textContent = error.message;
      status.classList.remove('hidden');
    }
  }
}

function closeTopicActivityModal() {
  const modal = document.getElementById('topic-activity-modal');
  if (modal) modal.classList.add('hidden');
  activeTopicActivityARN = '';
  activeTopicActivityName = '';
}

function syncTopicSelector(selectId, topics) {
  const select = document.getElementById(selectId);
  if (!select) return;

  const existingValue = select.value;
  const options = topics.map((topic) => {
    const arn = topic.topic_arn || '';
    const name = topic.display_name || arn;
    return `<option value="${escapeHTML(arn)}">${escapeHTML(name)}</option>`;
  }).join('');

  select.innerHTML = options || '<option value="">No topics available</option>';

  if (existingValue && topics.some((topic) => topic.topic_arn === existingValue)) {
    select.value = existingValue;
  }
}

async function createTopic() {
  try {
    const input = document.getElementById('topic-create-name');
    const name = input?.value?.trim() || '';
    if (!name) {
      setAlert('Topic name is required');
      return;
    }

    await apiPost('/api/services/ess-enn-ess/actions/create-topic', { name });
    if (input) input.value = '';
    setAlert(`Topic created: ${name}`, 'info');
    await loadPubSubState();
  } catch (error) {
    setAlert(error.message);
  }
}

async function deleteTopic(encodedTopicArn) {
  try {
    const topicArn = decodeURIComponent(encodedTopicArn || '').trim();
    if (!topicArn) return;
    if (!window.confirm(`Delete topic ${topicArn}?`)) return;
    await apiPost('/api/services/ess-enn-ess/actions/delete-topic', { topic_arn: topicArn });
    setAlert('Topic deleted.', 'info');
    if (expandedPubSubTopicARN === topicArn) {
      expandedPubSubTopicARN = '';
    }
    await loadPubSubState();
  } catch (error) {
    setAlert(error.message);
  }
}

async function createSubscriptionForTopic(encodedTopicArn, topicKey) {
  try {
    const topicArn = decodeURIComponent(encodedTopicArn || '').trim();
    const protocolSelect = document.getElementById(`subscription-protocol-${topicKey}`);
    const endpointInput = document.getElementById(`subscription-endpoint-${topicKey}`);
    const queueTargetSelect = document.getElementById(`subscription-queue-target-${topicKey}`);

    const protocol = protocolSelect?.value?.trim() || 'http';
    const endpoint = protocol === 'ess-queue-ess'
      ? (queueTargetSelect?.value?.trim() || '')
      : (endpointInput?.value?.trim() || '');

    if (!topicArn) {
      setAlert('Topic ARN is required for subscription');
      return;
    }
    if (!endpoint) {
      setAlert(protocol === 'ess-queue-ess' ? 'Select an Ess-Queue-Ess queue target' : 'Subscription endpoint is required');
      return;
    }

    await apiPost('/api/services/ess-enn-ess/actions/create-subscription', {
      topic_arn: topicArn,
      protocol,
      endpoint,
      auto_confirm: true,
    });

    if (endpointInput) endpointInput.value = '';
    if (queueTargetSelect && protocol === 'ess-queue-ess') {
      queueTargetSelect.selectedIndex = 0;
    }
    setAlert('Subscription created.', 'info');
    await loadPubSubState();
  } catch (error) {
    setAlert(error.message);
  }
}

async function deleteSubscription(encodedSubscriptionArn) {
  try {
    const subscriptionArn = decodeURIComponent(encodedSubscriptionArn || '').trim();
    if (!subscriptionArn) return;
    if (!window.confirm(`Delete subscription ${subscriptionArn}?`)) return;
    await apiPost('/api/services/ess-enn-ess/actions/delete-subscription', { subscription_arn: subscriptionArn });
    setAlert('Subscription deleted.', 'info');
    await loadPubSubState();
  } catch (error) {
    setAlert(error.message);
  }
}

async function publishTopicMessage() {
  try {
    const topicSelect = document.getElementById('publish-topic-arn');
    const subjectInput = document.getElementById('publish-subject');
    const messageInput = document.getElementById('publish-message');

    const topicArn = topicSelect?.value?.trim() || '';
    const subject = subjectInput?.value?.trim() || '';
    const message = messageInput?.value?.trim() || '';

    if (!topicArn) {
      setAlert('Select a topic to publish');
      return;
    }
    if (!message) {
      setAlert('Message body is required');
      return;
    }

    await apiPost('/api/services/ess-enn-ess/actions/publish', {
      topic_arn: topicArn,
      subject,
      message,
    });

    if (subjectInput) subjectInput.value = '';
    if (messageInput) messageInput.value = '';
    setAlert('Message published.', 'info');
    await loadPubSubState();
  } catch (error) {
    setAlert(error.message);
  }
}

function queueRowTemplate(queue) {
  const isExpanded = expandedQueues.has(queue.queue_id);
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
        <button class="flex-1 text-left flex items-start gap-2" title="Expand or collapse queue details" aria-label="Toggle queue details" onclick="toggleQueue('${queue.queue_id}')">
          <span id="queue-toggle-icon-${queue.queue_id}" data-role="queue-toggle-icon" class="h-7 w-7 shrink-0 rounded bg-slate-700 text-white text-sm inline-flex items-center justify-center">${isExpanded ? '−' : '+'}</span>
          <span>
            <div class="font-medium">${queue.queue_name}</div>
            <div class="text-xs text-slate-500">${queue.queue_url}</div>
          </span>
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
    const toggleIcon = document.getElementById(`queue-toggle-icon-${queue.queue_id}`);
    if (!panel) {
      return;
    }
    if (expandedQueues.has(queue.queue_id)) {
      panel.classList.remove('hidden');
      if (toggleIcon) toggleIcon.textContent = '−';
    } else {
      panel.classList.add('hidden');
      if (toggleIcon) toggleIcon.textContent = '+';
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
  const toggleIcon = document.getElementById(`queue-toggle-icon-${queueId}`);
  if (!panel) return;
  const expanded = !panel.classList.contains('hidden');
  if (expanded) {
    panel.classList.add('hidden');
    expandedQueues.delete(queueId);
    if (toggleIcon) toggleIcon.textContent = '+';
  } else {
    panel.classList.remove('hidden');
    expandedQueues.add(queueId);
    if (toggleIcon) toggleIcon.textContent = '−';
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
      } else if (view === 'ess-queue-ess') {
        renderQueuesIncremental(payload.queues || []);
      } else if (view === 'ess-enn-ess') {
        renderPubSubState(payload);
      } else if (view === 'essthree') {
        renderEssThreeSummary(payload);
      } else {
        renderCloudfauxntSummary(payload);
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
document.getElementById('menu-ess-enn-ess').addEventListener('click', () => switchView('ess-enn-ess'));
document.getElementById('menu-essthree').addEventListener('click', () => switchView('essthree'));
document.getElementById('menu-cloudfauxnt').addEventListener('click', () => switchView('cloudfauxnt'));

switchView('dashboard');
