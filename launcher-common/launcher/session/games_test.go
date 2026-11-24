package session

import (
	"slices"
	"testing"
)

func TestValidateServerGames(t *testing.T) {
	games, err := validateServerGames(nil, "age2")
	if err != nil || !slices.Equal(games, []string{"age2"}) {
		t.Fatalf("empty Games = %v, %v", games, err)
	}
	games, err = validateServerGames([]string{" age1 ", "age2"}, "age2")
	if err != nil || !slices.Equal(games, []string{"age1", "age2"}) {
		t.Fatalf("Games = %v, %v", games, err)
	}
	if _, err = validateServerGames([]string{"age1", "age9"}, "age1"); err == nil {
		t.Fatal("unsupported game must fail")
	}
	if _, err = validateServerGames([]string{"age1", "age1"}, "age1"); err == nil {
		t.Fatal("duplicate game must fail")
	}
	if _, err = validateServerGames([]string{"age1", "age3"}, "age2"); err == nil {
		t.Fatal("Games without the current game must fail")
	}
}
