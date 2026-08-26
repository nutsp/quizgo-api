package catalog

import "testing"

func TestLoadDefaultPackages(t *testing.T) {
	packages, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}

	if len(packages) != 5 {
		t.Fatalf("len(packages) = %d, want 5", len(packages))
	}

	threeMonth, ok := Find(packages, "premium-3m")
	if !ok {
		t.Fatal("premium-3m package not found")
	}
	if threeMonth.OriginalPrice != 499 || threeMonth.SalePrice != 399 {
		t.Fatalf("premium-3m prices = %d/%d", threeMonth.OriginalPrice, threeMonth.SalePrice)
	}
	if threeMonth.DiscountPercent != 20 {
		t.Fatalf("discount = %d, want 20", threeMonth.DiscountPercent)
	}
	if threeMonth.MonthlyPrice != 133 {
		t.Fatalf("monthly price = %d, want 133", threeMonth.MonthlyPrice)
	}
	if threeMonth.Badge != "แนะนำ" || !threeMonth.Purchasable {
		t.Fatalf("premium-3m badge/purchasable = %q/%v", threeMonth.Badge, threeMonth.Purchasable)
	}
	if threeMonth.AccessPolicy.ResultAccess != "full" || threeMonth.AccessPolicy.DailyCompletedAttempts != nil {
		t.Fatalf("premium-3m access policy = %#v, want full results and unlimited attempts", threeMonth.AccessPolicy)
	}
	if threeMonth.AccessPolicy.ExamCatalog != "all_member" || !threeMonth.AccessPolicy.Explanations || !threeMonth.AccessPolicy.WeaknessAnalysis || !threeMonth.AccessPolicy.History {
		t.Fatalf("premium-3m benefits = %#v, want all premium capabilities", threeMonth.AccessPolicy)
	}

	free, ok := Find(packages, "free")
	if !ok {
		t.Fatal("free package not found")
	}
	if free.AccessPolicy.ResultAccess != "summary" || free.AccessPolicy.DailyCompletedAttempts == nil || *free.AccessPolicy.DailyCompletedAttempts != 1 {
		t.Fatalf("free access policy = %#v, want summary results and one daily attempt", free.AccessPolicy)
	}
	if free.AccessPolicy.ExamCatalog != "free_only" || free.AccessPolicy.Explanations || free.AccessPolicy.WeaknessAnalysis || free.AccessPolicy.History {
		t.Fatalf("free benefits = %#v, want free-only summary capabilities", free.AccessPolicy)
	}

	business, ok := Find(packages, "business")
	if !ok || !business.ComingSoon || business.Purchasable {
		t.Fatalf("business = %#v, want coming soon and not purchasable", business)
	}
}

func TestFindRejectsUnknownPackage(t *testing.T) {
	packages, err := LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := Find(packages, "missing"); ok {
		t.Fatal("Find() returned unknown package")
	}
}
