package prompt

import (
	"bytes"
	"testing"
)

func TestParseKeepsValuesUnescaped(t *testing.T) {
	// The model must see names exactly as Paperless stores them; "&amp;" would make it
	// create new correspondents and tags with the escaped name.
	tpl, err := Parse("json", `Tags: {{.Tags | join ", "}}`+"\n"+`Text: {{.Content}}`)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := tpl.Execute(&b, map[string]interface{}{"Tags": []string{"Rechnungen & Belege", "Engel & Völkers"}, "Content": `Betrag "5 < 10" & 'ok'`}); err != nil {
		t.Fatal(err)
	}
	want := "Tags: Rechnungen & Belege, Engel & Völkers\nText: Betrag \"5 < 10\" & 'ok'"
	if b.String() != want {
		t.Fatalf("rendered %q, want %q", b.String(), want)
	}
}
