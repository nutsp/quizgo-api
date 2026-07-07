package repository

import "testing"

func TestQuestionTypeFilterOptionsUsePublicTextValue(t *testing.T) {
	values := domainQuestionTypeValues()
	if len(values) == 0 {
		t.Fatal("expected question type filter values")
	}
	if values[0] != "text" {
		t.Fatalf("expected first question type value to be text, got %q", values[0])
	}

	labels := domainQuestionTypeLabels()
	if labels["text"] != "ข้อความทั่วไป" {
		t.Fatalf("expected Thai label for text question type, got %q", labels["text"])
	}
	if _, ok := labels["normal"]; ok {
		t.Fatal("normal is an internal alias and should not be emitted as a filter option")
	}
}

func TestFilterOptionCountMarksZeroCountAsDisabled(t *testing.T) {
	available := newFilterOptionCount("trial", "ทดลองทำ", 1)
	if available.Disabled {
		t.Fatal("expected non-zero count option to be enabled")
	}

	empty := newFilterOptionCount("paid", "ซื้อรายชุด", 0)
	if !empty.Disabled {
		t.Fatal("expected zero count option to be disabled")
	}
}
