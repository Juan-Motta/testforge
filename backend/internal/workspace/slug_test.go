package workspace

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Voice Team":           "voice-team",
		"Ana's workspace":      "ana-s-workspace",
		"  --Hello__World!!  ": "hello-world",
		"Équipe Voix 2":        "equipe-voix-2",
		"Equipo Señal":         "equipo-senal",
		"!!!":                  "workspace",
		"日本":                   "workspace",
		"a-very-long-name-that-keeps-going-and-going-past-the-limit": "a-very-long-name-that-keeps-going-and-going-past",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRoleCanAdminister(t *testing.T) {
	for role, want := range map[Role]bool{RoleOwner: true, RoleAdmin: true, RoleMember: false, RoleViewer: false} {
		if got := role.CanAdminister(); got != want {
			t.Errorf("%s.CanAdminister() = %v, want %v", role, got, want)
		}
	}
}
