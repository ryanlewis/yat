package graph

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func assertCountMatches(t *testing.T, g *Graph) {
	t.Helper()

	for id := range g.items {
		if got, want := g.countUnblockedBy(id), len(g.UnblockedBy(id)); got != want {
			t.Errorf("countUnblockedBy(%s) = %d, want %d", id, got, want)
		}
	}
}

func TestCountUnblockedBy_Fixture(t *testing.T) {
	items := makeItems()
	assertCountMatches(t, mustBuild(t, items))

	items[1].Status = item.StatusDone
	assertCountMatches(t, mustBuild(t, items))
}

func TestCountUnblockedBy_Generated(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	statuses := []item.Status{item.StatusTodo, item.StatusInProgress, item.StatusDone}

	for round := 0; round < 50; round++ {
		items := make([]*item.Item, 0, 20)

		for n := 0; n < 20; n++ {
			it := &item.Item{ID: fmt.Sprintf("T-%02d", n), Status: statuses[rng.Intn(len(statuses))]}
			for d := 0; d < n; d++ {
				if rng.Intn(4) == 0 {
					it.Dependencies = append(it.Dependencies, fmt.Sprintf("T-%02d", d))
				}
			}

			items = append(items, it)
		}

		assertCountMatches(t, mustBuild(t, items))
	}
}
