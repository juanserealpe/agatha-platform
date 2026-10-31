package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const (
	allowedOrigin   = "http://localhost:5173"
	forbiddenOrigin = "https://evil.example"
)

func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "vacía deshabilita CORS", raw: "  ", want: nil},
		{name: "un origen", raw: "http://localhost:5173", want: []string{"http://localhost:5173"}},
		{
			name: "varios orígenes con espacios, vacíos y repetidos",
			raw:  " http://localhost:5173 ,, https://Agatha-Front.vercel.app/ ,http://localhost:5173",
			want: []string{"http://localhost:5173", "https://agatha-front.vercel.app"},
		},
		{name: "comodín rechazado", raw: "http://localhost:5173,*", wantErr: true},
		{name: "sin esquema", raw: "localhost:5173", wantErr: true},
		{name: "esquema no http", raw: "ftp://localhost", wantErr: true},
		{name: "con ruta", raw: "https://front.example/app", wantErr: true},
		{name: "con query", raw: "https://front.example?x=1", wantErr: true},
		{name: "sin host", raw: "https://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAllowedOrigins(tt.raw)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidCORSOrigin) {
					t.Fatalf("error = %v, se esperaba ErrInvalidCORSOrigin", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("orígenes = %q, se esperaba %q", got, tt.want)
			}
		})
	}
}

// okHandler simula un endpoint: marca si se ejecutó y responde JSON.
func okHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func preflight(path, origin, method, headers string) *http.Request {
	req := httptest.NewRequest(http.MethodOptions, path, nil)
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", method)
	if headers != "" {
		req.Header.Set("Access-Control-Request-Headers", headers)
	}
	return req
}

func TestCORS_OrigenAutorizado(t *testing.T) {
	var called bool
	h := CORS([]string{allowedOrigin}, okHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/devices", nil)
	req.Header.Set("Origin", allowedOrigin)
	rec := serve(h, req)

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("handler llamado = %v, status = %d; se esperaba llamado y 200", called, rec.Code)
	}
	if got := rec.Header().Values("Access-Control-Allow-Origin"); !reflect.DeepEqual(got, []string{allowedOrigin}) {
		t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba exactamente [%q]", got, allowedOrigin)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, se esperaba Origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("no se deben habilitar credenciales; Access-Control-Allow-Credentials = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type del handler = %q, se esperaba application/json", got)
	}
}

func TestCORS_OrigenNoAutorizado(t *testing.T) {
	var called bool
	h := CORS([]string{allowedOrigin}, okHandler(&called))

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/devices", nil)
	req.Header.Set("Origin", forbiddenOrigin)
	rec := serve(h, req)

	// El servidor responde igual; es el navegador el que bloquea la lectura
	// porque falta Access-Control-Allow-Origin.
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("handler llamado = %v, status = %d; se esperaba llamado y 200", called, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba vacío", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, se esperaba Origin (la respuesta depende del origen)", got)
	}
}

func TestCORS_SinOrigin(t *testing.T) {
	var called bool
	h := CORS([]string{allowedOrigin}, okHandler(&called))

	rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/dashboard/devices", nil))

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("handler llamado = %v, status = %d; se esperaba llamado y 200", called, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba vacío", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	t.Run("autorizado responde 204 sin ejecutar el handler", func(t *testing.T) {
		var called bool
		h := CORS([]string{allowedOrigin}, okHandler(&called))

		rec := serve(h, preflight("/api/dashboard/series", allowedOrigin, http.MethodGet, "content-type, Accept"))

		if called {
			t.Fatal("el preflight no debe llegar al handler")
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, se esperaba 204", rec.Code)
		}
		want := map[string]string{
			"Access-Control-Allow-Origin":  allowedOrigin,
			"Access-Control-Allow-Methods": "GET, POST, PUT",
			"Access-Control-Allow-Headers": "Accept, Content-Type, Last-Event-ID",
			"Access-Control-Max-Age":       "600",
		}
		for header, value := range want {
			if got := rec.Header().Values(header); !reflect.DeepEqual(got, []string{value}) {
				t.Fatalf("%s = %q, se esperaba exactamente [%q]", header, got, value)
			}
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Fatalf("Access-Control-Allow-Credentials = %q, se esperaba vacío", got)
		}
	})

	rejected := []struct {
		name, origin, method, headers string
	}{
		{name: "origen no autorizado", origin: forbiddenOrigin, method: http.MethodGet},
		{name: "método DELETE no permitido", origin: allowedOrigin, method: http.MethodDelete},
		{name: "método PATCH no permitido", origin: allowedOrigin, method: http.MethodPatch},
		{name: "Authorization no permitido en POST", origin: allowedOrigin, method: http.MethodPost, headers: "authorization, content-type"},
		{name: "encabezado no permitido", origin: allowedOrigin, method: http.MethodGet, headers: "Authorization"},
	}
	for _, tt := range rejected {
		t.Run(tt.name+" responde 403 sin encabezados CORS", func(t *testing.T) {
			var called bool
			h := CORS([]string{allowedOrigin}, okHandler(&called))

			rec := serve(h, preflight("/api/dashboard/devices", tt.origin, tt.method, tt.headers))

			if called {
				t.Fatal("el preflight rechazado no debe llegar al handler")
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, se esperaba 403", rec.Code)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba vacío", got)
			}
		})
	}

	// Lo que usa el frontend: gestión de alertas (POST con JSON), registro de
	// tokens (PUT) y la reconexión nativa del stream SSE (Last-Event-ID).
	allowed := []struct {
		name, path, method, headers string
	}{
		{name: "POST con JSON (clasificar una alerta)", path: "/api/alerts/123/classification", method: http.MethodPost, headers: "content-type"},
		{name: "PUT con JSON (token de push)", path: "/api/devices/push-token", method: http.MethodPut, headers: "Content-Type"},
		{name: "GET con Last-Event-ID (reconexión del stream)", path: "/api/notifications/stream", method: http.MethodGet, headers: "last-event-id"},
	}
	for _, tt := range allowed {
		t.Run(tt.name+" responde 204", func(t *testing.T) {
			var called bool
			h := CORS([]string{allowedOrigin}, okHandler(&called))

			rec := serve(h, preflight(tt.path, allowedOrigin, tt.method, tt.headers))

			if called || rec.Code != http.StatusNoContent {
				t.Fatalf("handler llamado = %v, status = %d; se esperaba 204 sin llegar al handler", called, rec.Code)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
				t.Fatalf("Access-Control-Allow-Origin = %q", got)
			}
		})
	}

	t.Run("POST real de un origen autorizado llega al handler con CORS", func(t *testing.T) {
		var called bool
		h := CORS([]string{allowedOrigin}, okHandler(&called))

		req := httptest.NewRequest(http.MethodPost, "/api/alerts/123/classification", nil)
		req.Header.Set("Origin", allowedOrigin)
		rec := serve(h, req)

		if !called || rec.Header().Get("Access-Control-Allow-Origin") != allowedOrigin {
			t.Fatalf("handler llamado = %v, Access-Control-Allow-Origin = %q", called, rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("OPTIONS que no es preflight pasa al handler", func(t *testing.T) {
		var called bool
		h := CORS([]string{allowedOrigin}, okHandler(&called))

		req := httptest.NewRequest(http.MethodOptions, "/api/dashboard/devices", nil)
		req.Header.Set("Origin", allowedOrigin)
		serve(h, req)

		if !called {
			t.Fatal("un OPTIONS sin Access-Control-Request-Method debe llegar al handler")
		}
	})
}

func TestCORS_SinOrigenesConfigurados(t *testing.T) {
	var called bool
	next := okHandler(&called)

	rec := serve(CORS(nil, next), preflight("/api/dashboard/devices", allowedOrigin, http.MethodGet, ""))

	if !called {
		t.Fatal("sin orígenes configurados el middleware no debe interceptar nada")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba vacío", got)
	}
}

// TestCORS_CompatibleConServeMux arma un mux con los mismos patrones que
// registran Dashboard (con método) y Callback (sin método).
func TestCORS_CompatibleConServeMux(t *testing.T) {
	var dashboardCalled, callbackMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/dashboard/devices", func(w http.ResponseWriter, r *http.Request) {
		dashboardCalled = r.Method
		WriteJSON(w, http.StatusOK, []string{})
	})
	mux.HandleFunc("/ingestion/callback", func(w http.ResponseWriter, r *http.Request) {
		callbackMethod = r.Method
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"code": "METHOD_NOT_ALLOWED"})
			return
		}
		WriteJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
	})
	h := CORS([]string{allowedOrigin}, mux)

	t.Run("GET de Dashboard conserva respuesta y agrega CORS", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/dashboard/devices", nil)
		req.Header.Set("Origin", allowedOrigin)
		rec := serve(h, req)

		if dashboardCalled != http.MethodGet || rec.Code != http.StatusOK {
			t.Fatalf("handler = %q, status = %d; se esperaba GET y 200", dashboardCalled, rec.Code)
		}
		if got := rec.Header().Values("Access-Control-Allow-Origin"); len(got) != 1 {
			t.Fatalf("Access-Control-Allow-Origin duplicado o ausente: %q", got)
		}
	})

	t.Run("preflight a ruta GET devuelve 204 en vez del 405 del mux", func(t *testing.T) {
		rec := serve(h, preflight("/api/dashboard/devices", allowedOrigin, http.MethodGet, ""))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, se esperaba 204", rec.Code)
		}
	})

	t.Run("método no registrado sigue respondiendo 405 del mux", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/dashboard/devices", nil)
		req.Header.Set("Origin", allowedOrigin)
		rec := serve(h, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, se esperaba 405", rec.Code)
		}
	})

	t.Run("callback servidor a servidor no cambia", func(t *testing.T) {
		rec := serve(h, httptest.NewRequest(http.MethodPost, "/ingestion/callback", nil))
		if callbackMethod != http.MethodPost || rec.Code != http.StatusAccepted {
			t.Fatalf("handler = %q, status = %d; se esperaba POST y 202", callbackMethod, rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("Access-Control-Allow-Origin = %q, se esperaba vacío sin Origin", got)
		}
	})

	t.Run("preflight POST al callback desde el navegador se rechaza", func(t *testing.T) {
		callbackMethod = ""
		rec := serve(h, preflight("/ingestion/callback", allowedOrigin, http.MethodPost, "authorization, content-type"))
		if rec.Code != http.StatusForbidden || callbackMethod != "" {
			t.Fatalf("status = %d, handler = %q; se esperaba 403 sin llegar al handler", rec.Code, callbackMethod)
		}
	})
}
