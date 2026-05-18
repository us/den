package config

import (
	"reflect"
	"strings"
	"testing"
)

// secretFields is the denylist: S3Config fields whose literal value must NEVER
// appear in String(). If a new secret-bearing field is added to S3Config, add
// it here AND mask it in String(); TestS3ConfigStringMasksSecrets will fail
// loudly until both are done (the per-field-name assertion forces the new
// field to at least be acknowledged in String()).
var secretFields = map[string]bool{
	"AccessKey": true,
	"SecretKey": true,
}

// TestS3ConfigStringMasksSecrets is a reflection-driven invariant over every
// S3Config field:
//
//	(1) the field NAME must appear in String() — a new field cannot be added
//	    and silently omitted from the operator-facing dump;
//	(2) a secretFields literal must NOT appear — secrets are masked;
//	(3) a non-secret string literal MUST appear — masking is not over-broad,
//	    so the test cannot pass vacuously by masking everything.
func TestS3ConfigStringMasksSecrets(t *testing.T) {
	const (
		secretSentinel = "SUPERSECRETLITERAL_DO_NOT_LEAK"
		plainSentinel  = "PLAINVISIBLEVALUE"
	)

	typ := reflect.TypeOf(S3Config{})
	cfg := S3Config{AllowInternalEndpoint: true}
	v := reflect.ValueOf(&cfg).Elem()

	// Populate every string field: secrets get the secret sentinel, plain
	// strings a per-field visible value.
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type.Kind() != reflect.String {
			continue
		}
		if secretFields[f.Name] {
			v.Field(i).SetString(secretSentinel)
		} else {
			v.Field(i).SetString(plainSentinel + "_" + f.Name)
		}
	}

	got := cfg.String()

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)

		// (1) every field name is present in the dump.
		if !strings.Contains(got, f.Name) {
			t.Errorf("S3Config.String() omits field %q — output: %s", f.Name, got)
		}

		if f.Type.Kind() != reflect.String {
			continue
		}
		if secretFields[f.Name] {
			// (2) secret literal must be absent.
			if strings.Contains(got, secretSentinel) {
				t.Errorf("S3Config.String() LEAKS secret field %q literal — output: %s",
					f.Name, got)
			}
		} else {
			// (3) non-secret literal must be present.
			want := plainSentinel + "_" + f.Name
			if !strings.Contains(got, want) {
				t.Errorf("S3Config.String() drops non-secret field %q value %q — output: %s",
					f.Name, want, got)
			}
		}
	}

	// Defense-in-depth: the bare secret sentinel must not appear anywhere,
	// independent of the per-field walk above.
	if strings.Contains(got, secretSentinel) {
		t.Fatalf("S3Config.String() contains the raw secret sentinel: %s", got)
	}
}

// TestMaskSecret pins the redaction tokens: empty is operator-diagnosable and
// distinct from a present-but-masked secret.
func TestMaskSecret(t *testing.T) {
	if got := maskSecret(""); got != "(empty)" {
		t.Errorf("maskSecret(\"\") = %q, want \"(empty)\"", got)
	}
	if got := maskSecret("anything"); got != "***" {
		t.Errorf("maskSecret(\"anything\") = %q, want \"***\"", got)
	}
}
