package interactive

import (
	"bytes"
	"strings"
	"testing"
)

func TestAccessiblePromptUsesRussianAndAcceptsDefaults(t *testing.T) {
	input := strings.NewReader("\n\nда\n")
	var output bytes.Buffer
	prompt := NewHuhPrompter(input, &output)
	prompt.Accessible = true
	prompt.SetLanguage("ru")

	selected, err := prompt.Select("Язык", "", []Option{{Label: "English", Value: "en"}, {Label: "Русский", Value: "ru"}}, "ru")
	if err != nil {
		t.Fatal(err)
	}
	value, err := prompt.Input("Интервал", "", "5h3m", nil)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := prompt.Confirm("Продолжить", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "ru" || value != "5h3m" || !confirmed {
		t.Fatalf("selected=%q value=%q confirmed=%t", selected, value, confirmed)
	}
	if !strings.Contains(output.String(), "Выберите номер [2]") || !strings.Contains(output.String(), "Значение [5h3m]") {
		t.Fatalf("output was not localized: %q", output.String())
	}
}

func TestAccessiblePromptCancelsOnEOF(t *testing.T) {
	prompt := NewHuhPrompter(strings.NewReader(""), &bytes.Buffer{})
	prompt.Accessible = true
	_, err := prompt.Input("Value", "", "", nil)
	if err != ErrCancelled {
		t.Fatalf("error = %v", err)
	}
}
