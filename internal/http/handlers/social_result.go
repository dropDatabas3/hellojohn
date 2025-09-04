package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
)

type socialResultHandler struct {
	c *app.Container
}

func NewSocialResultHandler(c *app.Container) http.Handler {
	return &socialResultHandler{c: c}
}

func randB64_2(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (h *socialResultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1650)
		return
	}

	q := r.URL.Query()
	code := strings.TrimSpace(q.Get("code"))
	if code == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "falta code", 1651)
		return
	}
	peek := q.Get("peek") == "1" // modo prueba: no consumir

	key := "social:code:" + code

	// Obtener del cache
	payload, ok := h.c.Cache.Get(key)
	if !ok || len(payload) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "code_not_found", "código inválido o expirado", 1652)
		return
	}
	// Consumir solo si NO estamos en peek
	if !peek {
		h.c.Cache.Delete(key)
	}

	// ¿HTML o JSON?
	accept := strings.ToLower(r.Header.Get("Accept"))
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	// Priorizar JSON si el cliente lo pide explícitamente
	forceJSON := strings.Contains(accept, "application/json")
	// Solo HTML si el cliente lo pide, o si no envía Accept y parece un navegador
	wantsHTML := !forceJSON && (strings.Contains(accept, "text/html") || (accept == "" && strings.Contains(ua, "mozilla/")))

	// Intentamos decodificar (solo para vista; si falla no rompemos)
	var resp AuthLoginResponse
	_ = json.Unmarshal(payload, &resp)

	if wantsHTML {
		// CSP con nonce para permitir el CSS/JS inline de esta página
		nonce := randB64_2(16)
		csp := "default-src 'self'; " +
			"img-src 'self' data:; " +
			"style-src 'self' 'nonce-" + nonce + "'; " +
			"script-src 'self' 'nonce-" + nonce + "'; " +
			"connect-src 'self'; " +
			"base-uri 'self'; " +
			"frame-ancestors 'none'"

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Content-Security-Policy", csp)
		if peek {
			w.Header().Set("X-Debug-Note", "peek=1 (no consume code)")
		}

		const tpl = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1" />
  <meta name="color-scheme" content="light dark">
  <title>HelloJohn • Login</title>
  <style nonce="{{.Nonce}}">
    :root{
      --brand1:#10b6b6; /* turquesa */
      --brand2:#60a5fa; /* celeste */
      --bg:#f5f9fc;
      --card:#ffffff;
      --text:#0f172a;
      --muted:#64748b;
      --ok:#16a34a;
      --radius:16px;
      --shadow:0 10px 30px rgba(2,132,199,.15);
      --shadow-soft:0 6px 20px rgba(2,132,199,.1);
    }
    *{box-sizing:border-box}
    html,body{height:100%}
    body{
      margin:0;
      font-family: system-ui,-apple-system,Segoe UI,Roboto,Arial,sans-serif;
      color:var(--text);
      background:
        radial-gradient(60% 60% at 100% 0%, rgba(96,165,250,.25) 0%, transparent 60%),
        radial-gradient(50% 50% at 0% 100%, rgba(16,182,182,.25) 0%, transparent 60%),
        var(--bg);
      display:grid;
      place-items:center;
      padding:24px;
    }
    .card{
      width:min(720px, 95vw);
      background:var(--card);
      border-radius:var(--radius);
      box-shadow:var(--shadow);
      overflow:hidden;
      animation:pop .25s ease-out both;
    }
    @keyframes pop{from{transform:translateY(6px);opacity:.0}to{transform:none;opacity:1}}
    .brand{
      display:flex;
      align-items:center;
      gap:12px;
      padding:18px 20px;
      background:linear-gradient(120deg,var(--brand1),var(--brand2));
      color:#fff;
    }
    .logo{
      width:36px;height:36px;border-radius:10px;
      display:grid;place-items:center;
      background:rgba(255,255,255,.2);
      font-weight:700;
      box-shadow:var(--shadow-soft);
      letter-spacing:.5px;
      user-select:none;
    }
    .brand h1{
      margin:0;font-size:18px;font-weight:700;letter-spacing:.4px;
    }
    .content{padding:22px}
    .status{
      display:flex;align-items:center;gap:12px;margin-bottom:8px;
    }
    .status .ok{
      width:22px;height:22px;flex:0 0 22px;color:var(--ok)
    }
    .subtitle{color:var(--muted);margin:0 0 18px 0}
    .codebox{
      display:flex; align-items:center; gap:10px; background:#f7fbff; border:1px solid #dfeefd;
      padding:10px 12px; border-radius:12px; margin:12px 0;
    }
    .codebox code{
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size:13px; background:#eef6ff; padding:4px 6px; border-radius:6px;
    }
    details{
      border:1px solid #e5eff8;border-radius:12px;overflow:hidden;
      background:#fbfdff;
    }
    details>summary{
      cursor:pointer;list-style:none;padding:14px 16px;font-weight:600;
      background:linear-gradient(180deg,#ffffff, #f7fbff);
      outline:none;
    }
    details[open]>summary{border-bottom:1px solid #eaf2fb}
    pre{
      margin:0;padding:16px 18px;
      white-space:pre-wrap;word-break:break-word;
      background:#0b1220;color:#e5e7eb;
      max-height:45vh;overflow:auto;font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace;
      line-height:1.4;border-bottom-left-radius:12px;border-bottom-right-radius:12px;
    }
    .actions{
      display:flex;gap:10px;flex-wrap:wrap;justify-content:flex-end;margin-top:18px
    }
    button, .btn{
      appearance:none;border:0;border-radius:10px;padding:10px 14px;font-weight:600;
      cursor:pointer;transition:transform .05s ease, box-shadow .2s ease;
    }
    .btn-primary{
      background:linear-gradient(120deg,var(--brand1),var(--brand2));
      color:#fff; box-shadow:var(--shadow-soft);
    }
    .btn-secondary{
      background:#eef6ff;color:#0b4b7d;border:1px solid #d7e8fb
    }
    button:active{transform:translateY(1px)}
    .hint{color:var(--muted);font-size:13px;margin-top:10px}
    footer{
      padding:14px 20px;color:#7b8aa0;font-size:12px;background:#f7fbff;border-top:1px solid #eaf2fb;
      display:flex;justify-content:space-between;align-items:center;gap:12px
    }
    .badge{
      font-size:11px;padding:4px 8px;border-radius:999px;background:#e6f7f7;color:#0f766e;border:1px solid #b6ecec
    }
    .grow{flex:1}
    .sr-only{position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0}
  </style>
</head>
<body>
  <div class="card" role="region" aria-label="Estado de inicio de sesión">
    <header class="brand">
      <div class="logo" aria-hidden="true">HJ</div>
      <h1>HelloJohn</h1>
      <span class="grow"></span>
      <span class="badge">Login</span>
    </header>

    <section class="content">
      <div class="status">
        <svg class="ok" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="1.5" opacity=".25"/>
          <path d="M7 12.5l3.2 3.2L17 9" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <h2 style="margin:0;font-size:20px;">¡Inicio de sesión exitoso!</h2>
      </div>
      <p class="subtitle">Tu sesión fue creada correctamente.</p>

      <div class="codebox">
        <strong>Código de login:</strong>
        <code id="codeVal">{{.Code}}</code>
        <button class="btn-secondary" id="copyCodeBtn" type="button">Copiar</button>
      </div>
      {{if .Peek}}<p class="hint">Modo prueba activo (<code>peek=1</code>): el código NO se consumió aún.</p>{{end}}

      <details>
        <summary>Ver respuesta (tokens JSON)</summary>
        <pre id="jsonView">Cargando…</pre>
      </details>

      <div class="actions">
        <button class="btn-secondary" id="copyBtn" type="button">Copiar JSON</button>
        <button class="btn-primary" id="closeBtn" type="button">Cerrar ventana</button>
      </div>
      <p class="hint">Si esta pantalla se abrió en un popup, notificaremos al sitio que te inició sesión.</p>
    </section>

    <footer>
      <span>© {{.Year}} HelloJohn</span>
      <span class="badge">Demo</span>
    </footer>
  </div>

  <!-- Payload base64 (lo inyecta el servidor) -->
  <script type="application/octet-stream" id="payload-b64" nonce="{{.Nonce}}">{{.PayloadB64}}</script>

  <script nonce="{{.Nonce}}">
    (function () {
      const pre = document.getElementById('jsonView');
      const b64 = (document.getElementById('payload-b64')?.textContent || '').trim();

      // Decodificar base64 y pretty print
      let raw = '';
      try { raw = b64 ? atob(b64) : ''; } catch {}
      try {
        const obj = JSON.parse(raw);
        pre.textContent = JSON.stringify(obj, null, 2);
      } catch (e) {
        pre.textContent = raw || 'Sin contenido';
      }

      // Utilidades de navegación
      const urlNow = new URL(window.location.href);
      const returnToParam = urlNow.searchParams.get('return_to');
      let returnTo = '/';
      try {
        if (returnToParam) {
          const rt = new URL(returnToParam, window.location.origin);
          if (rt.origin === window.location.origin) returnTo = rt.toString();
        } else if (document.referrer) {
          const ref = new URL(document.referrer);
          if (ref.origin === window.location.origin) returnTo = ref.toString();
        }
      } catch {}

      const isPopup = !!(window.opener && window.opener !== window && !window.opener.closed);

      // Ajustar texto del botón según contexto
      const closeBtn = document.getElementById('closeBtn');
      if (closeBtn) closeBtn.textContent = isPopup ? 'Cerrar ventana' : 'Volver';

      // Copiar JSON
      document.getElementById('copyBtn')?.addEventListener('click', async (event) => {
        try {
          await navigator.clipboard.writeText(pre.textContent);
          const old = event.target.textContent;
          event.target.textContent = '¡Copiado!';
          setTimeout(() => event.target.textContent = old, 1200);
        } catch { alert('No se pudo copiar'); }
      });

      // Copiar CODE
      document.getElementById('copyCodeBtn')?.addEventListener('click', async (event) => {
        try {
          const txt = document.getElementById('codeVal')?.textContent || '';
          await navigator.clipboard.writeText(txt);
          const old = event.target.textContent;
          event.target.textContent = '¡Copiado!';
          setTimeout(() => event.target.textContent = old, 1200);
        } catch { alert('No se pudo copiar'); }
      });

      // Cerrar o volver / redirigir
      closeBtn?.addEventListener('click', () => {
        if (isPopup) {
          window.close();
          try { window.open('', '_self'); } catch {}
          setTimeout(() => { try { window.close(); } catch {} }, 10);
        } else {
          if (history.length > 1) {
            history.back();
          } else {
            location.replace(returnTo);
          }
        }
      });

      // Comunicar al opener (si existe)
      try {
        const data = raw ? JSON.parse(raw) : null;
        if (isPopup && data) {
          window.opener.postMessage({ type: 'hellojohn:login_result', payload: data }, '*');
        }
      } catch {}

      // Autocerrar si viene ?autoclose=1 y es popup
      try {
        if (urlNow.searchParams.get('autoclose') === '1' && isPopup) {
          setTimeout(() => closeBtn?.click(), 800);
        }
      } catch {}
    })();
  </script>
</body>
</html>`

		// Datos para la vista (JSON en base64, sin entidades HTML)
		data := struct {
			Nonce      string
			Year       int
			PayloadB64 string
			Code       string
			Peek       bool
		}{
			Nonce:      nonce,
			Year:       time.Now().Year(),
			PayloadB64: base64.StdEncoding.EncodeToString(payload),
			Code:       code,
			Peek:       peek,
		}

		t := template.Must(template.New("ok").Parse(tpl))
		_ = t.Execute(w, data)
		return
	}

	// Por defecto: JSON
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(payload)
}
