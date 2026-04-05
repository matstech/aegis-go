package protocol

import "testing"

func TestXXH64HexMatchesServerVector(t *testing.T) {
	t.Parallel()

	payload := []byte("DuqjbeoyE9LIo77MaATfF0zl3hu2BZ31")

	if got, want := XXH64Hex(payload), "890c5c788922811a"; got != want {
		t.Fatalf("XXH64Hex() = %q, want %q", got, want)
	}
}

func TestCanonicalStringWithSignedHeadersAndBody(t *testing.T) {
	t.Parallel()

	got, err := CanonicalString("1fkEphx2qq", []string{"header1", "header2"}, []byte("DuqjbeoyE9LIo77MaATfF0zl3hu2BZ31"))
	if err != nil {
		t.Fatalf("CanonicalString() error = %v", err)
	}

	want := "1fkEphx2qq;header1;header2:890c5c788922811a"
	if got != want {
		t.Fatalf("CanonicalString() = %q, want %q", got, want)
	}
}

func TestCanonicalStringWithoutBody(t *testing.T) {
	t.Parallel()

	got, err := CanonicalString("1fkEphx2qq", []string{"header1", "header2"}, nil)
	if err != nil {
		t.Fatalf("CanonicalString() error = %v", err)
	}

	want := "1fkEphx2qq;header1;header2"
	if got != want {
		t.Fatalf("CanonicalString() = %q, want %q", got, want)
	}
}

func TestCanonicalStringRequiresCorrelationID(t *testing.T) {
	t.Parallel()

	if _, err := CanonicalString("", nil, nil); err == nil {
		t.Fatal("CanonicalString() error = nil, want non-nil")
	}
}

func TestSignMatchesServerVectorWithBody(t *testing.T) {
	t.Parallel()

	canonical := "1fkEphx2qq;header1;header2:890c5c788922811a"
	if got, want := Sign("QTEiL2Jy92", canonical), "XciMlTpNQSefPAjCbHzHU6fF3YorGGOMyP8qMuYKCOc3Z1MD5iSb9dgUyvg6arCRd/Bz4/EfJRO00HXLZLX1Dw=="; got != want {
		t.Fatalf("Sign() = %q, want %q", got, want)
	}
}

func TestSignMatchesServerVectorWithoutBody(t *testing.T) {
	t.Parallel()

	canonical := "1fkEphx2qq;header1;header2"
	if got, want := Sign("QTEiL2Jy92", canonical), "r2ncXWTsILhGhaDByZUFRrUPxT3nz1pw9qeXd2TdRizH75qq6m5UFoDa31CapZIp2TyTKTs3v6TqZr+8qYdHGQ=="; got != want {
		t.Fatalf("Sign() = %q, want %q", got, want)
	}
}
