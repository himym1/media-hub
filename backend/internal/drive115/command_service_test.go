package drive115

import "testing"

func TestValidateCommandInput(t *testing.T) {
	tests := []struct {
		name  string
		input CommandInput
		valid bool
	}{
		{"folder", CommandInput{Operation: "create_folder", Params: map[string]any{"parentId": "0", "name": "Movies"}}, true},
		{"move", CommandInput{Operation: "move", Params: map[string]any{"fileIds": []any{"12", "13"}, "targetParentId": "22"}}, true},
		{"rename path", CommandInput{Operation: "rename", Params: map[string]any{"fileId": "12", "name": "../private"}}, false},
		{"delete empty", CommandInput{Operation: "delete", Params: map[string]any{"fileIds": []any{}}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCommandInput(test.input)
			if test.valid && err != nil {
				t.Fatalf("validate: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
