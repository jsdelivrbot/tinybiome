package tinybiome

import (
	"testing"
)

func TestActorView(t *testing.T) {
	var av ActorView
	testActors := []*Actor{&Actor{ID: 10}, &Actor{ID: 20}}
	changes := av.Changes(testActors)

	t.Logf("%d changes", len(changes))
	if len(changes) != 2 {
		t.Error("incorrect amount of changes")
	}
	for _, c := range changes {
		t.Logf("change: %s", c.String())
		av.Apply(c)
	}

	changes = av.Changes(testActors)
	t.Logf("%d changes", len(changes))
	if len(changes) != 0 {
		t.Error("incorrect amount of changes")
	}
	for _, c := range changes {
		t.Logf("change: %s", c.String())
		av.Apply(c)
	}

	testActors = append(testActors[1:], &Actor{ID: 15})

	changes = av.Changes(testActors)
	t.Logf("%d changes", len(changes))
	if len(changes) != 2 {
		t.Error("incorrect amount of changes")
	}
	for _, c := range changes {
		t.Logf("change: %s", c.String())
		av.Apply(c)
	}
}
