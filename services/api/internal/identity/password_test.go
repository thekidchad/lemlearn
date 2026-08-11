package identity

import (
	"strings"
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	const password = "correcte-agrafe-cheval-pile"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("empreinte: %v", err)
	}
	if strings.Contains(hash, password) {
		t.Fatal("le mot de passe apparaît en clair dans l'empreinte")
	}
	if !VerifyPassword(password, hash) {
		t.Error("le bon mot de passe est refusé")
	}
	if VerifyPassword(password+"x", hash) {
		t.Error("un mot de passe faux est accepté")
	}
}

// Deux empreintes du même mot de passe doivent différer : sans sel aléatoire,
// une fuite de la base révélerait quels comptes partagent un mot de passe.
func TestHashIsSalted(t *testing.T) {
	first, err := HashPassword("correcte-agrafe-cheval-pile")
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword("correcte-agrafe-cheval-pile")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("deux empreintes identiques : le sel n'est pas aléatoire")
	}
}

func TestShortPasswordIsRejected(t *testing.T) {
	if _, err := HashPassword("court1234"); err == nil {
		t.Error("un mot de passe de moins de 12 caractères a été accepté")
	}
}

// Une empreinte tronquée, vide ou d'un autre format ne doit jamais valider,
// et surtout ne doit pas paniquer : elle peut venir d'une donnée corrompue.
func TestVerifyRejectsMalformedHash(t *testing.T) {
	for _, malformed := range []string{
		"",
		"pas-une-empreinte",
		"$argon2id$v=19$m=19456,t=2,p=1$sel",
		"$argon2i$v=19$m=19456,t=2,p=1$c2VsCg$a2V5Cg",
		"$argon2id$v=99$m=19456,t=2,p=1$c2VsCg$a2V5Cg",
	} {
		if VerifyPassword("correcte-agrafe-cheval-pile", malformed) {
			t.Errorf("empreinte malformée acceptée: %q", malformed)
		}
	}
}

// L'empreinte leurre sert à faire payer le même coût à un e-mail inconnu qu'à
// un compte réel. Si elle devenait invalide, VerifyPassword rendrait la main
// immédiatement et le canal temporel se rouvrirait.
func TestDecoyHashIsWellFormed(t *testing.T) {
	if _, _, _, _, _, err := decodePHC(decoyHash); err != nil {
		t.Fatalf("l'empreinte leurre n'est pas décodable, le coût de calcul n'est plus équivalent: %v", err)
	}
}

func TestTokenIsOpaqueAndHashed(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 40 {
		t.Errorf("jeton trop court: %d caractères", len(token))
	}
	if token == hash {
		t.Fatal("le jeton et son empreinte sont identiques")
	}
	if HashToken(token) != hash {
		t.Error("l'empreinte n'est pas reproductible")
	}

	other, _, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if other == token {
		t.Fatal("deux jetons identiques")
	}
}

func TestOTPFormat(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		code, err := NewOTP()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 {
			t.Fatalf("code de %d chiffres: %q", len(code), code)
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("caractère non numérique dans %q", code)
			}
		}
		seen[code] = true
	}
	// 200 tirages sur un million de valeurs : quelques collisions sont
	// possibles, une poignée de valeurs distinctes ne l'est pas.
	if len(seen) < 150 {
		t.Errorf("seulement %d codes distincts sur 200 tirages", len(seen))
	}
}

func TestRolePermissions(t *testing.T) {
	cases := []struct {
		role      Role
		manageCRM bool
		teach     bool
	}{
		{RoleOwner, true, true},
		{RoleAdmin, true, true},
		{RoleTrainer, false, true},
		{RoleLearner, false, false},
		{RoleSuperAdmin, true, true},
	}
	for _, c := range cases {
		if got := c.role.CanManageCRM(); got != c.manageCRM {
			t.Errorf("%s.CanManageCRM() = %v", c.role, got)
		}
		if got := c.role.CanTeach(); got != c.teach {
			t.Errorf("%s.CanTeach() = %v", c.role, got)
		}
	}
	if Role("inventé").Valid() {
		t.Error("un rôle inconnu est déclaré valide")
	}
}
