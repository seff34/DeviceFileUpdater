package web

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

const sessionCookie = "devupdater_session"

func allowedHost(h string) bool {
	host, _, err := net.SplitHostPort(h)
	if err != nil {
		host = h
	}
	return host == "127.0.0.1" || host == "localhost"
}

func (s *Server) authed(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1
}

// guard enforces:
//   - a loopback Host header (defeats DNS rebinding)
//   - a same-origin Origin on writes
//   - the token exchange: /?token=T sets the session cookie and redirects to /
//   - the session cookie on /api/
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHost(r.Host) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
				http.Error(w, "cross-origin request refused", http.StatusForbidden)
				return
			}
		}
		if r.URL.Path == "/" && r.URL.Query().Has("token") {
			if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(s.token)) != 1 {
				http.Error(w, "Geçersiz erişim anahtarı. Uygulamayı yeniden başlatın.", http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name: sessionCookie, Value: s.token, Path: "/",
				HttpOnly: true, SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") && !s.authed(r) {
			writeError(w, http.StatusUnauthorized, "Oturum bulunamadı. Uygulamayı 'devupdater ui' ile yeniden başlatın.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
