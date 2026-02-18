package indexer

import "testing"

func TestSplitSpecifier(t *testing.T) {
	tests := []struct {
		input   string
		wantPkg string
		wantSub string
	}{
		{"@company/auth", "@company/auth", ""},
		{"@company/auth/validators", "@company/auth", "validators"},
		{"@company/auth/utils/helpers", "@company/auth", "utils/helpers"},
		{"lodash", "lodash", ""},
		{"lodash/fp", "lodash", "fp"},
		{"lodash/fp/map", "lodash", "fp/map"},
		{"@scope", "@scope", ""},
		{"react", "react", ""},
		{"react/jsx-runtime", "react", "jsx-runtime"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pkg, sub := splitSpecifier(tt.input)
			if pkg != tt.wantPkg {
				t.Errorf("splitSpecifier(%q) pkg = %q, want %q", tt.input, pkg, tt.wantPkg)
			}
			if sub != tt.wantSub {
				t.Errorf("splitSpecifier(%q) sub = %q, want %q", tt.input, sub, tt.wantSub)
			}
		})
	}
}
