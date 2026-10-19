package main

import (
	"html/template"
	"net/http"
	"net/url"
)

// The mock "approve in your bank" page that scaRedirect points to.
var scaTmpl = template.Must(template.New("sca").Parse(`<!doctype html>
<html lang="az">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Bank}} · Təsdiq</title>
<style>
  :root { --bg:#f3f5f8; --card:#fff; --fg:#14213d; --muted:#5c6b80; --accent:#0b5cad; --line:#dde3ea; --danger:#b42318 }
  * { box-sizing:border-box }
  body { margin:0; font:16px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif; background:var(--bg); color:var(--fg);
         padding:24px max(16px, env(safe-area-inset-left)) }
  main { max-width:420px; margin:0 auto; background:var(--card); border:1px solid var(--line); border-radius:16px; padding:24px }
  .bank { font-weight:700; font-size:20px; color:var(--accent) }
  .tag { display:inline-block; font-size:12px; letter-spacing:.04em; text-transform:uppercase; color:var(--muted); border:1px solid var(--line); border-radius:999px; padding:2px 10px; margin-top:6px }
  h1 { font-size:18px; margin:20px 0 8px }
  ul { padding-left:20px; margin:8px 0 16px; color:var(--muted) }
  dl { display:grid; grid-template-columns:auto 1fr; gap:4px 12px; font-size:14px; margin:0 0 20px }
  dt { color:var(--muted) }
  .actions { display:grid; gap:10px }
  button { font:inherit; font-weight:600; padding:14px; border-radius:12px; border:0; cursor:pointer }
  .approve { background:var(--accent); color:#fff }
  .reject { background:transparent; color:var(--danger); border:1px solid var(--line) }
  button:focus-visible { outline:3px solid var(--fg); outline-offset:2px }
  .note { font-size:12px; color:var(--muted); margin-top:16px; text-align:center }
</style>
</head>
<body>
<main>
  <div class="bank">{{.Bank}}</div>
  <span class="tag">Mock bank · demo</span>
  {{if .Done}}
    <h1>Bu sorğu artıq cavablandırılıb</h1>
    <p>Status: <strong>{{.Status}}</strong></p>
  {{else}}
    <h1>Cibim hesab məlumatlarınıza giriş istəyir</h1>
    <p>Salam, {{.User}}. Aşağıdakı məlumatlar yalnız oxumaq üçün paylaşılacaq:</p>
    <ul>
      <li>Hesab və kartların siyahısı</li>
      <li>Balanslar</li>
      <li>Əməliyyat tarixçəsi (son 90 gün)</li>
    </ul>
    <dl>
      <dt>Etibarlıdır</dt><dd>{{.ValidUntil}} tarixinədək</dd>
      <dt>Razılıq ID</dt><dd>{{.ID}}</dd>
    </dl>
    <form method="post" class="actions">
      <button class="approve" name="decision" value="approve">Təsdiqlə (Face ID)</button>
      <button class="reject" name="decision" value="reject">İmtina et</button>
    </form>
    <p class="note">Cibim ödəniş edə və pul köçürə bilməz. Razılığı istənilən an geri götürə bilərsiniz.</p>
  {{end}}
</main>
</body>
</html>`))

type scaView struct {
	Bank, User, ID, ValidUntil, Status string
	Done                               bool
}

func (srv *Server) scaPage(w http.ResponseWriter, r *http.Request) {
	s := srv.store
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.consents[r.PathValue("consentId")]
	if !ok {
		http.Error(w, "Unknown consent", http.StatusNotFound)
		return
	}
	s.refreshStatus(c)
	srv.renderSCA(w, c)
}

func (srv *Server) renderSCA(w http.ResponseWriter, c *Consent) {
	bank, _ := srv.store.bank(c.BankCode)
	user, _ := srv.store.user(c.PSUID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = scaTmpl.Execute(w, scaView{
		Bank: bank.Name, User: user.Name, ID: c.ID, ValidUntil: c.ValidUntil.Format(dateLayout),
		Status: c.Status, Done: c.Status != StatusReceived,
	})
}

// scaDecision handles the page's buttons, then sends the user back to the app.
func (srv *Server) scaDecision(w http.ResponseWriter, r *http.Request) {
	s := srv.store
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.consents[r.PathValue("consentId")]
	if !ok {
		http.Error(w, "Unknown consent", http.StatusNotFound)
		return
	}
	s.refreshStatus(c)
	if c.Status != StatusReceived {
		srv.renderSCA(w, c)
		return
	}
	if r.FormValue("decision") == "approve" {
		c.Status = StatusValid
	} else {
		c.Status = StatusRejected
	}

	u, err := url.Parse(c.RedirectURI)
	if err != nil {
		srv.renderSCA(w, c)
		return
	}
	q := u.Query()
	q.Set("consentId", c.ID)
	q.Set("status", c.Status)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}
