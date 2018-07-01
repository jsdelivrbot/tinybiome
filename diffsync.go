package tinybiome

import (
	"fmt"
)

type Change interface {
	fmt.Stringer
}

type ActorView struct {
	known map[int64]*Actor
}

type addToActorView struct {
	*Actor
}

func (a addToActorView) String() string {
	return fmt.Sprintf("add actor %d", a.ID)
}

type removeFromActorView struct {
	ID int64
}

func (a removeFromActorView) String() string {
	return fmt.Sprintf("remove actor %d", a.ID)
}

func (av *ActorView) Apply(c Change) {
	if av.known == nil {
		av.known = make(map[int64]*Actor)
	}
	switch t := c.(type) {
	case addToActorView:
		av.known[t.ID] = t.Actor
	case removeFromActorView:
		delete(av.known, t.ID)
	}
}

func (av ActorView) Changes(goal []*Actor) (changes []Change) {
	goalIDs := make(map[int64]struct{}, len(goal))
	for _, actor := range goal {
		if _, found := av.known[actor.ID]; !found {
			changes = append(changes, addToActorView{actor})
		}
		goalIDs[actor.ID] = struct{}{}
	}
	for id, _ := range av.known {
		if _, found := goalIDs[id]; !found {
			changes = append(changes, removeFromActorView{id})
		}
	}
	return
}
