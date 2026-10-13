package httpclient

import (
	"net/http"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	body := `{"name":"c","spec":{"password":"hunter2","apiKey":"k-123","token_ref":"account.tok","privateKeyRef":"account.key","secret_manager_identifier":"sm","delegate_token":"abc","client_secret":"s3","password_wo":"pw"},"literal":"sk-live-999"}`
	got := Redact(body, []string{"sk-live-999"})
	for _, want := range []string{`"password":"***"`, `"apiKey":"***"`, `"delegate_token":"***"`, `"client_secret":"***"`, `"password_wo":"***"`, `"literal":"***"`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %s in %s", want, got)
		}
	}
	for _, keep := range []string{`"token_ref":"account.tok"`, `"privateKeyRef":"account.key"`, `"secret_manager_identifier":"sm"`, `"name":"c"`} {
		if !strings.Contains(got, keep) {
			t.Errorf("reference/identifier %s must not be redacted: %s", keep, got)
		}
	}
	if Redact("plain text without secrets", nil) != "plain text without secrets" {
		t.Error("text without secrets must be unchanged")
	}
}

func TestRedactHeadersAndURL(t *testing.T) {
	h := http.Header{}
	h.Set("X-Api-Key", "pat.abc")
	h.Set("Authorization", "Bearer tok")
	h.Set("Content-Type", "application/json")
	h.Set("X-Correlation", "value-with-pat.abc")
	got := RedactHeaders(h, []string{"pat.abc"})
	if got["X-Api-Key"] != "***" || got["Authorization"] != "***" || got["Content-Type"] != "application/json" || got["X-Correlation"] != "value-with-***" {
		t.Errorf("headers: %v", got)
	}
	u := RedactURL("https://app.harness.io/gateway/x?accountIdentifier=acct&apiKey=pat.abc&token=t", []string{"pat.abc"})
	if u != "https://app.harness.io/gateway/x?accountIdentifier=acct&apiKey=***&token=***" {
		t.Errorf("url: %s", u)
	}
}
