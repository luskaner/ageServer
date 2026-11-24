package launcher

import "testing"

func TestValidateGameArgs(t *testing.T) {
	valid := [][]string{
		{"-e", "{Game}", "--id", "{Id}"},
		{"-e", "{Game}"},
		{"--log", "-e", "{Game}", "--id", "{Id}"},
	}
	for _, args := range valid {
		if err := ValidateGameArgs(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	invalid := [][]string{
		{"-e", "{Games}"},
		{"-e", "{Game}", "{Game}"},
		{"--id", "{Game}"},
		{"-e", "{Game}", "-e", "{Game}"},
		{"-e", "age2"},
		{"-e"},
		{"{Games}", "-e", "{Game}"},
	}
	for _, args := range invalid {
		if err := ValidateGameArgs(args); err == nil {
			t.Fatalf("%v must be rejected", args)
		}
	}
}
