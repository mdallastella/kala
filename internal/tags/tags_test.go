package tags

import (
	"slices"
	"testing"
)

func TestNamespaceValidateOptionalValues(t *testing.T) {
	open := Namespace{Name: "client", Exclusive: true}
	if err := open.Validate(); err != nil {
		t.Fatalf("exclusive without values: %v", err)
	}
	withDefault := Namespace{Name: "client", Exclusive: true, Default: "acme"}
	if err := withDefault.Validate(); err == nil {
		t.Fatal("default without values: want error")
	}
}

func TestApplyExclusive(t *testing.T) {
	r := Rules{
		"client": {Name: "client", Exclusive: true},
		"status": {Name: "status", Exclusive: true, Values: []Value{{Value: "open"}, {Value: "closed", Terminal: true}}},
	}
	cases := []struct {
		name    string
		current []string
		add     []string
		want    []string
		wantErr bool
	}{
		{name: "open list accepts any value", current: nil, add: []string{"client:fimi"}, want: []string{"client:fimi"}},
		{name: "different head replaces", current: []string{"client:fimi", "client:fimi:ops", "x"}, add: []string{"client:acme"}, want: []string{"client:acme", "x"}},
		{name: "same head is kept", current: []string{"client:fimi"}, add: []string{"client:fimi:ops"}, want: []string{"client:fimi", "client:fimi:ops"}},
		{name: "same head added together", add: []string{"client:fimi", "client:fimi:ops"}, want: []string{"client:fimi", "client:fimi:ops"}},
		{name: "different heads added together", add: []string{"client:fimi", "client:acme"}, wantErr: true},
		{name: "bare namespace tag", add: []string{"client"}, wantErr: true},
		{name: "closed list rejects unknown", add: []string{"status:opne"}, wantErr: true},
		{name: "closed list accepts sub-value of listed head", current: []string{"status:open"}, add: []string{"status:closed:won"}, want: []string{"status:closed:won"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := r.Apply(c.current, c.add, nil)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
	if !r.Terminal([]string{"status:closed:won"}) {
		t.Error("a sub-value of a terminal value should be terminal")
	}
}

func TestRulesValidateExclusive(t *testing.T) {
	r := Rules{"client": {Name: "client", Exclusive: true}}
	ok := [][]string{{"client:fimi"}, {"client:fimi", "client:fimi:ops"}, {"other", "client:acme"}}
	for _, set := range ok {
		if err := r.Validate(set); err != nil {
			t.Errorf("%v: %v", set, err)
		}
	}
	bad := [][]string{{"client"}, {"client:fimi", "client:acme"}}
	for _, set := range bad {
		if err := r.Validate(set); err == nil {
			t.Errorf("%v: want error", set)
		}
	}
}
