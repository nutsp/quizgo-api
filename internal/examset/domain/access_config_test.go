package domain

import "testing"

func TestValidateAccessConfig(t *testing.T) {
	sale := 99.0
	tests := []struct {
		name                string
		accessType          string
		price               float64
		sale                *float64
		allowSinglePurchase bool
		wantErr             bool
	}{
		{"free valid", AccessFree, 0, nil, false, false},
		{"free invalid price", AccessFree, 10, nil, false, true},
		{"free invalid allow single", AccessFree, 0, nil, true, true},
		{"paid valid", AccessPaid, 49, nil, true, false},
		{"paid invalid price", AccessPaid, 0, nil, true, true},
		{"paid invalid no single", AccessPaid, 49, nil, false, true},
		{"premium valid", AccessPremium, 0, nil, false, false},
		{"premium single valid", AccessPremium, 99, nil, true, false},
		{"premium single invalid price", AccessPremium, 0, nil, true, true},
		{"private valid", AccessPrivate, 0, nil, false, false},
		{"private invalid price", AccessPrivate, 10, nil, false, true},
		{"private invalid sale", AccessPrivate, 0, &sale, false, true},
		{"private invalid allow single", AccessPrivate, 0, nil, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAccessConfig(tt.accessType, tt.price, tt.sale, tt.allowSinglePurchase)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateAccessConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeAccessConfig(t *testing.T) {
	price, allow := NormalizeAccessConfig(AccessPaid, 99, false)
	if price != 99 || !allow {
		t.Fatalf("paid should force allow_single_purchase=true, got price=%v allow=%v", price, allow)
	}
	price, allow = NormalizeAccessConfig(AccessFree, 10, true)
	if price != 0 || allow {
		t.Fatalf("free should reset price and allow, got price=%v allow=%v", price, allow)
	}
}

func TestIsPublicDiscoveryAccessType(t *testing.T) {
	if IsPublicDiscoveryAccessType(AccessPrivate) {
		t.Fatal("private must not be public discovery")
	}
	if !IsPublicDiscoveryAccessType(AccessFree) {
		t.Fatal("free must be public discovery")
	}
}
