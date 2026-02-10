package admin

import (
"fmt"
"log/slog"
"net/http"
"time"

"github.com/tonyellard/ess-enn-ess/internal/activity"
"github.com/tonyellard/ess-enn-ess/internal/config"
"github.com/tonyellard/ess-enn-ess/internal/topic"
)

// Server represents the admin dashboard server
type Server struct {
config         *config.Config
logger         *slog.Logger
topicStore     *topic.Store
activityLogger *activity.Logger
httpServer     *http.Server
mux            *http.ServeMux
}

// NewServer creates a new admin server
func NewServer(cfg *config.Config, logger *slog.Logger, topicStore *topic.Store, activityLogger *activity.Logger) *Server {
s := &Server{
config:         cfg,
logger:         logger,
topicStore:     topicStore,
activityLogger: activityLogger,
mux:            http.NewServeMux(),
}
s.registerRoutes()
return s
}

// registerRoutes registers all admin routes
func (s *Server) registerRoutes() {
s.mux.HandleFunc("/health", s.handleHealth)
s.mux.HandleFunc("/api/topics", s.handleGetTopics)
s.mux.HandleFunc("/api/activities", s.handleGetActivities)
s.mux.HandleFunc("/api/export", s.handleExport)
s.mux.HandleFunc("/api/import", s.handleImport)
s.mux.HandleFunc("/api/activities-stream", s.handleActivityStream)
s.mux.HandleFunc("/", s.handleDashboard)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "text/plain")
w.WriteHeader(http.StatusOK)
fmt.Fprint(w, "OK")
}

// handleDashboard serves the admin dashboard
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "text/html; charset=utf-8")
fmt.Fprint(w, dashboardHTML)
}

// handleGetTopics returns all topics as JSON
func (s *Server) handleGetTopics(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}

topics := s.topicStore.ListTopics()
w.Header().Set("Content-Type", "application/json")
fmt.Fprintf(w, "[")
for i, t := range topics {
if i > 0 {
fmt.Fprintf(w, ",")
}
fmt.Fprintf(w, `{"topic_arn":"%s","display_name":"%s","fifo_topic":%v,"content_based_deduplication":%v,"created_at":"%s","subscription_count":%d}`,
t.TopicArn, t.DisplayName, t.FifoTopic, t.ContentBased, t.CreatedAt.Format(time.RFC3339), t.SubscriptionCount)
}
fmt.Fprintf(w, "]")
}

// handleGetActivities returns activity log entries as JSON
func (s *Server) handleGetActivities(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}

topicArn := r.URL.Query().Get("topic")
eventType := r.URL.Query().Get("event")
status := r.URL.Query().Get("status")
limit := 100

entries := s.activityLogger.GetEntries()
filtered := make([]*activity.Entry, 0)
for _, entry := range entries {
if topicArn != "" && entry.TopicArn != topicArn {
continue
}
if eventType != "" && string(entry.EventType) != eventType {
continue
}
if status != "" && string(entry.Status) != status {
continue
}
filtered = append(filtered, entry)
}

if len(filtered) > limit {
filtered = filtered[len(filtered)-limit:]
}

w.Header().Set("Content-Type", "application/json")
fmt.Fprintf(w, "[")
for i, entry := range filtered {
if i > 0 {
fmt.Fprintf(w, ",")
}
fmt.Fprintf(w, `{"id":"%s","timestamp":"%s","event_type":"%s","topic_arn":"%s","status":"%s","duration_ms":%d,"error":"%s"}`,
entry.Id, entry.Timestamp.Format(time.RFC3339), entry.EventType, entry.TopicArn, entry.Status, entry.Duration.Milliseconds(), entry.Error)
}
fmt.Fprintf(w, "]")
}

// handleExport exports current state as YAML
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodGet {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}

w.Header().Set("Content-Type", "application/yaml")
w.Header().Set("Content-Disposition", "attachment; filename=sns_export.yaml")

topics := s.topicStore.ListTopics()
fmt.Fprintf(w, "topics:\n")
for _, t := range topics {
fmt.Fprintf(w, "  - arn: %s\n", t.TopicArn)
fmt.Fprintf(w, "    display_name: %s\n", t.DisplayName)
fmt.Fprintf(w, "    fifo_topic: %v\n", t.FifoTopic)
fmt.Fprintf(w, "    content_based_deduplication: %v\n", t.ContentBased)
}

s.logger.Info("SNS state exported")
}

// handleImport imports configuration from YAML (placeholder)
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
if r.Method != http.MethodPost {
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
return
}

w.Header().Set("Content-Type", "application/json")
fmt.Fprintf(w, `{"status":"success","message":"Import feature coming soon"}`)
s.logger.Info("Import endpoint called (not yet implemented)")
}

// handleActivityStream handles activity stream (placeholder)
func (s *Server) handleActivityStream(w http.ResponseWriter, r *http.Request) {
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
fmt.Fprintf(w, "data: {\"status\":\"connected\"}\n\n")
s.logger.Debug("Activity stream connected")
}

// Start starts the admin HTTP server
func (s *Server) Start() error {
s.httpServer = &http.Server{
Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.AdminPort),
Handler:      s.mux,
ReadTimeout:  time.Duration(s.config.Server.TimeoutSec) * time.Second,
WriteTimeout: time.Duration(s.config.Server.TimeoutSec) * time.Second,
}

s.logger.Info("Admin dashboard starting", "address", s.httpServer.Addr, "url", fmt.Sprintf("http://localhost:%d", s.config.Server.AdminPort))
return s.httpServer.ListenAndServe()
}

// Stop stops the admin HTTP server
func (s *Server) Stop() error {
if s.httpServer != nil {
return s.httpServer.Close()
}
return nil
}

var dashboardHTML = `<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"><title>SNS Dashboard</title><style>*{margin:0;padding:0;box-sizing:border-box}body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#f5f5f5;color:#333}header{background:#222;color:#fff;padding:1rem 2rem}header h1{font-size:1.5rem}header p{font-size:0.9rem;opacity:0.8;margin-top:0.25rem}.container{max-width:1200px;margin:0 auto;padding:2rem}.dashboard{display:grid;grid-template-columns:1fr 2fr;gap:2rem}.sidebar{background:#fff;border-radius:8px;padding:1.5rem;box-shadow:0 1px 3px rgba(0,0,0,0.1);height:fit-content}.sidebar h2{font-size:1.1rem;margin-bottom:1rem;border-bottom:2px solid #0066cc;padding-bottom:0.5rem}.sidebar ul{list-style:none}.sidebar a{color:#0066cc;cursor:pointer}.main{background:#fff;border-radius:8px;padding:1.5rem;box-shadow:0 1px 3px rgba(0,0,0,0.1)}.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:1rem;margin-bottom:2rem}.stat-card{background:#f9f9f9;padding:1rem;border-left:4px solid #0066cc;border-radius:4px}.stat-card .value{font-size:2rem;font-weight:bold;color:#0066cc;margin-top:0.5rem}table{width:100%;border-collapse:collapse;margin-top:1rem}th{background:#f5f5f5;padding:0.75rem;text-align:left;font-weight:600;border-bottom:2px solid #ddd}td{padding:0.75rem;border-bottom:1px solid #eee}.activity-log{max-height:400px;overflow-y:auto}.activity-entry{padding:0.75rem;border-left:3px solid #0066cc;background:#f9f9f9;margin-bottom:0.5rem;border-radius:2px}.status{display:inline-block;padding:0.25rem 0.75rem;border-radius:12px;font-size:0.85rem}.status.success{background:#d4edda;color:#155724}.status.failed{background:#f8d7da;color:#721c24}button{background:#0066cc;color:#fff;border:none;padding:0.5rem 1rem;border-radius:4px;cursor:pointer;font-size:0.9rem}button:hover{background:#0052a3}</style></head><body><header><h1>SNS Dashboard</h1><p>Real-time SNS monitoring</p></header><div class="container"><div class="dashboard"><aside class="sidebar"><h2>Topics</h2><ul id="topicList"><li><em>Loading...</em></li></ul></aside><main class="main"><div class="stats"><div class="stat-card"><div>Topics</div><div class="value" id="topicCount">0</div></div><div class="stat-card"><div>Subscriptions</div><div class="value" id="subscriptionCount">0</div></div><div class="stat-card"><div>Events</div><div class="value" id="eventCount">0</div></div></div><div><h2>Topics <button onclick="loadTopics()">Refresh</button></h2><table><thead><tr><th>ARN</th><th>Name</th><th>Type</th><th>Subs</th></tr></thead><tbody id="topicsBody"><tr><td colspan="4">No topics</td></tr></tbody></table></div><div style="margin-top:2rem"><h2>Activity Log <button onclick="loadActivities()">Refresh</button></h2><div class="activity-log" id="activityLog"><div>Loading...</div></div></div></main></div></div><script>async function loadTopics(){const r=await fetch('/api/topics');const t=await r.json();document.getElementById('topicCount').textContent=t.length;const tbody=document.getElementById('topicsBody');if(t.length===0){tbody.innerHTML='<tr><td colspan="4">No topics</td></tr>'}else{tbody.innerHTML=t.map(x=>'<tr><td><code>'+x.topic_arn+'</code></td><td>'+x.display_name+'</td><td>'+(x.fifo_topic?'FIFO':'Standard')+'</td><td>'+x.subscription_count+'</td></tr>').join('')}}async function loadActivities(){const r=await fetch('/api/activities');const a=await r.json();document.getElementById('eventCount').textContent=a.length;const log=document.getElementById('activityLog');if(a.length===0){log.innerHTML='<div>No activity</div>'}else{log.innerHTML=a.map(x=>'<div class="activity-entry"><div class="activity-time">'+new Date(x.timestamp).toLocaleTimeString()+'</div><div>'+x.event_type+' <span class="status '+x.status+'">'+x.status+'</span></div></div>').join('')}}window.addEventListener('load',()=>{loadTopics();loadActivities();setInterval(loadActivities,3000)})</script></body></html>`
