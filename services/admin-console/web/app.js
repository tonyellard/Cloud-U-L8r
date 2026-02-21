// SPDX-License-Identifier: Apache-2.0
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
let kayVeeActivityRows = [];
let kayVeeActivityNextToken = '';
let kayVeeSummary = null;
let kayVeeParameterRows = [];
let kayVeeParameterNextToken = '';
let kayVeeSecretRows = [];
let kayVeeSecretNextToken = '';
const kayVeeSecretValues = new Map();
let kayVeeLabelParameterTarget = '';
let kayVeeUpdateParameterTarget = null;
let essThreeActivityRows = [];
let essThreeActivityNextToken = '';
let essThreeExpandedBucket = '';
let essThreeObjectCache = new Map();
let cloudfauxntSummary = null;
let cloudfauxntEditingOriginName = '';

const validViews = new Set([
  'dashboard',
  'ess-queue-ess',
  'ess-enn-ess',
  'kay-vee',
  'essthree',
  'cloudfauxnt',
]);

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
    'kay-vee': 'services/kay-vee/README.md',
    'admin-console': 'services/admin-console/README.md',
  };

  const readmePath = readmePaths[serviceName];
  if (!readmePath) return '';
  return `${repoBaseURL}/blob/main/${readmePath}`;
}

function getAdminViewForService(serviceName) {
  const viewMap = {
    'ess-queue-ess': 'ess-queue-ess',
    'ess-enn-ess': 'ess-enn-ess',
    'kay-vee': 'kay-vee',
    'essthree': 'essthree',
    'cloudfauxnt': 'cloudfauxnt',
  };

  return viewMap[serviceName] || '';
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

function normalizeView(view) {
  if (!view || !validViews.has(view)) {
    return 'dashboard';
  }
  return view;
}

function getViewFromLocation() {
  const params = new URLSearchParams(window.location.search);
  return normalizeView(params.get('view'));
}

function relativeURLForView(view) {
  const url = new URL(window.location.href);
  if (view === 'dashboard') {
    url.searchParams.delete('view');
  } else {
    url.searchParams.set('view', view);
  }
  return `${url.pathname}${url.search}${url.hash}`;
}

function writeHistoryState(view, mode = 'push') {
  const state = { view };
  const relativeURL = relativeURLForView(view);
  if (mode === 'replace') {
    window.history.replaceState(state, '', relativeURL);
    return;
  }
  window.history.pushState(state, '', relativeURL);
}

function sortMenuServicesAlphabetically() {
  const nav = document.querySelector('aside nav');
  if (!nav) return;

  const dashboardButton = document.getElementById('menu-dashboard');
  const serviceButtons = Array.from(nav.querySelectorAll('.menu-btn'))
    .filter((button) => button.id !== 'menu-dashboard')
    .sort((a, b) => a.textContent.trim().localeCompare(b.textContent.trim(), undefined, { sensitivity: 'base' }));

  if (dashboardButton) {
    nav.appendChild(dashboardButton);
  }
  serviceButtons.forEach((button) => nav.appendChild(button));
}

function switchView(view, options = {}) {
  const nextView = normalizeView(view);
  const changed = nextView !== activeView;
  const updateHistory = options.updateHistory !== false;
  const historyMode = options.historyMode || 'push';

  activeView = nextView;
  setActiveMenu(nextView);
  setAlert('');

  if (updateHistory && (changed || historyMode === 'replace')) {
    writeHistoryState(nextView, historyMode);
  }

  const title = document.getElementById('view-title');
  const subtitle = document.getElementById('view-subtitle');
  if (nextView === 'dashboard') {
    title.textContent = 'Dashboard';
    subtitle.textContent = 'Live status of active emulator surface';
    loadDashboard();
  } else if (nextView === 'ess-queue-ess') {
    title.textContent = 'ess-queue-ess';
    subtitle.textContent = 'Queue operations and non-mutating message inspection';
    loadQueues();
  } else if (nextView === 'ess-enn-ess') {
    title.textContent = 'ess-enn-ess';
    subtitle.textContent = 'Topic, subscription, and publish operations';
    loadPubSubState();
  } else if (nextView === 'kay-vee') {
    title.textContent = 'kay-vee';
    subtitle.textContent = 'Parameter Store + Secrets Manager emulator admin surface';
    loadKayVeeOverview();
  } else if (nextView === 'essthree') {
    title.textContent = 'ess-three';
    subtitle.textContent = 'S3-compatible object storage emulator admin surface';
    loadEssThreeSummary();
  } else {
    title.textContent = 'cloudfauxnt';
    subtitle.textContent = 'Distribution, origin, behavior, and signing administration';
    loadCloudfauxntSummary();
  }

    connectSSE(nextView);
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
  const sortedServices = [...(data.services || [])].sort((a, b) => (
    displayServiceName(a.name).localeCompare(displayServiceName(b.name), undefined, { sensitivity: 'base' })
  ));

  const serviceRows = sortedServices.map((service) => {
    const serviceReadmeURL = getServiceReadmeURL(service.name);
    const serviceAdminView = getAdminViewForService(service.name);
    const serviceTitle = serviceAdminView
      ? `<button class="font-medium text-blue-700 hover:text-blue-900 hover:underline" title="Open ${displayServiceName(service.name)} admin view" aria-label="Open ${displayServiceName(service.name)} admin view" onclick="switchView('${serviceAdminView}')">${displayServiceName(service.name)}</button>`
      : `<div class="font-medium">${displayServiceName(service.name)}</div>`;
    const badge = service.status === 'online'
      ? '<span class="px-2 py-1 rounded text-xs bg-emerald-100 text-emerald-800">online</span>'
      : '<span class="px-2 py-1 rounded text-xs bg-red-100 text-red-800">offline</span>';

    const stats = (service.stats || []).map((stat) => (
      `<li class="text-sm text-slate-700">${stat.label}: <span class="font-semibold">${stat.value}</span></li>`
    )).join('');

    return `
      <div class="bg-white rounded border p-4 flex items-start gap-4">
        <div class="w-48 shrink-0">
          ${serviceTitle}
          <div class="mt-1">${badge}</div>
        </div>
        <div class="flex-1">
          <ul class="space-y-1">
            ${stats || '<li class="text-sm text-slate-500">No stats available.</li>'}
          </ul>
        </div>
        <div class="shrink-0 ml-auto">
          <div class="flex items-center gap-2">
            ${serviceReadmeURL ? `<a class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm hover:bg-slate-50 inline-flex items-center gap-1" title="Open service README on GitHub" aria-label="Open service README on GitHub" href="${serviceReadmeURL}" target="_blank" rel="noopener noreferrer">README <span aria-hidden="true">↗</span></a>` : ''}
            <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Export service configuration" aria-label="Export service configuration" onclick="exportServiceConfig('${service.name}')">Export Config</button>
          </div>
        </div>
      </div>
    `;
  }).join('');

  document.getElementById('view-content').innerHTML = `
    <div class="bg-white rounded border p-3 flex items-center justify-between">
      <div class="text-sm text-slate-600">Repository documentation</div>
      <a class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm hover:bg-slate-50 inline-flex items-center gap-1" title="Open main repository README on GitHub" aria-label="Open main repository README on GitHub" href="${repoBaseURL}/blob/main/README.md" target="_blank" rel="noopener noreferrer">Main README <span aria-hidden="true">↗</span></a>
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
    essThreeActivityRows = data.activity || [];
    essThreeActivityNextToken = data.activityNextToken || '';
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

async function toggleCloudfauxntSigning(enabled) {
  try {
    await apiPost('/api/services/cloudfauxnt/actions/set-signing', { enabled });
    setAlert(`Cloudfauxnt signing ${enabled ? 'enabled' : 'disabled'}.`, 'info');
    await loadCloudfauxntSummary();
  } catch (error) {
    setAlert(error.message);
    await loadCloudfauxntSummary();
  }
}

function openCloudfauxntSigningSettingsModal() {
  const signing = cloudfauxntSummary?.signing || {};
  const keyPairInput = document.getElementById('cloudfauxnt-signing-key-pair-id');
  const publicKeyPathInput = document.getElementById('cloudfauxnt-signing-public-key-path');
  const clockSkewInput = document.getElementById('cloudfauxnt-signing-clock-skew');
  const defaultURLTTLInput = document.getElementById('cloudfauxnt-signing-url-ttl');
  const defaultCookieTTLInput = document.getElementById('cloudfauxnt-signing-cookie-ttl');
  const allowWildcardInput = document.getElementById('cloudfauxnt-signing-allow-wildcards');

  if (!keyPairInput || !publicKeyPathInput || !clockSkewInput || !defaultURLTTLInput || !defaultCookieTTLInput || !allowWildcardInput) {
    return;
  }

  keyPairInput.value = signing.key_pair_id || '';
  publicKeyPathInput.value = signing.public_key_path || '';
  clockSkewInput.value = Number(signing.token_options?.clock_skew_seconds || 0);
  defaultURLTTLInput.value = Number(signing.token_options?.default_url_ttl_seconds || 0);
  defaultCookieTTLInput.value = Number(signing.token_options?.default_cookie_ttl_seconds || 0);
  allowWildcardInput.checked = signing.token_options?.allow_wildcard_patterns === true;

  const modal = document.getElementById('cloudfauxnt-signing-modal');
  if (modal) {
    modal.classList.remove('hidden');
  }
}

function closeCloudfauxntSigningSettingsModal() {
  const modal = document.getElementById('cloudfauxnt-signing-modal');
  if (modal) {
    modal.classList.add('hidden');
  }
}

function parseCloudfauxntSigningNumber(value, label) {
  const parsed = Number.parseInt(String(value ?? '').trim(), 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    throw new Error(`${label} must be a non-negative integer.`);
  }
  return parsed;
}

async function saveCloudfauxntSigningSettings() {
  try {
    const keyPairID = document.getElementById('cloudfauxnt-signing-key-pair-id')?.value?.trim() || '';
    const publicKeyPath = document.getElementById('cloudfauxnt-signing-public-key-path')?.value?.trim() || '';
    const clockSkewSeconds = parseCloudfauxntSigningNumber(document.getElementById('cloudfauxnt-signing-clock-skew')?.value, 'Clock skew');
    const defaultURLTTLSeconds = parseCloudfauxntSigningNumber(document.getElementById('cloudfauxnt-signing-url-ttl')?.value, 'Default URL TTL');
    const defaultCookieTTLSeconds = parseCloudfauxntSigningNumber(document.getElementById('cloudfauxnt-signing-cookie-ttl')?.value, 'Default cookie TTL');
    const allowWildcardPatterns = document.getElementById('cloudfauxnt-signing-allow-wildcards')?.checked === true;

    const updatedSummary = await apiPost('/api/services/cloudfauxnt/actions/update-signing-config', {
      key_pair_id: keyPairID,
      public_key_path: publicKeyPath,
      token_options: {
        clock_skew_seconds: clockSkewSeconds,
        default_url_ttl_seconds: defaultURLTTLSeconds,
        default_cookie_ttl_seconds: defaultCookieTTLSeconds,
        allow_wildcard_patterns: allowWildcardPatterns,
      },
    });

    cloudfauxntSummary = updatedSummary;
    renderCloudfauxntSummary(updatedSummary);
    closeCloudfauxntSigningSettingsModal();
    setAlert('Cloudfauxnt signing settings updated.', 'info');
  } catch (error) {
    setAlert(error.message);
  }
}

function openCloudfauxntOriginModal(encodedCurrentName = '') {
  const decodedCurrentName = decodeURIComponent(encodedCurrentName || '').trim();
  cloudfauxntEditingOriginName = decodedCurrentName;

  const title = document.getElementById('cloudfauxnt-origin-modal-title');
  const currentNameInput = document.getElementById('cloudfauxnt-origin-current-name');
  const nameInput = document.getElementById('cloudfauxnt-origin-name');
  const urlInput = document.getElementById('cloudfauxnt-origin-url');
  const patternsInput = document.getElementById('cloudfauxnt-origin-path-patterns');
  const stripPrefixInput = document.getElementById('cloudfauxnt-origin-strip-prefix');
  const targetPrefixInput = document.getElementById('cloudfauxnt-origin-target-prefix');
  const defaultRootInput = document.getElementById('cloudfauxnt-origin-default-root');
  const requireSignatureSelect = document.getElementById('cloudfauxnt-origin-require-signature');

  if (!title || !currentNameInput || !nameInput || !urlInput || !patternsInput || !stripPrefixInput || !targetPrefixInput || !defaultRootInput || !requireSignatureSelect) {
    return;
  }

  const existing = (cloudfauxntSummary?.origins || []).find((origin) => (origin.name || '') === decodedCurrentName);

  if (existing) {
    title.textContent = 'Edit Origin / Behavior';
    currentNameInput.value = existing.name || '';
    nameInput.value = existing.name || '';
    urlInput.value = existing.url || '';
    patternsInput.value = (existing.path_patterns || []).join('\n');
    stripPrefixInput.value = existing.strip_prefix || '';
    targetPrefixInput.value = existing.target_prefix || '';
    defaultRootInput.value = existing.default_root_object || '';
    if (existing.require_signature === true) {
      requireSignatureSelect.value = 'true';
    } else if (existing.require_signature === false) {
      requireSignatureSelect.value = 'false';
    } else {
      requireSignatureSelect.value = 'inherit';
    }
  } else {
    title.textContent = 'Add Origin / Behavior';
    currentNameInput.value = '';
    nameInput.value = '';
    urlInput.value = '';
    patternsInput.value = '';
    stripPrefixInput.value = '';
    targetPrefixInput.value = '';
    defaultRootInput.value = '';
    requireSignatureSelect.value = 'inherit';
  }

  const modal = document.getElementById('cloudfauxnt-origin-modal');
  if (modal) modal.classList.remove('hidden');
}

function closeCloudfauxntOriginModal() {
  const modal = document.getElementById('cloudfauxnt-origin-modal');
  if (modal) modal.classList.add('hidden');
  cloudfauxntEditingOriginName = '';
}

async function saveCloudfauxntOrigin() {
  try {
    const currentName = document.getElementById('cloudfauxnt-origin-current-name')?.value?.trim() || '';
    const name = document.getElementById('cloudfauxnt-origin-name')?.value?.trim() || '';
    const originURL = document.getElementById('cloudfauxnt-origin-url')?.value?.trim() || '';
    const pathPatternRaw = document.getElementById('cloudfauxnt-origin-path-patterns')?.value || '';
    const stripPrefix = document.getElementById('cloudfauxnt-origin-strip-prefix')?.value?.trim() || '';
    const targetPrefix = document.getElementById('cloudfauxnt-origin-target-prefix')?.value?.trim() || '';
    const defaultRootObject = document.getElementById('cloudfauxnt-origin-default-root')?.value?.trim() || '';
    const requireSignatureValue = document.getElementById('cloudfauxnt-origin-require-signature')?.value || 'inherit';

    const pathPatterns = pathPatternRaw
      .split(/\r?\n|,/)
      .map((entry) => entry.trim())
      .filter((entry) => entry.length > 0);

    if (!name || !originURL || pathPatterns.length === 0) {
      setAlert('Name, URL, and at least one path pattern are required.');
      return;
    }

    const body = {
      current_name: currentName || undefined,
      name,
      url: originURL,
      path_patterns: pathPatterns,
      strip_prefix: stripPrefix,
      target_prefix: targetPrefix,
      require_signature: requireSignatureValue === 'inherit'
        ? null
        : requireSignatureValue === 'true',
      default_root_object: defaultRootObject || null,
    };

    const actionPath = currentName
      ? '/api/services/cloudfauxnt/actions/update-origin'
      : '/api/services/cloudfauxnt/actions/create-origin';

    const updatedSummary = await apiPost(actionPath, body);
    cloudfauxntSummary = updatedSummary;
    renderCloudfauxntSummary(updatedSummary);
    closeCloudfauxntOriginModal();
    setAlert(`${currentName ? 'Updated' : 'Added'} origin ${name}.`, 'info');
  } catch (error) {
    setAlert(error.message);
  }
}

async function loadKayVeeOverview() {
  try {
    const [summary, activityData, parameterData, secretsData] = await Promise.all([
      apiGet('/api/services/kay-vee/summary'),
      apiGet('/api/services/kay-vee/activity?maxResults=25'),
      apiGet('/api/services/kay-vee/parameters/by-path?path=%2F&recursive=true&withDecryption=false&maxResults=10'),
      apiGet('/api/services/kay-vee/secrets?maxResults=25'),
    ]);
    kayVeeSummary = summary;
    kayVeeActivityRows = activityData.activity || [];
    kayVeeActivityNextToken = activityData.nextToken || '';
    kayVeeParameterRows = parameterData.parameters || [];
    kayVeeParameterNextToken = parameterData.nextToken || '';
    kayVeeSecretRows = secretsData.secrets || [];
    kayVeeSecretNextToken = secretsData.nextToken || '';
    renderKayVeeOverview({
      service: 'kay-vee',
      summary: kayVeeSummary,
      activity: kayVeeActivityRows,
      nextToken: kayVeeActivityNextToken,
      parameters: kayVeeParameterRows,
      parametersNextToken: kayVeeParameterNextToken,
      secrets: kayVeeSecretRows,
      secretsNextToken: kayVeeSecretNextToken,
    });
  } catch (error) {
    setAlert(error.message);
  }
}

async function loadMoreKayVeeActivity() {
  if (!kayVeeActivityNextToken) return;
  try {
    const activityData = await apiGet(`/api/services/kay-vee/activity?maxResults=25&nextToken=${encodeURIComponent(kayVeeActivityNextToken)}`);
    kayVeeActivityRows = kayVeeActivityRows.concat(activityData.activity || []);
    kayVeeActivityNextToken = activityData.nextToken || '';
    renderKayVeeActivityModalContent();
  } catch (error) {
    setAlert(error.message);
  }
}

function openKayVeeActivityModal() {
  const modal = document.getElementById('kayvee-activity-modal');
  if (!modal) return;
  modal.classList.remove('hidden');
  renderKayVeeActivityModalContent();
}

function closeKayVeeActivityModal() {
  const modal = document.getElementById('kayvee-activity-modal');
  if (!modal) return;
  modal.classList.add('hidden');
}

function renderKayVeeActivityModalContent() {
  const body = document.getElementById('kayvee-activity-body');
  if (!body) return;

  const rows = (kayVeeActivityRows || []).map((entry) => `
    <tr class="border-b align-top">
      <td class="py-2 pr-2 text-xs whitespace-nowrap">${escapeHTML(new Date(entry.timestamp).toLocaleString())}</td>
      <td class="py-2 pr-2 text-xs">${escapeHTML(entry.method || '-')}</td>
      <td class="py-2 pr-2 text-xs break-all">${escapeHTML(entry.path || '-')}</td>
      <td class="py-2 pr-2 text-xs break-all">${escapeHTML(entry.target || '-')}</td>
      <td class="py-2 pr-2 text-xs">${Number(entry.statusCode || 0)}</td>
      <td class="py-2 text-xs text-red-700">${escapeHTML(entry.errorType || '-')}</td>
    </tr>
  `).join('');

  body.innerHTML = `
    <div class="overflow-x-auto">
      <table class="w-full">
        <thead>
          <tr class="text-xs text-slate-500 border-b">
            <th class="text-left py-1 pr-2">Timestamp</th>
            <th class="text-left py-1 pr-2">Method</th>
            <th class="text-left py-1 pr-2">Path</th>
            <th class="text-left py-1 pr-2">Target</th>
            <th class="text-left py-1 pr-2">Status</th>
            <th class="text-left py-1">Error Type</th>
          </tr>
        </thead>
        <tbody>
          ${rows || '<tr><td colspan="6" class="py-2 text-sm text-slate-500">No activity entries.</td></tr>'}
        </tbody>
      </table>
    </div>
  `;

  const loadMore = document.getElementById('kayvee-activity-load-more');
  if (loadMore) {
    loadMore.classList.toggle('hidden', !kayVeeActivityNextToken);
  }
}

function exportKayVeeState() {
  const anchor = document.createElement('a');
  anchor.href = '/api/services/kay-vee/export';
  anchor.rel = 'noopener';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

function kayVeeParameterQueryFromForm(nextToken = '') {
  const path = document.getElementById('kv-param-path')?.value?.trim() || '/';
  const recursive = document.getElementById('kv-param-recursive')?.checked !== false;
  const withDecryption = document.getElementById('kv-param-decrypt')?.checked === true;
  const typeFilter = document.getElementById('kv-param-type-filter')?.value?.trim() || '';
  const labelFilter = document.getElementById('kv-param-label-filter')?.value?.trim() || '';

  const query = new URLSearchParams();
  query.set('path', path);
  query.set('recursive', String(recursive));
  query.set('withDecryption', String(withDecryption));
  query.set('maxResults', '10');
  if (nextToken) query.set('nextToken', nextToken);
  if (typeFilter) query.set('type', typeFilter);
  if (labelFilter) query.set('label', labelFilter);
  return query.toString();
}

async function loadKayVeeParameters(reset = true) {
  try {
    const query = kayVeeParameterQueryFromForm(reset ? '' : kayVeeParameterNextToken);
    const response = await apiGet(`/api/services/kay-vee/parameters/by-path?${query}`);
    if (reset) {
      kayVeeParameterRows = response.parameters || [];
    } else {
      kayVeeParameterRows = kayVeeParameterRows.concat(response.parameters || []);
    }
    kayVeeParameterNextToken = response.nextToken || '';

    renderKayVeeOverview({
      service: 'kay-vee',
      summary: kayVeeSummary,
      activity: kayVeeActivityRows,
      nextToken: kayVeeActivityNextToken,
      parameters: kayVeeParameterRows,
      parametersNextToken: kayVeeParameterNextToken,
      secrets: kayVeeSecretRows,
      secretsNextToken: kayVeeSecretNextToken,
    });
  } catch (error) {
    setAlert(error.message);
  }
}

async function loadKayVeeSecrets(reset = true) {
  try {
    const nameFilter = document.getElementById('kv-secret-name-filter')?.value?.trim() || '';
    const query = new URLSearchParams();
    query.set('maxResults', '25');
    if (!reset && kayVeeSecretNextToken) query.set('nextToken', kayVeeSecretNextToken);
    if (nameFilter) query.set('nameFilter', nameFilter);

    const response = await apiGet(`/api/services/kay-vee/secrets?${query.toString()}`);
    if (reset) {
      kayVeeSecretRows = response.secrets || [];
    } else {
      kayVeeSecretRows = kayVeeSecretRows.concat(response.secrets || []);
    }
    kayVeeSecretNextToken = response.nextToken || '';

    renderKayVeeOverview({
      service: 'kay-vee',
      summary: kayVeeSummary,
      activity: kayVeeActivityRows,
      nextToken: kayVeeActivityNextToken,
      parameters: kayVeeParameterRows,
      parametersNextToken: kayVeeParameterNextToken,
      secrets: kayVeeSecretRows,
      secretsNextToken: kayVeeSecretNextToken,
    });
  } catch (error) {
    setAlert(error.message);
  }
}

async function createKayVeeSecret() {
  try {
    const name = document.getElementById('kv-secret-create-name')?.value?.trim() || '';
    const description = document.getElementById('kv-secret-create-description')?.value?.trim() || '';
    const secretString = document.getElementById('kv-secret-create-value')?.value || '';

    if (!name) {
      setAlert('Secret name is required.');
      return;
    }
    if (!secretString) {
      setAlert('Secret value is required.');
      return;
    }

    await apiPost('/api/services/kay-vee/actions/create-secret', {
      name,
      description,
      secret_string: secretString,
    });
    setAlert(`Created secret ${name}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function deleteKayVeeSecret(secretID) {
  try {
    const decodedSecretID = decodeURIComponent(secretID || '').trim();
    if (!decodedSecretID) return;
    if (!window.confirm(`Delete secret ${decodedSecretID}?`)) return;

    await apiPost('/api/services/kay-vee/actions/delete-secret', { secret_id: decodedSecretID });
    setAlert(`Deleted secret ${decodedSecretID}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function restoreKayVeeSecret(secretID) {
  try {
    const decodedSecretID = decodeURIComponent(secretID || '').trim();
    if (!decodedSecretID) return;

    await apiPost('/api/services/kay-vee/actions/restore-secret', { secret_id: decodedSecretID });
    setAlert(`Restored secret ${decodedSecretID}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function revealKayVeeSecretValue(secretID) {
  try {
    const decodedSecretID = decodeURIComponent(secretID || '').trim();
    if (!decodedSecretID) return;

    if (kayVeeSecretValues.has(decodedSecretID)) {
      kayVeeSecretValues.delete(decodedSecretID);
      renderKayVeeOverview({
        service: 'kay-vee',
        summary: kayVeeSummary,
        activity: kayVeeActivityRows,
        nextToken: kayVeeActivityNextToken,
        parameters: kayVeeParameterRows,
        parametersNextToken: kayVeeParameterNextToken,
        secrets: kayVeeSecretRows,
        secretsNextToken: kayVeeSecretNextToken,
      });
      return;
    }

    const response = await apiGet(`/api/services/kay-vee/secrets/value?secretId=${encodeURIComponent(decodedSecretID)}`);
    kayVeeSecretValues.set(decodedSecretID, response.secret_string || response.secret_binary || '(empty)');

    renderKayVeeOverview({
      service: 'kay-vee',
      summary: kayVeeSummary,
      activity: kayVeeActivityRows,
      nextToken: kayVeeActivityNextToken,
      parameters: kayVeeParameterRows,
      parametersNextToken: kayVeeParameterNextToken,
      secrets: kayVeeSecretRows,
      secretsNextToken: kayVeeSecretNextToken,
    });
  } catch (error) {
    setAlert(error.message);
  }
}

async function putKayVeeSecretValue(secretID) {
  try {
    const decodedSecretID = decodeURIComponent(secretID || '').trim();
    if (!decodedSecretID) return;

    const secretString = window.prompt(`New value for ${decodedSecretID}:`, '');
    if (secretString === null) return;
    if (!secretString) {
      setAlert('Secret value is required.');
      return;
    }

    await apiPost('/api/services/kay-vee/actions/put-secret-value', {
      secret_id: decodedSecretID,
      secret_string: secretString,
    });
    kayVeeSecretValues.delete(decodedSecretID);
    setAlert(`Added new secret version for ${decodedSecretID}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function updateKayVeeSecret(secretID) {
  try {
    const decodedSecretID = decodeURIComponent(secretID || '').trim();
    if (!decodedSecretID) return;

    const description = window.prompt(`Description update for ${decodedSecretID} (leave empty to skip):`, '');
    if (description === null) return;
    const secretString = window.prompt(`Optional new value for ${decodedSecretID} (leave empty to keep current value):`, '');
    if (secretString === null) return;

    if (!description && !secretString) {
      setAlert('Provide a description or a new secret value to update.');
      return;
    }

    const body = { secret_id: decodedSecretID };
    if (description) body.description = description;
    if (secretString) body.secret_string = secretString;

    await apiPost('/api/services/kay-vee/actions/update-secret', body);
    kayVeeSecretValues.delete(decodedSecretID);
    setAlert(`Updated secret ${decodedSecretID}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function updateKayVeeSecretStage(secretID) {
  try {
    const decodedSecretID = decodeURIComponent(secretID || '').trim();
    if (!decodedSecretID) return;

    const versionStage = window.prompt('Version stage to move (for example AWSCURRENT):', 'AWSCURRENT');
    if (versionStage === null) return;
    const moveToVersionID = window.prompt('Move to version id:', '');
    if (moveToVersionID === null) return;
    const removeFromVersionID = window.prompt('Optional remove-from version id:', '');
    if (removeFromVersionID === null) return;

    if (!versionStage.trim() || !moveToVersionID.trim()) {
      setAlert('version stage and move-to version id are required.');
      return;
    }

    const body = {
      secret_id: decodedSecretID,
      version_stage: versionStage.trim(),
      move_to_version_id: moveToVersionID.trim(),
    };
    if (removeFromVersionID.trim()) {
      body.remove_from_version_id = removeFromVersionID.trim();
    }

    await apiPost('/api/services/kay-vee/actions/update-secret-version-stage', body);
    kayVeeSecretValues.delete(decodedSecretID);
    setAlert(`Updated version stage ${versionStage.trim()} for ${decodedSecretID}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function putKayVeeParameter() {
  try {
    const rawName = document.getElementById('kv-put-name')?.value?.trim() || '';
    const type = document.getElementById('kv-put-type')?.value?.trim() || 'String';
    const value = document.getElementById('kv-put-value')?.value || '';
    const overwrite = document.getElementById('kv-put-overwrite')?.checked === true;

    const name = rawName && rawName.startsWith('/') ? rawName : (rawName ? `/${rawName}` : '');

    if (!name) {
      setAlert('Parameter name is required.');
      return;
    }
    if (!value) {
      setAlert('Parameter value is required.');
      return;
    }

    await apiPost('/api/services/kay-vee/actions/put-parameter', { name, type, value, overwrite });
    setAlert(`Saved parameter ${name}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function updateKayVeeParameter() {
  try {
    const rawName = document.getElementById('kv-modal-update-name')?.value?.trim() || '';
    const name = rawName && rawName.startsWith('/') ? rawName : (rawName ? `/${rawName}` : '');
    const type = document.getElementById('kv-modal-update-type')?.value?.trim() || 'String';
    const value = document.getElementById('kv-modal-update-value')?.value || '';

    if (!name) {
      setAlert('Parameter name is required.');
      return;
    }
    if (!value) {
      setAlert('Parameter value is required.');
      return;
    }

    await apiPost('/api/services/kay-vee/actions/put-parameter', { name, type, value, overwrite: true });
    setAlert(`Updated parameter ${name}.`, 'info');
    closeKayVeeUpdateParameterModal();
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

function openKayVeeUpdateParameterModal(encodedParameterName) {
  const decodedName = decodeURIComponent(encodedParameterName || '').trim();
  if (!decodedName) return;

  const parameter = (kayVeeParameterRows || []).find((item) => (item.Name || '') === decodedName);
  if (!parameter) return;

  kayVeeUpdateParameterTarget = parameter;

  const nameInput = document.getElementById('kv-modal-update-name');
  const typeSelect = document.getElementById('kv-modal-update-type');
  const valueInput = document.getElementById('kv-modal-update-value');
  if (nameInput) nameInput.value = parameter.Name || '';
  if (typeSelect) typeSelect.value = parameter.Type || 'String';
  if (valueInput) valueInput.value = '';

  const modal = document.getElementById('kv-update-parameter-modal');
  if (modal) modal.classList.remove('hidden');
}

function closeKayVeeUpdateParameterModal() {
  const modal = document.getElementById('kv-update-parameter-modal');
  if (modal) modal.classList.add('hidden');
  kayVeeUpdateParameterTarget = null;
}

async function deleteKayVeeParameter(name) {
  try {
    const decodedName = decodeURIComponent(name || '').trim();
    const normalizedName = decodedName && decodedName.startsWith('/') ? decodedName : (decodedName ? `/${decodedName}` : '');
    if (!normalizedName) return;
    if (!window.confirm(`Delete parameter ${normalizedName}?`)) return;

    await apiPost('/api/services/kay-vee/actions/delete-parameter', { name: normalizedName });
    setAlert(`Deleted parameter ${normalizedName}.`, 'info');
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

async function labelKayVeeParameterVersion() {
  try {
    const rawName = document.getElementById('kv-modal-label-name')?.value?.trim() || kayVeeLabelParameterTarget;
    const name = rawName && rawName.startsWith('/') ? rawName : (rawName ? `/${rawName}` : '');
    const label = document.getElementById('kv-modal-label-value')?.value?.trim() || '';
    const versionRaw = document.getElementById('kv-modal-label-version')?.value?.trim() || '';
    const parameterVersion = Number(versionRaw);

    if (!name || !label || !Number.isInteger(parameterVersion) || parameterVersion <= 0) {
      setAlert('Name, label, and a positive integer version are required.');
      return;
    }

    await apiPost('/api/services/kay-vee/actions/label-parameter-version', { name, label, parameter_version: parameterVersion });
    setAlert(`Applied label ${label} to ${name}:${parameterVersion}.`, 'info');
    closeKayVeeLabelParameterModal();
    await loadKayVeeOverview();
  } catch (error) {
    setAlert(error.message);
  }
}

function openKayVeeLabelParameterModal(parameterName) {
  const decodedName = decodeURIComponent(parameterName || '').trim();
  if (!decodedName) return;

  kayVeeLabelParameterTarget = decodedName;

  const nameInput = document.getElementById('kv-modal-label-name');
  const labelInput = document.getElementById('kv-modal-label-value');
  const versionInput = document.getElementById('kv-modal-label-version');
  if (nameInput) nameInput.value = decodedName;
  if (labelInput) labelInput.value = '';
  if (versionInput) versionInput.value = '';

  const modal = document.getElementById('kv-label-parameter-modal');
  if (modal) modal.classList.remove('hidden');
}

function closeKayVeeLabelParameterModal() {
  const modal = document.getElementById('kv-label-parameter-modal');
  if (modal) modal.classList.add('hidden');
  kayVeeLabelParameterTarget = '';
}

function renderKayVeeOverview(payload) {
  const summary = payload.summary || {};
  const parameters = payload.parameters || kayVeeParameterRows || [];
  const secrets = payload.secrets || kayVeeSecretRows || [];

  const parameterRows = parameters.map((parameter) => `
    <tr class="border-b align-top">
      <td class="py-2 pr-2 text-xs break-all">${escapeHTML(parameter.Name || '')}</td>
      <td class="py-2 pr-2 text-xs">${escapeHTML(parameter.Type || '')}</td>
      <td class="py-2 pr-2 text-xs break-all">${escapeHTML(parameter.Value || '-')}</td>
      <td class="py-2 pr-2 text-xs">${Number(parameter.Version || 0)}</td>
      <td class="py-2 pr-2 text-xs whitespace-nowrap">${parameter.LastModifiedDate ? escapeHTML(new Date(parameter.LastModifiedDate).toLocaleString()) : '-'}</td>
      <td class="py-2 text-right">
        <div class="flex justify-end gap-2">
          <button class="px-2 py-1 rounded bg-slate-700 text-white text-xs" title="Update parameter" aria-label="Update parameter" onclick="openKayVeeUpdateParameterModal('${encodeURIComponent(parameter.Name || '')}')">Update</button>
          <button class="px-2 py-1 rounded bg-indigo-700 text-white text-xs" title="Label parameter version" aria-label="Label parameter version" onclick="openKayVeeLabelParameterModal('${encodeURIComponent(parameter.Name || '')}')">Label</button>
          <button class="h-8 w-8 rounded bg-red-600 text-white leading-none inline-flex items-center justify-center" title="Delete parameter" aria-label="Delete parameter" onclick="deleteKayVeeParameter('${encodeURIComponent(parameter.Name || '')}')">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18M8 6V4h8v2m-9 0l1 14h6l1-14" />
            </svg>
          </button>
        </div>
      </td>
    </tr>
  `).join('');

  const secretRows = secrets.map((secret) => {
    const secretKey = secret.Name || secret.ARN || '';
    const isRevealed = kayVeeSecretValues.has(secretKey);
    const revealedValue = isRevealed ? (kayVeeSecretValues.get(secretKey) || '(empty)') : '••••••••';
    const isDeleted = !!secret.DeletedDate;
    return `
      <tr class="border-b align-top">
        <td class="py-2 pr-2 text-xs break-all">${escapeHTML(secret.Name || '')}</td>
        <td class="py-2 pr-2 text-xs break-all">${escapeHTML(secret.Description || '-')}</td>
        <td class="py-2 pr-2 text-xs break-all">${escapeHTML(revealedValue)}</td>
        <td class="py-2 pr-2 text-xs">${isDeleted ? '<span class="px-2 py-1 rounded bg-amber-100 text-amber-800">deleted</span>' : '<span class="px-2 py-1 rounded bg-emerald-100 text-emerald-800">active</span>'}</td>
        <td class="py-2 pr-2 text-xs whitespace-nowrap">${secret.LastChangedDate ? escapeHTML(new Date(secret.LastChangedDate).toLocaleString()) : '-'}</td>
        <td class="py-2 text-right">
          <div class="flex justify-end gap-2">
            <button class="px-2 py-1 rounded bg-indigo-700 text-white text-xs" title="${isRevealed ? 'Hide secret value' : 'Reveal secret value'}" aria-label="${isRevealed ? 'Hide secret value' : 'Reveal secret value'}" onclick="revealKayVeeSecretValue('${encodeURIComponent(secretKey)}')">${isRevealed ? 'Hide' : 'Reveal'}</button>
            ${!isDeleted ? `<button class="px-2 py-1 rounded bg-slate-700 text-white text-xs" title="Update secret" aria-label="Update secret" onclick="updateKayVeeSecret('${encodeURIComponent(secretKey)}')">Update</button>` : ''}
            ${!isDeleted ? `<button class="px-2 py-1 rounded bg-slate-700 text-white text-xs" title="Update secret stage" aria-label="Update secret stage" onclick="updateKayVeeSecretStage('${encodeURIComponent(secretKey)}')">Stage</button>` : ''}
            ${isDeleted
              ? `<button class="px-2 py-1 rounded bg-emerald-700 text-white text-xs" title="Restore secret" aria-label="Restore secret" onclick="restoreKayVeeSecret('${encodeURIComponent(secretKey)}')">Restore</button>`
              : `<button class="h-8 w-8 rounded bg-red-600 text-white leading-none inline-flex items-center justify-center" title="Delete secret" aria-label="Delete secret" onclick="deleteKayVeeSecret('${encodeURIComponent(secretKey)}')"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="h-4 w-4" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18M8 6V4h8v2m-9 0l1 14h6l1-14" /></svg></button>`}
          </div>
        </td>
      </tr>
    `;
  }).join('');

  document.getElementById('view-content').innerHTML = `
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Parameters</div>
        <div class="text-2xl font-semibold">${Number(summary.parameters || 0)}</div>
      </div>
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Secrets (Total)</div>
        <div class="text-2xl font-semibold">${Number(summary.secretsTotal || 0)}</div>
      </div>
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Secrets (Active)</div>
        <div class="text-2xl font-semibold">${Number(summary.secretsActive || 0)}</div>
      </div>
      <div class="bg-white rounded border p-4">
        <div class="text-sm text-slate-500">Secrets (Deleted)</div>
        <div class="text-2xl font-semibold">${Number(summary.secretsDeleted || 0)}</div>
      </div>
    </div>

    <div class="bg-white rounded border p-4">
      <div class="flex items-center justify-end gap-2">
        <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="View activity log" aria-label="View activity log" onclick="openKayVeeActivityModal()">Activity Log</button>
        <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Refresh kay-vee view" aria-label="Refresh kay-vee view" onclick="loadKayVeeOverview()">Refresh</button>
      </div>
    </div>

    <div class="bg-slate-900 text-white rounded border border-slate-900 px-4 py-2 text-sm font-semibold tracking-wide uppercase">Parameters</div>

    <div class="grid lg:grid-cols-2 gap-4">
      <div class="bg-white rounded border p-4">
        <h3 class="font-semibold mb-2">Put Parameter</h3>
        <div class="grid gap-2">
          <input id="kv-put-name" class="border rounded px-2 py-1 text-sm" placeholder="/app/dev/key" />
          <select id="kv-put-type" class="border rounded px-2 py-1 text-sm">
            <option value="String">String</option>
            <option value="SecureString">SecureString</option>
            <option value="StringList">StringList</option>
          </select>
          <textarea id="kv-put-value" class="border rounded px-2 py-1 text-sm" rows="2" placeholder="Value"></textarea>
          <label class="text-sm flex items-center gap-2"><input id="kv-put-overwrite" type="checkbox" /> Overwrite existing</label>
          <div class="flex justify-end">
            <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Save parameter" aria-label="Save parameter" onclick="putKayVeeParameter()">Save</button>
          </div>
        </div>
      </div>
    </div>

    <div class="bg-white rounded border p-4">
      <div class="flex items-center justify-between mb-2">
        <h3 class="font-semibold">Parameters</h3>
        <div class="flex items-center gap-2">
          <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Refresh parameters" aria-label="Refresh parameters" onclick="loadKayVeeParameters(true)">Refresh</button>
          ${(payload.parametersNextToken || kayVeeParameterNextToken) ? '<button class="px-3 py-1 rounded bg-indigo-700 text-white text-sm" title="Load more parameters" aria-label="Load more parameters" onclick="loadKayVeeParameters(false)">Load More</button>' : ''}
        </div>
      </div>
      <div class="grid md:grid-cols-5 gap-2 mb-3">
        <input id="kv-param-path" class="border rounded px-2 py-1 text-sm" placeholder="Path" value="/" />
        <input id="kv-param-type-filter" class="border rounded px-2 py-1 text-sm" placeholder="Type filter (optional)" />
        <input id="kv-param-label-filter" class="border rounded px-2 py-1 text-sm" placeholder="Label filter (optional)" />
        <label class="text-sm flex items-center gap-2"><input id="kv-param-recursive" type="checkbox" checked /> Recursive</label>
        <label class="text-sm flex items-center gap-2"><input id="kv-param-decrypt" type="checkbox" /> Decrypt</label>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="text-xs text-slate-500 border-b">
              <th class="text-left py-1 pr-2">Name</th>
              <th class="text-left py-1 pr-2">Type</th>
              <th class="text-left py-1 pr-2">Value</th>
              <th class="text-left py-1 pr-2">Version</th>
              <th class="text-left py-1 pr-2">Last Modified</th>
              <th class="text-right py-1">Actions</th>
            </tr>
          </thead>
          <tbody>
            ${parameterRows || '<tr><td colspan="6" class="py-2 text-sm text-slate-500">No parameters found.</td></tr>'}
          </tbody>
        </table>
      </div>
    </div>

    <div class="bg-slate-900 text-white rounded border border-slate-900 px-4 py-2 text-sm font-semibold tracking-wide uppercase">Secrets</div>

    <div class="bg-white rounded border p-4">
      <h3 class="font-semibold mb-2">Create Secret</h3>
      <div class="grid gap-2">
        <input id="kv-secret-create-name" class="border rounded px-2 py-1 text-sm" placeholder="app/dev/secret" />
        <input id="kv-secret-create-description" class="border rounded px-2 py-1 text-sm" placeholder="Description (optional)" />
        <textarea id="kv-secret-create-value" class="border rounded px-2 py-1 text-sm" rows="2" placeholder="Secret value"></textarea>
        <div class="flex justify-end">
          <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Create secret" aria-label="Create secret" onclick="createKayVeeSecret()">Create Secret</button>
        </div>
      </div>
    </div>

    <div class="bg-white rounded border p-4">
      <h3 class="font-semibold mb-2">Secrets</h3>
      <div class="grid md:grid-cols-3 gap-2 mb-3">
        <input id="kv-secret-name-filter" class="border rounded px-2 py-1 text-sm" placeholder="Name filter (optional)" />
        <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Refresh secrets" aria-label="Refresh secrets" onclick="loadKayVeeSecrets(true)">Refresh</button>
        ${(payload.secretsNextToken || kayVeeSecretNextToken) ? '<button class="px-3 py-1 rounded bg-indigo-700 text-white text-sm" title="Load more secrets" aria-label="Load more secrets" onclick="loadKayVeeSecrets(false)">Load More</button>' : ''}
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="text-xs text-slate-500 border-b">
              <th class="text-left py-1 pr-2">Name</th>
              <th class="text-left py-1 pr-2">Description</th>
              <th class="text-left py-1 pr-2">Current Value</th>
              <th class="text-left py-1 pr-2">State</th>
              <th class="text-left py-1 pr-2">Last Changed</th>
              <th class="text-right py-1">Actions</th>
            </tr>
          </thead>
          <tbody>
            ${secretRows || '<tr><td colspan="6" class="py-2 text-sm text-slate-500">No secrets found.</td></tr>'}
          </tbody>
        </table>
      </div>
    </div>

    <div id="kayvee-activity-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50" onclick="if (event.target === this) closeKayVeeActivityModal()">
      <div class="bg-white w-full max-w-5xl rounded border shadow-lg">
        <div class="px-4 py-3 border-b flex items-center justify-between">
          <h3 class="font-semibold">Kay-Vee Activity Log</h3>
          <div class="flex items-center gap-2">
            <button id="kayvee-activity-load-more" class="hidden px-3 py-1 rounded bg-indigo-700 text-white text-sm" title="Load more activity" aria-label="Load more activity" onclick="loadMoreKayVeeActivity()">Load More</button>
            <button class="h-7 w-7 rounded bg-slate-200 text-slate-700 text-sm" title="Close activity log" aria-label="Close activity log" onclick="closeKayVeeActivityModal()">✕</button>
          </div>
        </div>
        <div class="p-4 max-h-[70vh] overflow-auto" id="kayvee-activity-body">
          <p class="text-sm text-slate-500">No activity entries.</p>
        </div>
      </div>
    </div>

    <div id="kv-label-parameter-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50" onclick="if (event.target === this) closeKayVeeLabelParameterModal()">
      <div class="bg-white w-full max-w-md rounded border shadow-lg">
        <div class="px-4 py-3 border-b flex items-center justify-between">
          <h3 class="font-semibold">Label Parameter Version</h3>
          <button class="h-7 w-7 rounded bg-slate-200 text-slate-700 text-sm" title="Close label modal" aria-label="Close label modal" onclick="closeKayVeeLabelParameterModal()">✕</button>
        </div>
        <div class="p-4 grid gap-2">
          <input id="kv-modal-label-name" class="border rounded px-2 py-1 text-sm" placeholder="/app/dev/key" readonly />
          <input id="kv-modal-label-value" class="border rounded px-2 py-1 text-sm" placeholder="stable" />
          <input id="kv-modal-label-version" type="number" min="1" class="border rounded px-2 py-1 text-sm" placeholder="1" />
          <div class="flex justify-end gap-2 pt-1">
            <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Cancel label" aria-label="Cancel label" onclick="closeKayVeeLabelParameterModal()">Cancel</button>
            <button class="px-3 py-1 rounded bg-indigo-700 text-white text-sm" title="Apply label" aria-label="Apply label" onclick="labelKayVeeParameterVersion()">Apply Label</button>
          </div>
        </div>
      </div>
    </div>

    <div id="kv-update-parameter-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50" onclick="if (event.target === this) closeKayVeeUpdateParameterModal()">
      <div class="bg-white w-full max-w-md rounded border shadow-lg">
        <div class="px-4 py-3 border-b flex items-center justify-between">
          <h3 class="font-semibold">Update Parameter</h3>
          <button class="h-7 w-7 rounded bg-slate-200 text-slate-700 text-sm" title="Close update modal" aria-label="Close update modal" onclick="closeKayVeeUpdateParameterModal()">✕</button>
        </div>
        <div class="p-4 grid gap-2">
          <input id="kv-modal-update-name" class="border rounded px-2 py-1 text-sm" placeholder="/app/dev/key" readonly />
          <select id="kv-modal-update-type" class="border rounded px-2 py-1 text-sm">
            <option value="String">String</option>
            <option value="SecureString">SecureString</option>
            <option value="StringList">StringList</option>
          </select>
          <textarea id="kv-modal-update-value" class="border rounded px-2 py-1 text-sm" rows="3" placeholder="New value"></textarea>
          <div class="flex justify-end gap-2 pt-1">
            <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Cancel update" aria-label="Cancel update" onclick="closeKayVeeUpdateParameterModal()">Cancel</button>
            <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Update parameter" aria-label="Update parameter" onclick="updateKayVeeParameter()">Update</button>
          </div>
        </div>
      </div>
    </div>
  `;
}

function renderFutureBanner(text) {
  return `<div class="border border-amber-200 bg-amber-50 text-amber-800 rounded px-3 py-2 text-sm">${text}</div>`;
}

function renderEssThreeSummary(data) {
  const buckets = data.buckets || [];
  const rows = buckets.map((bucket) => {
    const isExpanded = essThreeExpandedBucket === bucket.name;
    const cached = essThreeObjectCache.get(bucket.name);
    let objectRows = '';
    let objectSection = '';

    if (isExpanded) {
      if (cached && cached.objects) {
        objectRows = cached.objects.map((obj) => `
          <tr class="border-b">
            <td class="py-1 pr-2 text-xs break-all">${escapeHTML(obj.key || '')}</td>
            <td class="py-1 pr-2 text-xs text-right">${formatObjectSize(obj.size)}</td>
            <td class="py-1 pr-2 text-xs">${escapeHTML(obj.contentType || '-')}</td>
            <td class="py-1 pr-2 text-xs whitespace-nowrap">${escapeHTML(obj.lastModified ? new Date(obj.lastModified).toLocaleString() : '-')}</td>
            <td class="py-1 text-right">
              <button class="px-2 py-0.5 rounded bg-red-600 text-white text-xs" title="Delete object" aria-label="Delete object ${escapeHTML(obj.key || '')}" onclick="deleteEssThreeObject('${escapeHTML(bucket.name)}', '${escapeAttr(obj.key)}')">Delete</button>
            </td>
          </tr>
        `).join('');

        const loadMoreBtn = cached.isTruncated
          ? `<button class="px-3 py-1 rounded bg-indigo-700 text-white text-xs" title="Load more objects" aria-label="Load more objects" onclick="loadMoreEssThreeObjects('${escapeHTML(bucket.name)}')">Load More</button>`
          : '';

        objectSection = `
          <tr>
            <td colspan="3" class="p-0">
              <div class="bg-slate-50 border-t p-3">
                <div class="overflow-x-auto">
                  <table class="w-full">
                    <thead>
                      <tr class="text-xs text-slate-500 border-b">
                        <th class="text-left py-1 pr-2">Key</th>
                        <th class="text-right py-1 pr-2">Size</th>
                        <th class="text-left py-1 pr-2">Content Type</th>
                        <th class="text-left py-1 pr-2">Last Modified</th>
                        <th class="text-right py-1">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      ${objectRows || '<tr><td colspan="5" class="py-2 text-xs text-slate-500">No objects in this bucket.</td></tr>'}
                    </tbody>
                  </table>
                </div>
                <div class="flex justify-end mt-2 gap-2">
                  ${loadMoreBtn}
                </div>
              </div>
            </td>
          </tr>
        `;
      } else {
        objectSection = `
          <tr>
            <td colspan="3" class="p-0">
              <div class="bg-slate-50 border-t p-3 text-sm text-slate-500">Loading objects...</div>
            </td>
          </tr>
        `;
      }
    }

    return `
      <tr class="border-b">
        <td class="py-2 pr-2 text-sm">
          <button class="text-left font-medium hover:underline" title="Browse bucket contents" aria-label="Browse bucket ${escapeHTML(bucket.name)}" onclick="toggleEssThreeBucket('${escapeHTML(bucket.name)}')">${isExpanded ? '▾' : '▸'} ${escapeHTML(bucket.name || '')}</button>
        </td>
        <td class="py-2 pr-2 text-sm text-right">${Number(bucket.object_count || 0)}</td>
        <td class="py-2 text-right whitespace-nowrap">
          <button class="px-2 py-0.5 rounded bg-amber-600 text-white text-xs" title="Delete all objects in bucket" aria-label="Purge bucket ${escapeHTML(bucket.name || '')}" onclick="purgeEssThreeBucket('${escapeHTML(bucket.name)}')">Purge</button>
          <button class="px-2 py-0.5 rounded bg-red-600 text-white text-xs" title="Delete bucket" aria-label="Delete bucket ${escapeHTML(bucket.name || '')}" onclick="deleteEssThreeBucket('${escapeHTML(bucket.name)}')">Delete</button>
        </td>
      </tr>
      ${objectSection}
    `;
  }).join('');

  document.getElementById('view-content').innerHTML = `
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
      <div class="flex items-center justify-end gap-2 mb-3">
        <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="View activity log" aria-label="View activity log" onclick="openEssThreeActivityModal()">Activity Log</button>
        <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Refresh view" aria-label="Refresh ess-three view" onclick="loadEssThreeSummary()">Refresh</button>
      </div>
    </div>

    <div class="bg-slate-900 text-white rounded border border-slate-900 px-4 py-2 text-sm font-semibold tracking-wide uppercase">Buckets</div>

    <div class="grid lg:grid-cols-2 gap-4">
      <div class="bg-white rounded border p-4">
        <h3 class="font-semibold mb-2">Create Bucket</h3>
        <div class="grid gap-2">
          <input id="s3-create-bucket-name" class="border rounded px-2 py-1 text-sm" placeholder="my-bucket-name" />
          <div class="flex justify-end">
            <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Create bucket" aria-label="Create bucket" onclick="createEssThreeBucket()">Create Bucket</button>
          </div>
        </div>
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
              <th class="text-right py-1 pr-2">Object Count</th>
              <th class="text-right py-1">Actions</th>
            </tr>
          </thead>
          <tbody>
            ${rows || '<tr><td colspan="3" class="py-2 text-sm text-slate-500">No buckets found.</td></tr>'}
          </tbody>
        </table>
      </div>
    </div>

    <div id="essthree-activity-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50" onclick="if (event.target === this) closeEssThreeActivityModal()">
      <div class="bg-white w-full max-w-5xl rounded border shadow-lg">
        <div class="px-4 py-3 border-b flex items-center justify-between">
          <h3 class="font-semibold">ess-three Activity Log</h3>
          <div class="flex items-center gap-2">
            <button id="essthree-activity-load-more" class="hidden px-3 py-1 rounded bg-indigo-700 text-white text-sm" title="Load more activity" aria-label="Load more activity" onclick="loadMoreEssThreeActivity()">Load More</button>
            <button class="h-7 w-7 rounded bg-slate-200 text-slate-700 text-sm" title="Close activity log" aria-label="Close activity log" onclick="closeEssThreeActivityModal()">✕</button>
          </div>
        </div>
        <div class="p-4 max-h-[70vh] overflow-auto" id="essthree-activity-body">
          <p class="text-sm text-slate-500">No activity entries.</p>
        </div>
      </div>
    </div>
  `;
}

function formatObjectSize(bytes) {
  if (bytes == null) return '-';
  const b = Number(bytes);
  if (b === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(b) / Math.log(1024)), units.length - 1);
  const val = b / Math.pow(1024, i);
  return `${i === 0 ? val : val.toFixed(1)} ${units[i]}`;
}

function escapeAttr(str) {
  return (str || '').replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

async function toggleEssThreeBucket(bucketName) {
  if (essThreeExpandedBucket === bucketName) {
    essThreeExpandedBucket = '';
    loadEssThreeSummary();
    return;
  }

  essThreeExpandedBucket = bucketName;
  essThreeObjectCache.delete(bucketName);
  loadEssThreeSummary();

  try {
    const data = await apiGet(`/api/services/essthree/buckets/${encodeURIComponent(bucketName)}/objects?maxKeys=50`);
    essThreeObjectCache.set(bucketName, data);
    if (essThreeExpandedBucket === bucketName) {
      loadEssThreeSummary();
    }
  } catch (error) {
    setAlert(`Failed to list objects: ${error.message}`);
  }
}

async function loadMoreEssThreeObjects(bucketName) {
  const cached = essThreeObjectCache.get(bucketName);
  if (!cached || !cached.nextContinuationToken) return;

  try {
    const data = await apiGet(`/api/services/essthree/buckets/${encodeURIComponent(bucketName)}/objects?maxKeys=50&continuationToken=${encodeURIComponent(cached.nextContinuationToken)}`);
    cached.objects = (cached.objects || []).concat(data.objects || []);
    cached.isTruncated = data.isTruncated;
    cached.nextContinuationToken = data.nextContinuationToken;
    essThreeObjectCache.set(bucketName, cached);
    loadEssThreeSummary();
  } catch (error) {
    setAlert(`Failed to load more objects: ${error.message}`);
  }
}

async function createEssThreeBucket() {
  const nameInput = document.getElementById('s3-create-bucket-name');
  const name = (nameInput?.value || '').trim();
  if (!name) {
    setAlert('Bucket name is required');
    return;
  }

  try {
    await apiPost('/api/services/essthree/actions/create-bucket', { name });
    nameInput.value = '';
    loadEssThreeSummary();
  } catch (error) {
    setAlert(`Failed to create bucket: ${error.message}`);
  }
}

async function deleteEssThreeBucket(bucketName) {
  if (!window.confirm(`Delete bucket "${bucketName}"? The bucket must be empty.`)) return;

  try {
    await apiPost('/api/services/essthree/actions/delete-bucket', { name: bucketName });
    if (essThreeExpandedBucket === bucketName) {
      essThreeExpandedBucket = '';
      essThreeObjectCache.delete(bucketName);
    }
    loadEssThreeSummary();
  } catch (error) {
    setAlert(`Failed to delete bucket: ${error.message}`);
  }
}

async function purgeEssThreeBucket(bucketName) {
  if (!window.confirm(`Purge ALL objects from bucket "${bucketName}"? This cannot be undone.`)) return;

  try {
    await apiPost('/api/services/essthree/actions/purge-bucket', { name: bucketName });
    essThreeObjectCache.delete(bucketName);
    loadEssThreeSummary();
  } catch (error) {
    setAlert(`Failed to purge bucket: ${error.message}`);
  }
}

async function deleteEssThreeObject(bucketName, key) {
  if (!window.confirm(`Delete object "${key}" from bucket "${bucketName}"?`)) return;

  try {
    await apiPost('/api/services/essthree/actions/delete-object', { bucket: bucketName, key });
    const cached = essThreeObjectCache.get(bucketName);
    if (cached && cached.objects) {
      cached.objects = cached.objects.filter((o) => o.key !== key);
      essThreeObjectCache.set(bucketName, cached);
    }
    loadEssThreeSummary();
  } catch (error) {
    setAlert(`Failed to delete object: ${error.message}`);
  }
}

function openEssThreeActivityModal() {
  const modal = document.getElementById('essthree-activity-modal');
  if (!modal) return;
  modal.classList.remove('hidden');
  renderEssThreeActivityModalContent();
}

function closeEssThreeActivityModal() {
  const modal = document.getElementById('essthree-activity-modal');
  if (!modal) return;
  modal.classList.add('hidden');
}

function renderEssThreeActivityModalContent() {
  const body = document.getElementById('essthree-activity-body');
  if (!body) return;

  const rows = (essThreeActivityRows || []).map((entry) => `
    <tr class="border-b align-top">
      <td class="py-2 pr-2 text-xs whitespace-nowrap">${escapeHTML(new Date(entry.timestamp).toLocaleString())}</td>
      <td class="py-2 pr-2 text-xs">${escapeHTML(entry.method || '-')}</td>
      <td class="py-2 pr-2 text-xs break-all">${escapeHTML(entry.path || '-')}</td>
      <td class="py-2 pr-2 text-xs">${escapeHTML(entry.action || '-')}</td>
      <td class="py-2 pr-2 text-xs">${Number(entry.statusCode || 0)}</td>
      <td class="py-2 pr-2 text-xs text-red-700">${escapeHTML(entry.errorType || '-')}</td>
      <td class="py-2 text-xs text-slate-500">${escapeHTML(entry.detail || '-')}</td>
    </tr>
  `).join('');

  body.innerHTML = `
    <div class="overflow-x-auto">
      <table class="w-full">
        <thead>
          <tr class="text-xs text-slate-500 border-b">
            <th class="text-left py-1 pr-2">Timestamp</th>
            <th class="text-left py-1 pr-2">Method</th>
            <th class="text-left py-1 pr-2">Path</th>
            <th class="text-left py-1 pr-2">Action</th>
            <th class="text-left py-1 pr-2">Status</th>
            <th class="text-left py-1 pr-2">Error Type</th>
            <th class="text-left py-1">Detail</th>
          </tr>
        </thead>
        <tbody>
          ${rows || '<tr><td colspan="7" class="py-2 text-sm text-slate-500">No activity entries.</td></tr>'}
        </tbody>
      </table>
    </div>
  `;

  const loadMore = document.getElementById('essthree-activity-load-more');
  if (loadMore) {
    loadMore.classList.toggle('hidden', !essThreeActivityNextToken);
  }
}

async function loadMoreEssThreeActivity() {
  if (!essThreeActivityNextToken) return;
  try {
    const activityData = await apiGet(`/api/services/essthree/activity?maxResults=25&nextToken=${encodeURIComponent(essThreeActivityNextToken)}`);
    essThreeActivityRows = essThreeActivityRows.concat(activityData.activity || []);
    essThreeActivityNextToken = activityData.nextToken || '';
    renderEssThreeActivityModalContent();
  } catch (error) {
    setAlert(`Failed to load activity: ${error.message}`);
  }
}

function getCloudfauxntDistributionURL(data) {
  const serverHostRaw = (data?.server?.host || '').trim();
  const normalizedHost = (serverHostRaw === '' || serverHostRaw === '0.0.0.0' || serverHostRaw === '::' || serverHostRaw === '[::]')
    ? window.location.hostname
    : serverHostRaw;
  const serverPort = Number(data?.server?.port || 0);
  if (!normalizedHost || !Number.isFinite(serverPort) || serverPort <= 0) {
    return '';
  }

  try {
    const distributionURL = new URL(`${window.location.protocol}//${normalizedHost}`);
    distributionURL.port = String(serverPort);
    return distributionURL.toString();
  } catch {
    return '';
  }
}

function renderCloudfauxntSummary(data) {
  cloudfauxntSummary = data;
  const origins = data.origins || [];
  const distributionURL = getCloudfauxntDistributionURL(data);
  const signingEnabled = data.signing?.enabled === true;
  const signingOptions = data.signing?.token_options || {};
  const rows = origins.map((origin) => `
    <tr class="border-b align-top">
      <td class="py-2 pr-2 text-sm">${escapeHTML(origin.name || '')}</td>
      <td class="py-2 pr-2 text-sm break-all">${escapeHTML(origin.url || '')}</td>
      <td class="py-2 pr-2 text-sm">${(origin.path_patterns || []).map((pattern) => `<div>${escapeHTML(pattern)}</div>`).join('')}</td>
      <td class="py-2 pr-2 text-sm">${origin.require_signature ? 'required' : 'not required'}</td>
      <td class="py-2 pr-2 text-sm">${escapeHTML(origin.default_root_object || data.server?.default_root_object || '-')}</td>
      <td class="py-2 text-right">
        <button class="px-2 py-1 rounded bg-slate-700 text-white text-xs" title="Edit origin and behavior" aria-label="Edit origin and behavior" onclick="openCloudfauxntOriginModal('${encodeURIComponent(origin.name || '')}')">Edit</button>
      </td>
    </tr>
  `).join('');

  document.getElementById('view-content').innerHTML = `
    <div class="bg-white rounded border p-4 flex items-center justify-between">
      <div>
        <div class="text-sm text-slate-500">Distribution URL</div>
        <div class="text-sm font-medium break-all">${escapeHTML(distributionURL || 'Unavailable')}</div>
      </div>
      ${distributionURL
        ? `<a class="px-3 py-1 rounded bg-slate-900 text-white text-sm inline-flex items-center gap-1" title="Open cloudfauxnt distribution" aria-label="Open cloudfauxnt distribution" href="${distributionURL}" target="_blank" rel="noopener noreferrer">Open Distribution <span aria-hidden="true">↗</span></a>`
        : ''}
    </div>
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
        <div class="mt-2 flex items-center justify-between">
          <span class="text-sm font-semibold text-slate-700">${signingEnabled ? 'On' : 'Off'}</span>
          <label class="inline-flex items-center cursor-pointer" title="Toggle cloudfauxnt signing" aria-label="Toggle cloudfauxnt signing">
            <input type="checkbox" value="" class="sr-only peer" ${signingEnabled ? 'checked' : ''} onchange="toggleCloudfauxntSigning(this.checked)">
            <div class="relative w-11 h-6 bg-gray-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
          </label>
        </div>
        <div class="mt-2 text-xs text-slate-500">Key Pair: ${escapeHTML(data.signing?.key_pair_id || '-')}</div>
        <div class="mt-1 text-xs text-slate-500">Clock Skew: ${Number(signingOptions.clock_skew_seconds || 0)}s</div>
        <div class="mt-1 text-xs text-slate-500">URL TTL: ${Number(signingOptions.default_url_ttl_seconds || 0)}s · Cookie TTL: ${Number(signingOptions.default_cookie_ttl_seconds || 0)}s</div>
        <div class="mt-3">
          <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Edit signing settings" aria-label="Edit signing settings" onclick="openCloudfauxntSigningSettingsModal()">Edit Settings</button>
        </div>
      </div>
    </div>
    <div class="bg-white rounded border p-4">
      <div class="flex items-center justify-between mb-2">
        <h3 class="font-semibold">Origin & Behavior Overview</h3>
        <div class="flex items-center gap-2">
          <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Add origin and behavior" aria-label="Add origin and behavior" onclick="openCloudfauxntOriginModal()">Add Origin</button>
          <button class="h-7 w-7 rounded bg-slate-700 text-white text-sm" title="Refresh cloudfauxnt overview" aria-label="Refresh cloudfauxnt overview" onclick="loadCloudfauxntSummary()">↻</button>
        </div>
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
              <th class="text-left py-1 pr-2">Default Root</th>
              <th class="text-right py-1">Actions</th>
            </tr>
          </thead>
          <tbody>
            ${rows || '<tr><td colspan="6" class="py-2 text-sm text-slate-500">No origins configured.</td></tr>'}
          </tbody>
        </table>
      </div>
    </div>

    <div id="cloudfauxnt-origin-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50" onclick="if (event.target === this) closeCloudfauxntOriginModal()">
      <div class="bg-white w-full max-w-2xl rounded border shadow-lg">
        <div class="px-4 py-3 border-b flex items-center justify-between">
          <h3 id="cloudfauxnt-origin-modal-title" class="font-semibold">Add Origin / Behavior</h3>
          <button class="h-7 w-7 rounded bg-slate-200 text-slate-700 text-sm" title="Close origin modal" aria-label="Close origin modal" onclick="closeCloudfauxntOriginModal()">✕</button>
        </div>
        <div class="p-4 grid gap-3">
          <input id="cloudfauxnt-origin-current-name" type="hidden" />
          <div>
            <label for="cloudfauxnt-origin-name" class="block text-xs text-slate-500 mb-1">Origin Name</label>
            <input id="cloudfauxnt-origin-name" class="w-full border rounded px-2 py-1 text-sm" placeholder="s3" />
          </div>
          <div>
            <label for="cloudfauxnt-origin-url" class="block text-xs text-slate-500 mb-1">Origin URL</label>
            <input id="cloudfauxnt-origin-url" class="w-full border rounded px-2 py-1 text-sm" placeholder="http://essthree:9300" />
          </div>
          <div>
            <label for="cloudfauxnt-origin-path-patterns" class="block text-xs text-slate-500 mb-1">Path Patterns (one per line)</label>
            <textarea id="cloudfauxnt-origin-path-patterns" rows="3" class="w-full border rounded px-2 py-1 text-sm" placeholder="/s3/*"></textarea>
          </div>
          <div class="grid md:grid-cols-2 gap-3">
            <div>
              <label for="cloudfauxnt-origin-strip-prefix" class="block text-xs text-slate-500 mb-1">Strip Prefix</label>
              <input id="cloudfauxnt-origin-strip-prefix" class="w-full border rounded px-2 py-1 text-sm" placeholder="/s3" />
            </div>
            <div>
              <label for="cloudfauxnt-origin-target-prefix" class="block text-xs text-slate-500 mb-1">Target Prefix</label>
              <input id="cloudfauxnt-origin-target-prefix" class="w-full border rounded px-2 py-1 text-sm" placeholder="/test-bucket" />
            </div>
          </div>
          <div class="grid md:grid-cols-2 gap-3">
            <div>
              <label for="cloudfauxnt-origin-default-root" class="block text-xs text-slate-500 mb-1">Default Root Object (optional)</label>
              <input id="cloudfauxnt-origin-default-root" class="w-full border rounded px-2 py-1 text-sm" placeholder="index.html" />
            </div>
            <div>
              <label for="cloudfauxnt-origin-require-signature" class="block text-xs text-slate-500 mb-1">Require Signature</label>
              <select id="cloudfauxnt-origin-require-signature" class="w-full border rounded px-2 py-1 text-sm">
                <option value="inherit">Inherit global setting</option>
                <option value="true">Required</option>
                <option value="false">Not required</option>
              </select>
            </div>
          </div>
          <div class="flex justify-end gap-2 pt-1">
            <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Cancel origin changes" aria-label="Cancel origin changes" onclick="closeCloudfauxntOriginModal()">Cancel</button>
            <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Save origin and behavior" aria-label="Save origin and behavior" onclick="saveCloudfauxntOrigin()">Save</button>
          </div>
        </div>
      </div>
    </div>

    <div id="cloudfauxnt-signing-modal" class="hidden fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50" onclick="if (event.target === this) closeCloudfauxntSigningSettingsModal()">
      <div class="bg-white w-full max-w-lg rounded border shadow-lg">
      <div class="px-4 py-3 border-b flex items-center justify-between">
        <h3 class="font-semibold">Edit Signing Settings</h3>
        <button class="h-7 w-7 rounded bg-slate-200 text-slate-700 text-sm" title="Close signing modal" aria-label="Close signing modal" onclick="closeCloudfauxntSigningSettingsModal()">✕</button>
      </div>
      <div class="p-4 grid gap-3">
        <div>
        <label for="cloudfauxnt-signing-key-pair-id" class="block text-xs text-slate-500 mb-1">Key Pair ID</label>
        <input id="cloudfauxnt-signing-key-pair-id" class="w-full border rounded px-2 py-1 text-sm" placeholder="K1234567890" />
        </div>
        <div>
        <label for="cloudfauxnt-signing-public-key-path" class="block text-xs text-slate-500 mb-1">Public Key Path</label>
        <input id="cloudfauxnt-signing-public-key-path" class="w-full border rounded px-2 py-1 text-sm" placeholder="keys/public_key.pem" />
        </div>
        <div class="grid md:grid-cols-2 gap-3">
        <div>
          <label for="cloudfauxnt-signing-clock-skew" class="block text-xs text-slate-500 mb-1">Clock Skew (seconds)</label>
          <input id="cloudfauxnt-signing-clock-skew" type="number" min="0" class="w-full border rounded px-2 py-1 text-sm" placeholder="30" />
        </div>
        <div>
          <label for="cloudfauxnt-signing-url-ttl" class="block text-xs text-slate-500 mb-1">Default URL TTL (seconds)</label>
          <input id="cloudfauxnt-signing-url-ttl" type="number" min="0" class="w-full border rounded px-2 py-1 text-sm" placeholder="300" />
        </div>
        </div>
        <div class="grid md:grid-cols-2 gap-3 items-end">
        <div>
          <label for="cloudfauxnt-signing-cookie-ttl" class="block text-xs text-slate-500 mb-1">Default Cookie TTL (seconds)</label>
          <input id="cloudfauxnt-signing-cookie-ttl" type="number" min="0" class="w-full border rounded px-2 py-1 text-sm" placeholder="300" />
        </div>
        <label class="inline-flex items-center gap-2 text-sm text-slate-700">
          <input id="cloudfauxnt-signing-allow-wildcards" type="checkbox" class="rounded border-slate-300" />
          Allow wildcard patterns
        </label>
        </div>
        <div class="flex justify-end gap-2 pt-1">
        <button class="px-3 py-1 rounded border border-slate-300 text-slate-700 text-sm" title="Cancel signing changes" aria-label="Cancel signing changes" onclick="closeCloudfauxntSigningSettingsModal()">Cancel</button>
        <button class="px-3 py-1 rounded bg-slate-900 text-white text-sm" title="Save signing settings" aria-label="Save signing settings" onclick="saveCloudfauxntSigningSettings()">Save</button>
        </div>
      </div>
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
      } else if (view === 'kay-vee') {
        kayVeeSummary = payload.summary || null;
        kayVeeActivityRows = payload.activity || [];
        kayVeeActivityNextToken = payload.nextToken || '';
        kayVeeParameterRows = payload.parameters || kayVeeParameterRows;
        kayVeeParameterNextToken = payload.parametersNextToken || kayVeeParameterNextToken;
        kayVeeSecretRows = payload.secrets || kayVeeSecretRows;
        kayVeeSecretNextToken = payload.secretsNextToken || kayVeeSecretNextToken;
        renderKayVeeOverview(payload);
      } else if (view === 'essthree') {
        essThreeActivityRows = payload.activity || [];
        essThreeActivityNextToken = payload.activityNextToken || '';
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
document.getElementById('menu-kay-vee').addEventListener('click', () => switchView('kay-vee'));
document.getElementById('menu-essthree').addEventListener('click', () => switchView('essthree'));
document.getElementById('menu-cloudfauxnt').addEventListener('click', () => switchView('cloudfauxnt'));

window.addEventListener('popstate', (event) => {
  const stateView = event.state?.view;
  const nextView = normalizeView(stateView || getViewFromLocation());
  switchView(nextView, { updateHistory: false });
});

sortMenuServicesAlphabetically();
switchView(getViewFromLocation(), { historyMode: 'replace' });
