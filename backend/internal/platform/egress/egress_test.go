package egress

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckURL(t *testing.T) {
	ok := map[string]string{
		"https://api.openai.com/v1/":       "https://api.openai.com/v1",
		"  https://cloud.langfuse.com  ":   "https://cloud.langfuse.com",
		"https://my.openai.azure.com:8443": "https://my.openai.azure.com:8443",
	}
	for in, want := range ok {
		got, err := CheckURL(in, false)
		if err != nil || got != want {
			t.Errorf("CheckURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "api.openai.com", "ftp://x.com", "http://api.openai.com", "https://user:pw@x.com", "https://", "https://x.com/?q=1", "https://x.com/#f"} {
		if _, err := CheckURL(bad, false); err == nil {
			t.Errorf("CheckURL(%q) accepted", bad)
		}
	}
	if got, err := CheckURL("http://localhost:3000", true); err != nil || got != "http://localhost:3000" {
		t.Errorf("http allowed with private network: %q, %v", got, err)
	}
}

func TestClientBlocksPrivateAddressesUnlessAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	_, err := NewClient(false, time.Second).Get(srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("loopback request err = %v, want ErrBlocked", err)
	}

	res, err := NewClient(true, time.Second).Get(srv.URL)
	if err != nil {
		t.Fatalf("allowed client: %v", err)
	}
	res.Body.Close()
}

func TestIsPublic(t *testing.T) {
	for ip, want := range map[string]bool{
		"8.8.8.8": true, "127.0.0.1": false, "10.1.2.3": false, "192.168.1.1": false,
		"172.16.0.1": false, "169.254.169.254": false, "0.0.0.0": false, "100.64.0.1": false,
		"::1": false, "fd00::1": false, "fe80::1": false, "2606:4700::1111": true,
		"::ffff:127.0.0.1": false,
	} {
		if got := isPublic(ip); got != want {
			t.Errorf("isPublic(%s) = %v, want %v", ip, got, want)
		}
	}
}

func TestIsPublicReservedRanges(t *testing.T) {
	for _, ip := range []string{"64:ff9b::7f00:1", "::127.0.0.1", "fec0::1", "0.1.2.3", "198.18.0.1", "192.0.0.8", "240.0.0.1", "255.255.255.255"} {
		if isPublic(ip) {
			t.Errorf("isPublic(%s) = true, want false", ip)
		}
	}
}
