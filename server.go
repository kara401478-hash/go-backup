package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yourusername/go-backup/internal/backup"
	"github.com/yourusername/go-backup/internal/scheduler"
)

// HistoryEntry はバックアップ履歴の1件
type HistoryEntry struct {
	backup.Result
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// Server はWebサーバー
type Server struct {
	port      int
	scheduler *scheduler.Scheduler
	mu        sync.Mutex
	history   []HistoryEntry
	running   bool
}

// New はServerを作成する
func New(port int) *Server {
	s := &Server{port: port}
	s.scheduler = scheduler.New(func(result backup.Result) {
		sch := s.scheduler.Get()
		s.addHistory(HistoryEntry{Result: result, Src: sch.Src, Dst: sch.Dst})
	})
	s.scheduler.Start()
	return s
}

func (s *Server) addHistory(entry HistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append([]HistoryEntry{entry}, s.history...)
	if len(s.history) > 20 {
		s.history = s.history[:20]
	}
}

// Start はサーバーを起動する
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/", s.handleUI)
	mux.HandleFunc("/api/backup", s.handleBackup)
	mux.HandleFunc("/api/schedule", s.handleSchedule)
	mux.HandleFunc("/api/history", s.handleHistory)
	mux.HandleFunc("/api/status", s.handleStatus)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🚀 go-backup 起動 → http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleBackup は今すぐバックアップを実行する
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var req struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Src == "" || req.Dst == "" {
		http.Error(w, `{"error":"srcとdstが必要です"}`, 400)
		return
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		fmt.Fprint(w, `{"error":"バックアップが既に実行中です"}`)
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		log.Printf("バックアップ開始: %s → %s", req.Src, req.Dst)
		result := backup.Run(req.Src, req.Dst)
		s.addHistory(HistoryEntry{Result: result, Src: req.Src, Dst: req.Dst})
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"message":"バックアップを開始しました"}`)
}

// handleSchedule はスケジュールの取得・設定
func (s *Server) handleSchedule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		sch := s.scheduler.Get()
		json.NewEncoder(w).Encode(sch)
		return
	}

	if r.Method == http.MethodPost {
		var sch scheduler.Schedule
		if err := json.NewDecoder(r.Body).Decode(&sch); err != nil {
			http.Error(w, `{"error":"パースエラー"}`, 400)
			return
		}
		s.scheduler.Set(sch)
		fmt.Fprint(w, `{"message":"スケジュールを保存しました"}`)
		return
	}

	http.Error(w, "Method not allowed", 405)
}

// handleHistory はバックアップ履歴を返す
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if s.history == nil {
		fmt.Fprint(w, "[]")
		return
	}
	json.NewEncoder(w).Encode(s.history)
}

// handleStatus は実行中かどうかを返す
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	running := s.running
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"running":%v,"time":"%s"}`, running, time.Now().Format("15:04:05"))
}

// handleUI はブラウザUIを返す
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, ui)
}

const ui = `<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>go-backup</title>
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: system-ui, sans-serif; background: #f5f5f4; color: #1c1917; min-height: 100vh; }
.header { background: white; border-bottom: 1px solid #e7e5e4; padding: 1rem 1.5rem; display: flex; align-items: center; gap: 10px; }
.dot { width: 10px; height: 10px; border-radius: 50%; background: #4ade80; }
.dot.running { background: #f59e0b; animation: pulse 1s infinite; }
@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.4} }
.title { font-size: 18px; font-weight: 600; }
.time { margin-left: auto; font-size: 13px; color: #78716c; }
.container { max-width: 800px; margin: 0 auto; padding: 1.5rem; display: grid; gap: 1rem; }
.card { background: white; border: 1px solid #e7e5e4; border-radius: 12px; padding: 1.25rem; }
.card-title { font-size: 14px; font-weight: 600; color: #78716c; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 1rem; }
.field { margin-bottom: 12px; }
.field label { display: block; font-size: 13px; color: #78716c; margin-bottom: 4px; }
.field input { width: 100%; padding: 8px 12px; border: 1px solid #d4d4d4; border-radius: 8px; font-size: 14px; outline: none; transition: border-color .2s; }
.field input:focus { border-color: #a78bfa; }
.btn { padding: 9px 18px; border-radius: 8px; border: none; font-size: 14px; cursor: pointer; font-weight: 500; transition: all .15s; }
.btn-primary { background: #7c3aed; color: white; }
.btn-primary:hover { background: #6d28d9; }
.btn-primary:disabled { background: #c4b5fd; cursor: not-allowed; }
.btn-secondary { background: white; border: 1px solid #d4d4d4; color: #1c1917; }
.btn-secondary:hover { background: #f5f5f4; }
.row { display: flex; gap: 10px; align-items: flex-end; }
.row .field { flex: 1; margin-bottom: 0; }
.toggle { display: flex; align-items: center; gap: 10px; margin-bottom: 12px; }
.toggle input[type=checkbox] { width: 18px; height: 18px; cursor: pointer; }
.time-row { display: flex; gap: 10px; }
.time-row .field { flex: 1; }
.msg { padding: 10px 14px; border-radius: 8px; font-size: 13px; margin-top: 10px; display: none; }
.msg.success { background: #f0fdf4; color: #166534; border: 1px solid #bbf7d0; }
.msg.error { background: #fef2f2; color: #991b1b; border: 1px solid #fecaca; }
.msg.info { background: #eff6ff; color: #1e40af; border: 1px solid #bfdbfe; }
.history-item { padding: 10px 0; border-bottom: 1px solid #f5f5f4; display: flex; align-items: center; gap: 12px; font-size: 13px; }
.history-item:last-child { border-bottom: none; }
.badge { padding: 2px 8px; border-radius: 5px; font-size: 11px; font-weight: 600; }
.badge.ok { background: #dcfce7; color: #166534; }
.badge.ng { background: #fee2e2; color: #991b1b; }
.history-info { flex: 1; }
.history-path { color: #78716c; font-size: 12px; margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 400px; }
.history-meta { color: #78716c; text-align: right; }
.empty { text-align: center; color: #a8a29e; padding: 2rem; font-size: 14px; }
</style>
</head>
<body>
<div class="header">
  <div class="dot" id="dot"></div>
  <span class="title">go-backup</span>
  <span class="time" id="clock"></span>
</div>
<div class="container">

  <!-- 今すぐバックアップ -->
  <div class="card">
    <div class="card-title">📦 今すぐバックアップ</div>
    <div class="row">
      <div class="field">
        <label>バックアップ元フォルダ</label>
        <input type="text" id="src" placeholder="例: C:\Users\name\Documents" />
      </div>
      <div class="field">
        <label>バックアップ先フォルダ</label>
        <input type="text" id="dst" placeholder="例: D:\Backup" />
      </div>
      <button class="btn btn-primary" id="runBtn" onclick="runBackup()">実行</button>
    </div>
    <div class="msg" id="runMsg"></div>
  </div>

  <!-- スケジュール -->
  <div class="card">
    <div class="card-title">⏰ 自動バックアップ</div>
    <div class="toggle">
      <input type="checkbox" id="schedEnabled" />
      <label for="schedEnabled" style="font-size:14px;">スケジュール実行を有効にする</label>
    </div>
    <div class="row">
      <div class="field">
        <label>バックアップ元</label>
        <input type="text" id="schedSrc" placeholder="例: C:\Users\name\Documents" />
      </div>
      <div class="field">
        <label>バックアップ先</label>
        <input type="text" id="schedDst" placeholder="例: D:\Backup" />
      </div>
    </div>
    <div class="time-row">
      <div class="field">
        <label>実行時刻（時）</label>
        <input type="number" id="schedHour" min="0" max="23" value="2" />
      </div>
      <div class="field">
        <label>実行時刻（分）</label>
        <input type="number" id="schedMin" min="0" max="59" value="0" />
      </div>
      <button class="btn btn-secondary" onclick="saveSchedule()" style="align-self:flex-end">保存</button>
    </div>
    <div class="msg" id="schedMsg"></div>
  </div>

  <!-- 履歴 -->
  <div class="card">
    <div class="card-title">📋 バックアップ履歴</div>
    <div id="history"><div class="empty">まだバックアップがありません</div></div>
  </div>

</div>
<script>
function showMsg(id, type, text) {
  const el = document.getElementById(id);
  el.className = 'msg ' + type;
  el.textContent = text;
  el.style.display = 'block';
  if (type !== 'error') setTimeout(() => el.style.display = 'none', 4000);
}

async function runBackup() {
  const src = document.getElementById('src').value.trim();
  const dst = document.getElementById('dst').value.trim();
  if (!src || !dst) { showMsg('runMsg', 'error', 'フォルダを両方入力してください'); return; }

  document.getElementById('runBtn').disabled = true;
  showMsg('runMsg', 'info', '⏳ バックアップを開始しました...');

  try {
    const res = await fetch('/api/backup', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({src, dst})
    });
    const data = await res.json();
    if (res.ok) {
      showMsg('runMsg', 'info', '⏳ バックアップ実行中... 完了したら履歴に表示されます');
      setTimeout(loadHistory, 3000);
    } else {
      showMsg('runMsg', 'error', data.error || 'エラーが発生しました');
      document.getElementById('runBtn').disabled = false;
    }
  } catch(e) {
    showMsg('runMsg', 'error', '接続エラー');
    document.getElementById('runBtn').disabled = false;
  }
}

async function saveSchedule() {
  const sch = {
    enabled: document.getElementById('schedEnabled').checked,
    src: document.getElementById('schedSrc').value.trim(),
    dst: document.getElementById('schedDst').value.trim(),
    hour: parseInt(document.getElementById('schedHour').value),
    minute: parseInt(document.getElementById('schedMin').value),
  };
  try {
    const res = await fetch('/api/schedule', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(sch)
    });
    const data = await res.json();
    showMsg('schedMsg', 'success', '✅ ' + (data.message || '保存しました'));
  } catch(e) {
    showMsg('schedMsg', 'error', '保存に失敗しました');
  }
}

async function loadSchedule() {
  try {
    const res = await fetch('/api/schedule');
    const data = await res.json();
    document.getElementById('schedEnabled').checked = data.enabled || false;
    document.getElementById('schedSrc').value = data.src || '';
    document.getElementById('schedDst').value = data.dst || '';
    document.getElementById('schedHour').value = data.hour ?? 2;
    document.getElementById('schedMin').value = data.minute ?? 0;
  } catch(e) {}
}

async function loadHistory() {
  try {
    const res = await fetch('/api/history');
    const data = await res.json();
    const el = document.getElementById('history');
    if (!data || data.length === 0) {
      el.innerHTML = '<div class="empty">まだバックアップがありません</div>';
      return;
    }
    el.innerHTML = data.map(h => {
      const ok = h.success;
      const time = new Date(h.started_at).toLocaleString('ja-JP');
      const duration = h.finished_at ? Math.round((new Date(h.finished_at) - new Date(h.started_at)) / 1000) : 0;
      return '<div class="history-item">' +
        '<span class="badge ' + (ok ? 'ok' : 'ng') + '">' + (ok ? '成功' : '失敗') + '</span>' +
        '<div class="history-info">' +
          '<div>' + (h.src || '') + ' → ' + (h.dst || '') + '</div>' +
          '<div class="history-path">' + (ok ? h.files_copied + ' ファイル / ' + formatBytes(h.total_bytes) : h.error) + '</div>' +
        '</div>' +
        '<div class="history-meta">' + time + '<br>' + duration + '秒</div>' +
        '</div>';
    }).join('');
  } catch(e) {}
}

async function checkStatus() {
  try {
    const res = await fetch('/api/status');
    const data = await res.json();
    const dot = document.getElementById('dot');
    const btn = document.getElementById('runBtn');
    if (data.running) {
      dot.className = 'dot running';
      btn.disabled = true;
    } else {
      dot.className = 'dot';
      btn.disabled = false;
    }
    document.getElementById('clock').textContent = data.time;
  } catch(e) {}
}

function formatBytes(bytes) {
  if (!bytes) return '0 B';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024*1024) return (bytes/1024).toFixed(1) + ' KB';
  if (bytes < 1024*1024*1024) return (bytes/1024/1024).toFixed(1) + ' MB';
  return (bytes/1024/1024/1024).toFixed(1) + ' GB';
}

// 初期化
loadSchedule();
loadHistory();
checkStatus();
setInterval(checkStatus, 3000);
setInterval(loadHistory, 5000);
</script>
</body>
</html>`
