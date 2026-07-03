package signup

import (
	"reflect"
	"testing"
)

func TestSplitFlagsAndPositional(t *testing.T) {
	cases := []struct {
		name           string
		in             []string
		wantFlags      []string
		wantPositional []string
	}{
		{
			name:           "flags before positionals",
			in:             []string{"--json", "a", "b"},
			wantFlags:      []string{"--json"},
			wantPositional: []string{"a", "b"},
		},
		{
			name:           "flags after positionals",
			in:             []string{"a", "b", "--json"},
			wantFlags:      []string{"--json"},
			wantPositional: []string{"a", "b"},
		},
		{
			name:           "flags interleaved",
			in:             []string{"a", "--json", "b"},
			wantFlags:      []string{"--json"},
			wantPositional: []string{"a", "b"},
		},
		{
			name:           "single-dash flag",
			in:             []string{"-v", "user"},
			wantFlags:      []string{"-v"},
			wantPositional: []string{"user"},
		},
		{
			name:           "name=value stays as one token",
			in:             []string{"user", "--out=foo.json", "user2"},
			wantFlags:      []string{"--out=foo.json"},
			wantPositional: []string{"user", "user2"},
		},
		{
			name:           "double-dash terminator",
			in:             []string{"--json", "--", "--weirdname", "user"},
			wantFlags:      []string{"--json"},
			wantPositional: []string{"--weirdname", "user"},
		},
		{
			name:           "no args",
			in:             nil,
			wantFlags:      nil,
			wantPositional: nil,
		},
		{
			name:           "only positionals",
			in:             []string{"a", "b", "c"},
			wantFlags:      nil,
			wantPositional: []string{"a", "b", "c"},
		},
		{
			name:           "only flags",
			in:             []string{"--json", "-v"},
			wantFlags:      []string{"--json", "-v"},
			wantPositional: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotFlags, gotPositional := splitFlagsAndPositional(tc.in)
			if !reflect.DeepEqual(gotFlags, tc.wantFlags) {
				t.Errorf("flags = %v, want %v", gotFlags, tc.wantFlags)
			}
			if !reflect.DeepEqual(gotPositional, tc.wantPositional) {
				t.Errorf("positional = %v, want %v", gotPositional, tc.wantPositional)
			}
		})
	}
}
