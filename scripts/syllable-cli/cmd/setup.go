package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Config types (setup-local)
// ---------------------------------------------------------------------------

type setupConfig struct {
	DefaultOrg   string                `yaml:"default_org,omitempty"   json:"default_org"`
	DefaultEnv   string                `yaml:"default_env,omitempty"   json:"default_env"`
	Environments map[string]setupEnv   `yaml:"environments,omitempty"  json:"environments"`
	Orgs         map[string]setupOrg   `yaml:"orgs,omitempty"          json:"orgs"`
}

type setupEnv struct {
	BaseURL string `yaml:"base_url" json:"base_url"`
}

type setupOrg struct {
	APIKey string                  `yaml:"api_key,omitempty" json:"api_key"`
	Envs   map[string]setupOrgEnv  `yaml:"envs,omitempty"    json:"envs"`
}

type setupOrgEnv struct {
	APIKey string `yaml:"api_key" json:"api_key"`
}

const setupKeyPlaceholder = "__existing__"

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Configure orgs, API keys, and environments",
		Long:  "Opens a browser-based UI to configure ~/.syllable/config.yaml.\nExisting API keys are never sent to the browser.",
		Example: `  syllable setup`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := setupConfigFilePath()
			if err != nil {
				return fmt.Errorf("cannot determine config path: %w", err)
			}

			real := setupLoadConfig(cfgPath)
			masked := setupMaskKeys(real)

			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return fmt.Errorf("cannot start server: %w", err)
			}
			port := ln.Addr().(*net.TCPAddr).Port
			url := fmt.Sprintf("http://127.0.0.1:%d", port)

			mux := http.NewServeMux()
			srv := &http.Server{Handler: mux}
			done := make(chan struct{})

			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				cfgJSON, _ := json.Marshal(masked)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				page := strings.ReplaceAll(setupHTMLPage, "%%INIT_CONFIG%%", string(cfgJSON))
				fmt.Fprint(w, page)
			})

			mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", 405)
					return
				}
				var incoming setupConfig
				if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
					http.Error(w, "bad request", 400)
					return
				}

				merged := setupMergeKeys(&incoming, real)

				if err := setupWriteConfig(cfgPath, merged); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(500)
					json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
					return
				}

				real = merged
				masked = setupMaskKeys(real)

				exit := r.URL.Query().Get("exit") == "true"
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"path": cfgPath})
				if exit {
					go func() {
						time.Sleep(500 * time.Millisecond)
						close(done)
					}()
				}
			})

			fmt.Printf("Opening Syllable CLI Setup at %s\n", url)
			fmt.Println("Press Ctrl+C to cancel.")

			go setupOpenBrowser(url)
			go func() {
				if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
					fmt.Fprintf(os.Stderr, "server error: %v\n", err)
				}
			}()

			<-done
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			srv.Shutdown(ctx)
			fmt.Printf("\nConfig saved to %s\n", cfgPath)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".syllable", "config.yaml"), nil
}

func setupLoadConfig(path string) *setupConfig {
	cfg := &setupConfig{
		DefaultEnv:   "prod",
		Environments: map[string]setupEnv{},
		Orgs:         map[string]setupOrg{},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = yaml.Unmarshal(data, cfg)
	if cfg.Environments == nil {
		cfg.Environments = map[string]setupEnv{}
	}
	if cfg.Orgs == nil {
		cfg.Orgs = map[string]setupOrg{}
	}
	for name, org := range cfg.Orgs {
		if org.Envs == nil {
			org.Envs = map[string]setupOrgEnv{}
		}
		cfg.Orgs[name] = org
	}
	return cfg
}

func setupWriteConfig(path string, cfg *setupConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func setupMaskKeys(cfg *setupConfig) *setupConfig {
	out := &setupConfig{
		DefaultOrg:   cfg.DefaultOrg,
		DefaultEnv:   cfg.DefaultEnv,
		Environments: cfg.Environments,
		Orgs:         map[string]setupOrg{},
	}
	for name, org := range cfg.Orgs {
		masked := setupOrg{
			APIKey: setupMaskKey(org.APIKey),
			Envs:   map[string]setupOrgEnv{},
		}
		for env, e := range org.Envs {
			masked.Envs[env] = setupOrgEnv{APIKey: setupMaskKey(e.APIKey)}
		}
		out.Orgs[name] = masked
	}
	return out
}

func setupMaskKey(k string) string {
	if k == "" {
		return ""
	}
	return setupKeyPlaceholder
}

func setupMergeKeys(incoming *setupConfig, stored *setupConfig) *setupConfig {
	for name, org := range incoming.Orgs {
		if org.APIKey == setupKeyPlaceholder {
			if s, ok := stored.Orgs[name]; ok {
				org.APIKey = s.APIKey
			} else {
				org.APIKey = ""
			}
		}
		for env, e := range org.Envs {
			if e.APIKey == setupKeyPlaceholder {
				if s, ok := stored.Orgs[name]; ok {
					if se, ok := s.Envs[env]; ok {
						e.APIKey = se.APIKey
					} else {
						e.APIKey = ""
					}
				} else {
					e.APIKey = ""
				}
				org.Envs[env] = e
			}
		}
		if len(org.Envs) == 0 {
			org.Envs = nil
		}
		incoming.Orgs[name] = org
	}
	if len(incoming.Environments) == 0 {
		incoming.Environments = nil
	}
	return incoming
}

func setupOpenBrowser(url string) {
	time.Sleep(300 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		cmd = exec.Command("cmd", "/c", "start", url)
	}
	_ = cmd.Start()
}

// ---------------------------------------------------------------------------
// Embedded UI  (%%INIT_CONFIG%% is replaced with JSON-encoded masked config)
// ---------------------------------------------------------------------------

const setupHTMLPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Syllable CLI Setup</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
:root {
  --bg: #0f1117; --surface: #1a1d27; --surface2: #22263a; --border: #2e3350;
  --accent: #00c896; --text: #e8eaf0; --muted: #8890a8; --danger: #ff5f6d;
  --warn: #f5a623; --radius: 10px;
  --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
}
body { background: var(--bg); color: var(--text); font-family: var(--font); font-size: 14px; min-height: 100vh; padding: 40px 20px 100px; }
.container { max-width: 720px; margin: 0 auto; }
header { margin-bottom: 36px; }
header h1 { font-size: 22px; font-weight: 600; color: var(--accent); letter-spacing: -0.3px; }
header p { color: var(--muted); margin-top: 6px; font-size: 13px; }
section { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius); padding: 24px; margin-bottom: 20px; }
section h2 { font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.8px; color: var(--muted); margin-bottom: 18px; }
.field { margin-bottom: 0; }
.field label { display: block; font-size: 12px; color: var(--muted); margin-bottom: 5px; }
.field input, .field select {
  width: 100%; padding: 9px 12px; background: var(--surface2);
  border: 1px solid var(--border); border-radius: 6px; color: var(--text);
  font-size: 14px; font-family: var(--font); outline: none; transition: border-color .15s;
}
.field input:focus, .field select:focus { border-color: var(--accent); }
.field select option { background: var(--surface2); }
.pw-wrap { position: relative; }
.pw-wrap input { padding-right: 40px; }
.pw-toggle { position: absolute; right: 10px; top: 50%; transform: translateY(-50%); background: none; border: none; cursor: pointer; color: var(--muted); font-size: 15px; padding: 2px; }
.pw-toggle:hover { color: var(--text); }
.key-set { font-size: 11px; color: var(--warn); margin-top: 4px; }
.row2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 8px 16px; border-radius: 6px; font-size: 13px; font-weight: 500; cursor: pointer; border: none; transition: opacity .15s; font-family: var(--font); }
.btn:hover { opacity: .85; }
.btn-primary { background: var(--accent); color: #0f1117; }
.btn-ghost { background: none; color: var(--muted); border: 1px solid var(--border); margin-top: 10px; }
.btn-rm { background: none; color: var(--danger); border: none; cursor: pointer; font-size: 18px; line-height: 1; padding: 4px 6px; align-self: flex-end; margin-bottom: 2px; }
.btn-rm:hover { color: #ff8a90; }
.org-card { background: var(--surface2); border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 12px; }
.org-header { display: flex; align-items: flex-end; gap: 10px; margin-bottom: 14px; }
.org-header .field { margin: 0; flex: 1; }
.tabs { display: flex; gap: 4px; margin-bottom: 14px; }
.tab { padding: 5px 12px; border-radius: 5px; font-size: 12px; font-weight: 500; cursor: pointer; border: 1px solid var(--border); background: none; color: var(--muted); transition: all .15s; font-family: var(--font); }
.tab.active { background: var(--accent); color: #0f1117; border-color: var(--accent); }
.envkey-row { display: grid; grid-template-columns: 160px 1fr auto; gap: 10px; align-items: flex-end; margin-bottom: 10px; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; background: var(--surface2); color: var(--muted); border: 1px solid var(--border); margin-right: 4px; }
.hint { font-size: 12px; color: var(--muted); margin-top: 4px; }
.footer { position: fixed; bottom: 0; left: 0; right: 0; padding: 16px 20px; background: var(--bg); border-top: 1px solid var(--border); }
.footer-inner { max-width: 720px; margin: 0 auto; display: flex; align-items: center; justify-content: space-between; }
.btn-save { padding: 11px 28px; font-size: 15px; }
.toast { position: fixed; bottom: 80px; left: 50%; transform: translateX(-50%); background: var(--accent); color: #0f1117; padding: 12px 24px; border-radius: 8px; font-weight: 600; font-size: 14px; opacity: 0; transition: opacity .3s; pointer-events: none; white-space: nowrap; }
.toast.show { opacity: 1; }
.empty-state { color: var(--muted); font-size: 13px; margin-bottom: 12px; }
</style>
</head>
<body>
<div class="container">
  <header>
    <h1>Syllable CLI Setup</h1>
    <p>Configures <code>~/.syllable/config.yaml</code> &nbsp;&middot;&nbsp; Existing keys are never sent to the browser &mdash; leave blank to keep them.</p>
  </header>

  <section>
    <h2>Environments</h2>
    <p class="hint" style="margin-bottom:16px">Builtins <span class="badge">prod</span><span class="badge">staging</span><span class="badge">dev</span> work without config. Add entries here only for custom URLs.</p>
    <div id="envs-list"></div>
    <button class="btn btn-ghost" onclick="addEnvRow()">+ Add environment</button>
  </section>

  <section>
    <h2>Organizations</h2>
    <div id="orgs-list"></div>
    <button class="btn btn-ghost" onclick="addOrg()">+ Add org</button>
  </section>

  <section>
    <h2>Defaults</h2>
    <div class="row2">
      <div class="field">
        <label>Default org</label>
        <select id="default-org"></select>
      </div>
      <div class="field">
        <label>Default environment</label>
        <input id="default-env" type="text" placeholder="prod" />
      </div>
    </div>
  </section>
</div>

<div class="footer">
  <div class="footer-inner" id="footer-inner">
    <span class="hint">Unsaved changes are lost if you close the tab.</span>
    <div style="display:flex;gap:10px">
      <button class="btn btn-ghost btn-save" onclick="save(false)">Save &amp; Continue</button>
      <button class="btn btn-primary btn-save" onclick="save(true)">Save &amp; Exit</button>
    </div>
  </div>
</div>
<div class="toast" id="toast"></div>

<script>
var EXISTING = '__existing__';
var state = %%INIT_CONFIG%%;
if (!state.environments) state.environments = {};
if (!state.orgs) state.orgs = {};
if (!state.default_env) state.default_env = 'prod';

Object.keys(state.orgs).forEach(function(n) {
  if (!state.orgs[n].envs) state.orgs[n].envs = {};
});

function render() { renderEnvs(); renderOrgs(); renderDefaults(); }

function mk(tag, cls) {
  var e = document.createElement(tag);
  if (cls) e.className = cls;
  return e;
}

function inp(type, value, placeholder) {
  var i = mk('input');
  i.type = type; i.value = value || ''; i.placeholder = placeholder || '';
  i.autocomplete = 'off';
  return i;
}

function labeled(labelText, child) {
  var wrap = mk('div', 'field');
  var lbl = mk('label'); lbl.textContent = labelText;
  wrap.appendChild(lbl); wrap.appendChild(child);
  return wrap;
}

function pwField(labelText, storedValue) {
  var hasExisting = storedValue === EXISTING;
  var i = inp('password', hasExisting ? '' : (storedValue || ''), hasExisting ? 'Leave blank to keep existing key' : 'Enter API key\u2026');

  var toggle = mk('button', 'pw-toggle'); toggle.type = 'button'; toggle.textContent = '\uD83D\uDC41';
  toggle.onclick = function() {
    i.type = i.type === 'password' ? 'text' : 'password';
    toggle.textContent = i.type === 'password' ? '\uD83D\uDC41' : '\uD83D\uDE48';
  };
  var wrap = mk('div', 'pw-wrap');
  wrap.appendChild(i); wrap.appendChild(toggle);

  var field = labeled(labelText, wrap);

  if (hasExisting) {
    var note = mk('div', 'key-set');
    note.textContent = '\u2713 Key configured \u2014 leave blank to keep it';
    field.appendChild(note);
  }

  i._getValue = function() {
    var v = i.value.trim();
    if (v === '') return hasExisting ? EXISTING : '';
    return v;
  };

  return field;
}

function rmBtn(onclick) {
  var b = mk('button', 'btn-rm'); b.title = 'Remove'; b.textContent = '\u00D7';
  b.onclick = onclick; return b;
}

function renderEnvs() {
  var list = document.getElementById('envs-list');
  list.innerHTML = '';
  var keys = Object.keys(state.environments);
  if (keys.length === 0) {
    var e = mk('p', 'empty-state'); e.textContent = 'No custom environments configured.';
    list.appendChild(e);
    return;
  }
  keys.forEach(function(name) {
    var env = state.environments[name];
    var saved = {name: name};

    var row = mk('div');
    row.style.cssText = 'display:grid;grid-template-columns:1fr 1fr auto;gap:10px;align-items:flex-end;margin-bottom:10px';

    var nameInp = inp('text', name, 'name');
    nameInp.addEventListener('change', function() {
      var old = saved.name; var n = nameInp.value.trim();
      if (!n || n === old) return;
      var v = state.environments[old]; delete state.environments[old];
      state.environments[n] = v; saved.name = n;
    });

    var urlInp = inp('text', env.base_url, 'https://\u2026');
    urlInp.addEventListener('change', function() {
      state.environments[saved.name] = {base_url: urlInp.value.trim()};
    });

    row.appendChild(labeled('Name', nameInp));
    row.appendChild(labeled('Base URL', urlInp));
    row.appendChild(rmBtn(function() { delete state.environments[saved.name]; renderEnvs(); }));
    list.appendChild(row);
  });
}

function addEnvRow() {
  var n = 'custom-' + (Object.keys(state.environments).length + 1);
  state.environments[n] = {base_url: ''}; renderEnvs();
}

function renderOrgs() {
  var list = document.getElementById('orgs-list');
  list.innerHTML = '';
  var keys = Object.keys(state.orgs);
  if (keys.length === 0) {
    var e = mk('p', 'empty-state'); e.textContent = 'No orgs configured yet.';
    list.appendChild(e);
  } else {
    keys.forEach(function(name) { list.appendChild(buildOrgCard(name)); });
  }
  renderDefaultOrgSelect();
}

function buildOrgCard(orgName) {
  var org = state.orgs[orgName];
  var saved = {name: orgName};
  var hasEnvKeys = org.envs && Object.keys(org.envs).length > 0;
  var mode = hasEnvKeys ? 'env' : 'single';

  var card = mk('div', 'org-card');

  var nameInp = inp('text', orgName, 'org-name');
  nameInp.addEventListener('change', function() {
    var old = saved.name; var n = nameInp.value.trim();
    if (!n || n === old) return;
    var v = state.orgs[old]; delete state.orgs[old];
    state.orgs[n] = v;
    if (state.default_org === old) state.default_org = n;
    saved.name = n; renderDefaultOrgSelect();
  });
  var header = mk('div', 'org-header');
  header.appendChild(labeled('Org name', nameInp));
  header.appendChild(rmBtn(function() { delete state.orgs[saved.name]; renderOrgs(); }));
  card.appendChild(header);

  var tabSingle = mk('button', 'tab' + (mode === 'single' ? ' active' : ''));
  tabSingle.textContent = 'Single API key';
  tabSingle.onclick = function() { setOrgMode(saved.name, 'single'); };

  var tabEnv = mk('button', 'tab' + (mode === 'env' ? ' active' : ''));
  tabEnv.textContent = 'Per-environment keys';
  tabEnv.onclick = function() { setOrgMode(saved.name, 'env'); };

  var tabs = mk('div', 'tabs');
  tabs.appendChild(tabSingle); tabs.appendChild(tabEnv);
  card.appendChild(tabs);

  var body = mk('div');
  if (mode === 'single') {
    var kf = pwField('API key', org.api_key || '');
    kf._keyInput = kf.querySelector('input');
    body.appendChild(kf);
    body._getKey = function() { return kf.querySelector('input')._getValue(); };
  } else {
    buildEnvKeyRows(body, saved);
  }
  body.id = 'org-body-' + orgName;
  card.appendChild(body);
  return card;
}

function buildEnvKeyRows(container, saved) {
  container.innerHTML = '';
  var org = state.orgs[saved.name];
  var envs = org.envs || {};

  Object.keys(envs).forEach(function(envName) {
    var envSaved = {name: envName};
    var row = mk('div', 'envkey-row');

    var envInp = inp('text', envName, 'environment');
    envInp.addEventListener('change', function() {
      var old = envSaved.name; var n = envInp.value.trim();
      if (!n || n === old) return;
      var v = state.orgs[saved.name].envs[old];
      delete state.orgs[saved.name].envs[old];
      state.orgs[saved.name].envs[n] = v; envSaved.name = n;
    });

    var kf = pwField('API key', envs[envName].api_key || '');
    var ki = kf.querySelector('input');

    row.appendChild(labeled('Environment', envInp));
    row.appendChild(kf);
    row.appendChild(rmBtn(function() {
      delete state.orgs[saved.name].envs[envSaved.name]; renderOrgs();
    }));
    container.appendChild(row);

    ki.addEventListener('change', function() {
      var v = ki._getValue ? ki._getValue() : ki.value.trim();
      if (!state.orgs[saved.name].envs[envSaved.name]) state.orgs[saved.name].envs[envSaved.name] = {};
      state.orgs[saved.name].envs[envSaved.name].api_key = v;
    });
  });

  var addBtn = mk('button', 'btn btn-ghost');
  addBtn.style.marginTop = '6px'; addBtn.textContent = '+ Add environment';
  addBtn.onclick = function() { addOrgEnv(saved.name); };
  container.appendChild(addBtn);
}

function setOrgMode(orgName, mode) {
  var org = state.orgs[orgName];
  if (mode === 'single') { org.envs = {}; }
  else {
    org.api_key = '';
    if (!org.envs || Object.keys(org.envs).length === 0) org.envs = {prod: {api_key: ''}};
  }
  renderOrgs();
}

function addOrg() {
  var n = 'new-org-' + (Object.keys(state.orgs).length + 1);
  state.orgs[n] = {api_key: '', envs: {}}; renderOrgs();
}

function addOrgEnv(orgName) {
  if (!state.orgs[orgName].envs) state.orgs[orgName].envs = {};
  var n = 'env-' + (Object.keys(state.orgs[orgName].envs).length + 1);
  state.orgs[orgName].envs[n] = {api_key: ''}; renderOrgs();
}

function renderDefaultOrgSelect() {
  var sel = document.getElementById('default-org');
  var prev = sel.value || state.default_org;
  sel.innerHTML = '<option value="">-- none --</option>';
  Object.keys(state.orgs).forEach(function(n) {
    var opt = document.createElement('option');
    opt.value = n; opt.textContent = n;
    if (n === prev) opt.selected = true;
    sel.appendChild(opt);
  });
}

function renderDefaults() {
  renderDefaultOrgSelect();
  document.getElementById('default-env').value = state.default_env || 'prod';
}

function collectState() {
  Object.keys(state.orgs).forEach(function(orgName) {
    var org = state.orgs[orgName];
    var hasEnvKeys = org.envs && Object.keys(org.envs).length > 0;
    if (!hasEnvKeys) {
      var bodyEl = document.getElementById('org-body-' + orgName);
      if (bodyEl) {
        var ki = bodyEl.querySelector('input[type=password], input[type=text]');
        if (ki && ki._getValue) { org.api_key = ki._getValue(); }
        else if (ki) { org.api_key = ki.value.trim() || (org.api_key === EXISTING ? EXISTING : ''); }
      }
    }
    state.orgs[orgName] = org;
  });
}

function save(exit) {
  collectState();
  state.default_org = document.getElementById('default-org').value;
  state.default_env = document.getElementById('default-env').value;

  var envsClean = {};
  Object.keys(state.environments).forEach(function(k) {
    if (k && state.environments[k].base_url) envsClean[k] = state.environments[k];
  });

  var payload = {
    default_org: state.default_org,
    default_env: state.default_env,
    environments: envsClean,
    orgs: state.orgs
  };

  fetch('/save?exit=' + (exit ? 'true' : 'false'), {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(payload)
  })
  .then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.error) { alert('Error: ' + data.error); return; }
    if (exit) {
      showToast('Saved \u2014 closing\u2026');
      setTimeout(function() { window.close(); }, 1200);
    } else {
      showToast('Saved to ' + data.path);
    }
  })
  .catch(function(e) { alert('Save failed: ' + e); });
}

function showToast(msg) {
  var t = document.getElementById('toast');
  t.textContent = msg; t.classList.add('show');
  setTimeout(function() { t.classList.remove('show'); }, 3000);
}

render();
</script>
</body>
</html>`
