package admin

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ess-enn-ess Admin Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #f5f7fa;
            color: #2c3e50;
            line-height: 1.6;
        }
        
        header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: #fff;
            padding: 2rem;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        
        header h1 {
            font-size: 2rem;
            font-weight: 600;
            margin-bottom: 0.5rem;
        }
        
        header p {
            font-size: 0.95rem;
            opacity: 0.9;
        }
        
        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 2rem;
        }
        
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        
        .stat-card {
            background: #fff;
            padding: 1.5rem;
            border-radius: 12px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            border-left: 4px solid #667eea;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        
        .stat-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        }
        
        .stat-label {
            font-size: 0.875rem;
            color: #7f8c8d;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 0.5rem;
        }
        
        .stat-value {
            font-size: 2.5rem;
            font-weight: 700;
            color: #667eea;
        }
        
        .stat-detail {
            font-size: 0.875rem;
            color: #95a5a6;
            margin-top: 0.5rem;
        }
        
        .tabs {
            background: #fff;
            border-radius: 12px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            margin-bottom: 2rem;
            overflow: hidden;
        }
        
        .tab-header {
            display: flex;
            border-bottom: 2px solid #f0f0f0;
            overflow-x: auto;
        }
        
        .tab-btn {
            padding: 1rem 2rem;
            background: none;
            border: none;
            cursor: pointer;
            font-size: 0.95rem;
            font-weight: 500;
            color: #7f8c8d;
            transition: all 0.3s;
            white-space: nowrap;
        }
        
        .tab-btn:hover {
            background: #f8f9fa;
            color: #667eea;
        }
        
        .tab-btn.active {
            color: #667eea;
            border-bottom: 3px solid #667eea;
        }
        
        .tab-content {
            padding: 2rem;
            display: none;
        }
        
        .tab-content.active {
            display: block;
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
            background: #fff;
            border-radius: 8px;
            overflow: hidden;
        }
        
        th {
            background: #f8f9fa;
            padding: 1rem;
            text-align: left;
            font-weight: 600;
            color: #2c3e50;
            border-bottom: 2px solid #e9ecef;
            font-size: 0.875rem;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        td {
            padding: 1rem;
            border-bottom: 1px solid #f0f0f0;
            font-size: 0.9rem;
        }
        
        tr:hover {
            background: #f8f9fa;
        }
        
        .badge {
            display: inline-block;
            padding: 0.375rem 0.75rem;
            border-radius: 20px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        .badge-success {
            background: #d4edda;
            color: #155724;
        }
        
        .badge-warning {
            background: #fff3cd;
            color: #856404;
        }
        
        .badge-danger {
            background: #f8d7da;
            color: #721c24;
        }
        
        .badge-info {
            background: #d1ecf1;
            color: #0c5460;
        }
        
        .badge-secondary {
            background: #e9ecef;
            color: #6c757d;
        }
        
        code {
            background: #f4f4f4;
            padding: 0.2rem 0.5rem;
            border-radius: 4px;
            font-size: 0.875rem;
            font-family: 'Courier New', monospace;
            color: #e83e8c;
        }
        
        .btn {
            padding: 0.5rem 1rem;
            background: #667eea;
            color: #fff;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.9rem;
            font-weight: 500;
            transition: all 0.3s;
            margin-right: 0.5rem;
        }
        
        .btn:hover {
            background: #5568d3;
            transform: translateY(-1px);
            box-shadow: 0 2px 8px rgba(102, 126, 234, 0.4);
        }
        
        .btn-secondary {
            background: #6c757d;
        }
        
        .btn-secondary:hover {
            background: #5a6268;
        }
        
        .btn-small {
            padding: 0.25rem 0.5rem;
            font-size: 0.75rem;
            margin: 0 0.25rem;
        }
        
        .btn-danger {
            background: #e74c3c;
        }
        
        .btn-danger:hover {
            background: #c0392b;
        }
        
        .form-card {
            background: #f8f9fa;
            padding: 1.5rem;
            border-radius: 8px;
            margin-bottom: 1.5rem;
            border: 2px dashed #dee2e6;
        }
        
        .form-group {
            margin-bottom: 1rem;
        }
        
        .form-group label {
            display: block;
            font-weight: 600;
            margin-bottom: 0.25rem;
            color: #495057;
            font-size: 0.9rem;
        }
        
        .form-group input,
        .form-group select {
            width: 100%;
            padding: 0.5rem;
            border: 1px solid #ced4da;
            border-radius: 4px;
            font-size: 0.9rem;
        }
        
        .form-group input:focus,
        .form-group select:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }
        
        .form-row {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 1rem;
        }
        
        .checkbox-group {
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        
        .checkbox-group input[type="checkbox"] {
            width: auto;
        }
        
        .activity-stream {
            max-height: 500px;
            overflow-y: auto;
            background: #fff;
            border-radius: 8px;
            padding: 1rem;
        }
        
        .activity-item {
            padding: 1rem;
            border-left: 3px solid #667eea;
            background: #f8f9fa;
            margin-bottom: 0.75rem;
            border-radius: 4px;
            transition: all 0.2s;
        }
        
        .activity-item:hover {
            transform: translateX(4px);
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        
        .activity-time {
            font-size: 0.75rem;
            color: #95a5a6;
            margin-bottom: 0.25rem;
        }
        
        .activity-type {
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 0.25rem;
        }
        
        .activity-detail {
            font-size: 0.875rem;
            color: #7f8c8d;
        }
        
        .empty-state {
            text-align: center;
            padding: 3rem;
            color: #95a5a6;
        }
        
        .empty-state svg {
            width: 64px;
            height: 64px;
            margin-bottom: 1rem;
            opacity: 0.3;
        }
        
        @media (max-width: 768px) {
            .stats-grid {
                grid-template-columns: 1fr;
            }
            
            .tab-header {
                flex-direction: column;
            }
            
            table {
                font-size: 0.8rem;
            }
        }
    </style>
</head>
<body>
    <header>
        <h1>🔔 ess-enn-ess Admin Dashboard</h1>
        <p>AWS SNS Emulator - Real-time monitoring and management</p>
    </header>
    
    <div class="container">
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-label">Topics</div>
                <div class="stat-value" id="totalTopics">0</div>
                <div class="stat-detail">Active topics</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Subscriptions</div>
                <div class="stat-value" id="totalSubs">0</div>
                <div class="stat-detail"><span id="confirmedSubs">0</span> confirmed</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Messages Published</div>
                <div class="stat-value" id="publishedCount">0</div>
                <div class="stat-detail">Total published</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Deliveries</div>
                <div class="stat-value" id="deliveredCount">0</div>
                <div class="stat-detail"><span id="failedCount">0</span> failed</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Total Events</div>
                <div class="stat-value" id="totalEvents">0</div>
                <div class="stat-detail">Activity log entries</div>
            </div>
        </div>
        
        <div class="tabs">
            <div class="tab-header">
                <button class="tab-btn active" onclick="showTab('topics')">📋 Topics</button>
                <button class="tab-btn" onclick="showTab('subscriptions')">📬 Subscriptions</button>
                <button class="tab-btn" onclick="showTab('activity')">📊 Activity Log</button>
                <button class="tab-btn" onclick="showTab('export')">💾 Export/Import</button>
            </div>
            
            <div id="topics-tab" class="tab-content active">
                <div class="form-card">
                    <h3 style="margin-bottom: 1rem;">Create New Topic</h3>
                    <div class="form-row">
                        <div class="form-group">
                            <label for="topicName">Topic Name *</label>
                            <input type="text" id="topicName" placeholder="my-topic-name" required>
                        </div>
                        <div class="form-group">
                            <label for="displayName">Display Name (optional)</label>
                            <input type="text" id="displayName" placeholder="My Topic">
                        </div>
                    </div>
                    <div class="checkbox-group" style="margin-bottom: 1rem;">
                        <input type="checkbox" id="fifoTopic">
                        <label for="fifoTopic" style="margin: 0;">FIFO Topic (.fifo suffix required)</label>
                    </div>
                    <button class="btn" onclick="createTopic()">➕ Create Topic</button>
                </div>
                
                <div style="margin-bottom: 1rem;">
                    <button class="btn" onclick="loadTopics()">🔄 Refresh</button>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>Topic ARN</th>
                            <th>Display Name</th>
                            <th>Type</th>
                            <th>Subscriptions</th>
                            <th>Created</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody id="topicsBody">
                        <tr><td colspan="6" class="empty-state">No topics yet</td></tr>
                    </tbody>
                </table>
            </div>
            
            <div id="subscriptions-tab" class="tab-content">
                <div class="form-card">
                    <h3 style="margin-bottom: 1rem;">Create New Subscription</h3>
                    <div class="form-row">
                        <div class="form-group">
                            <label for="subTopicArn">Topic ARN *</label>
                            <input type="text" id="subTopicArn" placeholder="arn:aws:sns:us-east-1:000000000000:my-topic" required>
                        </div>
                        <div class="form-group">
                            <label for="subProtocol">Protocol *</label>
                            <select id="subProtocol">
                                <option value="http">HTTP</option>
                                <option value="https">HTTPS</option>
                                <option value="sqs">SQS</option>
                                <option value="email">Email</option>
                            </select>
                        </div>
                    </div>
                    <div class="form-group">
                        <label for="subEndpoint">Endpoint *</label>
                        <input type="text" id="subEndpoint" placeholder="http://localhost:8080/webhook or queue-name" required>
                    </div>
                    <div class="checkbox-group" style="margin-bottom: 1rem;">
                        <input type="checkbox" id="autoConfirm" checked>
                        <label for="autoConfirm" style="margin: 0;">Auto-confirm subscription</label>
                    </div>
                    <button class="btn" onclick="createSubscription()">➕ Create Subscription</button>
                </div>
                
                <div style="margin-bottom: 1rem;">
                    <button class="btn" onclick="loadSubscriptions()">🔄 Refresh</button>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>Subscription ARN</th>
                            <th>Topic ARN</th>
                            <th>Protocol</th>
                            <th>Endpoint</th>
                            <th>Status</th>
                            <th>Created</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody id="subsBody">
                        <tr><td colspan="7" class="empty-state">No subscriptions yet</td></tr>
                    </tbody>
                </table>
            </div>
            
            <div id="activity-tab" class="tab-content">
                <div style="margin-bottom: 1rem;">
                    <button class="btn" onclick="loadActivities()">🔄 Refresh</button>
                    <button class="btn btn-secondary" onclick="clearFilters()">Clear Filters</button>
                </div>
                <div class="activity-stream" id="activityLog">
                    <div class="empty-state">Loading activities...</div>
                </div>
            </div>
            
            <div id="export-tab" class="tab-content">
                <h3 style="margin-bottom: 1rem;">Export Configuration</h3>
                <p style="margin-bottom: 1rem; color: #7f8c8d;">Download current topics and subscriptions as YAML</p>
                <button class="btn" onclick="exportConfig()">📥 Download Export</button>
                
                <h3 style="margin: 2rem 0 1rem;">Import Configuration</h3>
                <p style="margin-bottom: 1rem; color: #7f8c8d;">Import feature coming soon</p>
            </div>
        </div>
    </div>
    
    <script>
        let currentTab = 'topics';
        let autoRefresh = true;
        
        function showTab(tabName) {
            document.querySelectorAll('.tab-content').forEach(tab => tab.classList.remove('active'));
            document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
            
            document.getElementById(tabName + '-tab').classList.add('active');
            event.target.classList.add('active');
            currentTab = tabName;
            
            if (tabName === 'topics') loadTopics();
            else if (tabName === 'subscriptions') loadSubscriptions();
            else if (tabName === 'activity') loadActivities();
        }
        
        async function loadStats() {
            try {
                const response = await fetch('/api/stats');
                const stats = await response.json();
                
                document.getElementById('totalTopics').textContent = stats.topics.total;
                document.getElementById('totalSubs').textContent = stats.subscriptions.total;
                document.getElementById('confirmedSubs').textContent = stats.subscriptions.confirmed;
                document.getElementById('publishedCount').textContent = stats.messages.published;
                document.getElementById('deliveredCount').textContent = stats.messages.delivered;
                document.getElementById('failedCount').textContent = stats.messages.failed;
                document.getElementById('totalEvents').textContent = stats.events.total;
            } catch (error) {
                console.error('Error loading stats:', error);
            }
        }
        
        async function loadTopics() {
            try {
                const response = await fetch('/api/topics');
                const topics = await response.json();
                const tbody = document.getElementById('topicsBody');
                
                if (topics.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="6" class="empty-state">No topics yet. Create one using the form above.</td></tr>';
                    return;
                }
                
                let html = '';
                for (let i = 0; i < topics.length; i++) {
                    const topic = topics[i];
                    const topicType = topic.fifo_topic ? 'FIFO' : 'Standard';
                    const badgeClass = topic.fifo_topic ? 'info' : 'secondary';
                    const displayName = topic.display_name || '-';
                    const subCount = topic.subscription_count || 0;
                    const createdAt = new Date(topic.created_at).toLocaleString();
                    
                    html += '<tr>';
                    html += '<td><code>' + topic.topic_arn + '</code></td>';
                    html += '<td>' + displayName + '</td>';
                    html += '<td><span class="badge badge-' + badgeClass + '">' + topicType + '</span></td>';
                    html += '<td>' + subCount + '</td>';
                    html += '<td>' + createdAt + '</td>';
                    html += '<td><button class="btn btn-danger btn-small" onclick="deleteTopic(\'' + topic.topic_arn + '\')">🗑️ Delete</button></td>';
                    html += '</tr>';
                }
                tbody.innerHTML = html;
            } catch (error) {
                console.error('Error loading topics:', error);
            }
        }
        
        async function createTopic() {
            const name = document.getElementById('topicName').value.trim();
            const displayName = document.getElementById('displayName').value.trim();
            const isFifo = document.getElementById('fifoTopic').checked;
            
            if (!name) {
                alert('Topic name is required');
                return;
            }
            
            const attributes = {};
            if (displayName) attributes.DisplayName = displayName;
            if (isFifo) attributes.FifoTopic = 'true';
            
            try {
                const response = await fetch('/api/topics', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, attributes })
                });
                
                if (!response.ok) {
                    const error = await response.text();
                    alert('Error creating topic: ' + error);
                    return;
                }
                
                // Clear form
                document.getElementById('topicName').value = '';
                document.getElementById('displayName').value = '';
                document.getElementById('fifoTopic').checked = false;
                
                // Reload topics
                await loadTopics();
                await loadStats();
                alert('Topic created successfully!');
            } catch (error) {
                console.error('Error creating topic:', error);
                alert('Error creating topic: ' + error.message);
            }
        }
        
        async function deleteTopic(topicArn) {
            if (!confirm('Are you sure you want to delete this topic?\n\n' + topicArn)) {
                return;
            }
            
            try {
                const response = await fetch('/api/topics/delete', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ topic_arn: topicArn })
                });
                
                if (!response.ok) {
                    const error = await response.text();
                    alert('Error deleting topic: ' + error);
                    return;
                }
                
                await loadTopics();
                await loadStats();
                alert('Topic deleted successfully!');
            } catch (error) {
                console.error('Error deleting topic:', error);
                alert('Error deleting topic: ' + error.message);
            }
        }
        
        async function loadSubscriptions() {
            try {
                const response = await fetch('/api/subscriptions');
                const subs = await response.json();
                const tbody = document.getElementById('subsBody');
                
                if (subs.length === 0) {
                    tbody.innerHTML = '<tr><td colspan="7" class="empty-state">No subscriptions yet. Create one using the form above.</td></tr>';
                    return;
                }
                
                let html = '';
                for (let i = 0; i < subs.length; i++) {
                    const sub = subs[i];
                    let badgeClass = 'secondary';
                    if (sub.status === 'confirmed') badgeClass = 'success';
                    else if (sub.status === 'pending') badgeClass = 'warning';
                    
                    const createdAt = new Date(sub.created_at).toLocaleString();
                    
                    html += '<tr>';
                    html += '<td><code style="font-size: 0.75rem;">' + sub.subscription_arn + '</code></td>';
                    html += '<td><code style="font-size: 0.75rem;">' + sub.topic_arn + '</code></td>';
                    html += '<td><span class="badge badge-info">' + sub.protocol + '</span></td>';
                    html += '<td><code style="font-size: 0.75rem;">' + sub.endpoint + '</code></td>';
                    html += '<td><span class="badge badge-' + badgeClass + '">' + sub.status + '</span></td>';
                    html += '<td>' + createdAt + '</td>';
                    html += '<td><button class="btn btn-danger btn-small" onclick="deleteSubscription(\'' + sub.subscription_arn + '\')">🗑️ Delete</button></td>';
                    html += '</tr>';
                }
                tbody.innerHTML = html;
            } catch (error) {
                console.error('Error loading subscriptions:', error);
            }
        }
        
        async function createSubscription() {
            const topicArn = document.getElementById('subTopicArn').value.trim();
            const protocol = document.getElementById('subProtocol').value;
            const endpoint = document.getElementById('subEndpoint').value.trim();
            const autoConfirm = document.getElementById('autoConfirm').checked;
            
            if (!topicArn || !endpoint) {
                alert('Topic ARN and Endpoint are required');
                return;
            }
            
            try {
                const response = await fetch('/api/subscriptions', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ 
                        topic_arn: topicArn, 
                        protocol: protocol, 
                        endpoint: endpoint,
                        auto_confirm: autoConfirm
                    })
                });
                
                if (!response.ok) {
                    const error = await response.text();
                    alert('Error creating subscription: ' + error);
                    return;
                }
                
                // Clear form
                document.getElementById('subTopicArn').value = '';
                document.getElementById('subEndpoint').value = '';
                document.getElementById('autoConfirm').checked = true;
                
                // Reload subscriptions
                await loadSubscriptions();
                await loadStats();
                alert('Subscription created successfully!');
            } catch (error) {
                console.error('Error creating subscription:', error);
                alert('Error creating subscription: ' + error.message);
            }
        }
        
        async function deleteSubscription(subscriptionArn) {
            if (!confirm('Are you sure you want to delete this subscription?\n\n' + subscriptionArn)) {
                return;
            }
            
            try {
                const response = await fetch('/api/subscriptions/delete', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ subscription_arn: subscriptionArn })
                });
                
                if (!response.ok) {
                    const error = await response.text();
                    alert('Error deleting subscription: ' + error);
                    return;
                }
                
                await loadSubscriptions();
                await loadStats();
                alert('Subscription deleted successfully!');
            } catch (error) {
                console.error('Error deleting subscription:', error);
                alert('Error deleting subscription: ' + error.message);
            }
        }
        
        async function loadActivities() {
            try {
                const response = await fetch('/api/activities');
                const activities = await response.json();
                const log = document.getElementById('activityLog');
                
                if (activities.length === 0) {
                    log.innerHTML = '<div class="empty-state">No activity logged yet</div>';
                    return;
                }
                
                let html = '';
                const reversed = activities.slice().reverse();
                for (let i = 0; i < reversed.length; i++) {
                    const activity = reversed[i];
                    const timestamp = new Date(activity.timestamp).toLocaleString();
                    const eventType = activity.event_type.replace(/_/g, ' ').toUpperCase();
                    
                    let badgeClass = 'info';
                    if (activity.status === 'success') badgeClass = 'success';
                    else if (activity.status === 'failed') badgeClass = 'danger';
                    else if (activity.status === 'retrying') badgeClass = 'warning';
                    
                    html += '<div class="activity-item">';
                    html += '<div class="activity-time">' + timestamp + '</div>';
                    html += '<div class="activity-type">' + eventType + '</div>';
                    html += '<div class="activity-detail">';
                    html += '<span class="badge badge-' + badgeClass + '">' + activity.status + '</span> ';
                    if (activity.topic_arn) html += '<code>' + activity.topic_arn + '</code> ';
                    if (activity.message_id) html += 'Message: ' + activity.message_id + ' ';
                    if (activity.duration_ms) html += '(' + activity.duration_ms + 'ms) ';
                    if (activity.error) html += '<br><span style="color: #e74c3c;">' + activity.error + '</span>';
                    html += '</div>';
                    html += '</div>';
                }
                log.innerHTML = html;
            } catch (error) {
                console.error('Error loading activities:', error);
            }
        }
        
        function exportConfig() {
            window.location.href = '/api/export';
        }
        
        function clearFilters() {
            loadActivities();
        }
        
        // Initial load
        window.addEventListener('load', () => {
            loadStats();
            loadTopics();
            
            // Auto-refresh every 3 seconds
            setInterval(() => {
                if (autoRefresh) {
                    loadStats();
                    if (currentTab === 'topics') loadTopics();
                    else if (currentTab === 'subscriptions') loadSubscriptions();
                    else if (currentTab === 'activity') loadActivities();
                }
            }, 3000);
        });
    </script>
</body>
</html>`
