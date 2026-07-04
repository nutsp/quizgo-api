package mathconvert

import "testing"

func TestConvertSimpleMath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"sqrt", "ค่าของ sqrt(49) คือข้อใด", "ค่าของ $\\sqrt{49}$ คือข้อใด"},
		{"power", "2^3", "$2^{3}$"},
		{"fraction math row", "1/2", "$\\frac{1}{2}$"},
		{"skip date", "วันที่ 1/2/2569", "วันที่ 1/2/2569"},
		{"preserve latex", `$\\frac{2^3}{4}$`, `$\\frac{2^3}{4}$`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertSimpleMath(tt.in)
			if got != tt.want {
				t.Fatalf("ConvertSimpleMath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHasStrongMathSignals(t *testing.T) {
	if !HasStrongMathSignals("ค่าของ sqrt(49) คือข้อใด") {
		t.Fatal("expected sqrt signal")
	}
	if HasStrongMathSignals("ข้อใดเป็นคำราชาศัพท์") {
		t.Fatal("did not expect plain text signal")
	}
	if HasStrongMathSignals("1/2") {
		t.Fatal("plain fraction alone should not auto-detect")
	}
}

func TestShouldConvert(t *testing.T) {
	if !ShouldConvert("math", "") {
		t.Fatal("math type should convert")
	}
	if !ShouldConvert("", "markdown_math") {
		t.Fatal("markdown_math should convert")
	}
	if ShouldConvert("normal", "plain") {
		t.Fatal("plain should not convert")
	}
}
