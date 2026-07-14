package domain

import "testing"

func TestValidateAccessConfig(t *testing.T) {
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
		{"premium valid", AccessPremium, 0, nil, false, false},
		{"premium single valid", AccessPremium, 99, nil, true, false},
		{"premium single invalid price", AccessPremium, 0, nil, true, true},
		{"trial no longer accepted", AccessTrial, 0, nil, false, true},
		{"paid no longer accepted", AccessPaid, 49, nil, true, true},
		{"private no longer accepted", AccessPrivate, 0, nil, false, true},
		{"unknown invalid", "member", 0, nil, false, true},
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
	price, allow := NormalizeAccessConfig(AccessPremium, 99, true)
	if price != 99 || !allow {
		t.Fatalf("premium single purchase should preserve price and allow, got price=%v allow=%v", price, allow)
	}
	price, allow = NormalizeAccessConfig(AccessFree, 10, true)
	if price != 0 || allow {
		t.Fatalf("free should reset price and allow, got price=%v allow=%v", price, allow)
	}
	price, allow = NormalizeAccessConfig(AccessPremium, 99, false)
	if price != 0 || allow {
		t.Fatalf("premium without single purchase should reset price and allow, got price=%v allow=%v", price, allow)
	}
}

func TestIsPublicDiscoveryAccessType(t *testing.T) {
	if IsPublicDiscoveryAccessType(AccessPrivate) {
		t.Fatal("private must not be public discovery")
	}
	if !IsPublicDiscoveryAccessType(AccessFree) {
		t.Fatal("free must be public discovery")
	}
	if !IsPublicDiscoveryAccessType(AccessPremium) {
		t.Fatal("premium must be public discovery")
	}
}
