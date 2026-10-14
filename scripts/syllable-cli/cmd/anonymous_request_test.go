package cmd

import "testing"

func TestOnlyExplicitSDKAnonymousRequestSkipsCredentialRequirement(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{{"true", true}, {"false", false}} {
		cmd := usersCmd()
		if err := bindSDKQueryParameters(cmd); err != nil {
			t.Fatal(err)
		}
		leaf, rest, err := cmd.Find([]string{"send-email", "audit@example.invalid", "--skip-auth=" + tc.value})
		if err != nil {
			t.Fatal(err)
		}
		if err = leaf.ParseFlags(rest); err != nil {
			t.Fatal(err)
		}
		if got := sdkAnonymousRequest(leaf); got != tc.want {
			t.Errorf("skip-auth %s anonymous=%t", tc.value, got)
		}
	}
	if sdkAnonymousRequest(agentsCmd()) || sdkAnonymousRequest(nil) {
		t.Fatal("other operations must require credentials")
	}
}
