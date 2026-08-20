package identity_test

import (
	"context"
	"testing"
	"time"

	"github.com/lemlearn/api/internal/identity"
	"github.com/lemlearn/api/internal/platform/ddb"
)

func invited(t *testing.T) (*identity.Service, identity.Org, string) {
	t.Helper()
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Institut Vulcain", "marie@vulcain.fr")

	_, token, err := s.InviteLearner(context.Background(), identity.InviteInput{
		OrgID: org.ID, ContactID: "CON-1", Email: "lea@example.fr",
		FirstName: "Léa", LastName: "Bertrand",
	})
	if err != nil {
		t.Fatalf("invitation: %v", err)
	}
	return s, org, token
}

// Un compte invité n'existe qu'en creux tant que l'apprenant n'a pas choisi de
// mot de passe : lui en fabriquer un obligerait à le lui transmettre, donc à
// le faire circuler en clair dans un courriel qui reste dans une boîte.
func TestInvitedAccountCannotLogInBeforeAccepting(t *testing.T) {
	s, _, _ := invited(t)

	if _, _, err := s.Login(context.Background(), "lea@example.fr", "",
		"127.0.0.1", "test"); err == nil {
		t.Fatal("un compte invité s'est connecté sans mot de passe")
	}
	if _, _, err := s.Login(context.Background(), "lea@example.fr", "n-importe-quoi",
		"127.0.0.1", "test"); err == nil {
		t.Fatal("un compte invité s'est connecté avec un mot de passe inventé")
	}
}

// Le lien fixe le mot de passe, active le compte et le relie à sa fiche : sans
// ce lien, l'espace apprenant ne saurait pas de quelles inscriptions il parle.
func TestAcceptingTheInvitationOpensTheLearnerSpace(t *testing.T) {
	s, org, token := invited(t)

	user, err := s.AcceptInvitation(context.Background(), token, "trombone-cactus-rivage")
	if err != nil {
		t.Fatalf("acceptation: %v", err)
	}
	if user.Role != identity.RoleLearner {
		t.Errorf("rôle = %q", user.Role)
	}
	if user.ContactID != "CON-1" {
		t.Errorf("fiche rattachée = %q", user.ContactID)
	}
	if user.OrgID != org.ID {
		t.Errorf("organisation = %q", user.OrgID)
	}

	if _, _, err := s.Login(context.Background(), "lea@example.fr", "trombone-cactus-rivage",
		"127.0.0.1", "test"); err != nil {
		t.Fatalf("connexion après acceptation: %v", err)
	}
}

// Un lien d'invitation sert une fois. Le laisser rejouable permettrait à
// quiconque l'a vu passer de reprendre le compte en changeant le mot de passe.
func TestInvitationTokenIsSingleUse(t *testing.T) {
	s, _, token := invited(t)

	if _, err := s.AcceptInvitation(context.Background(), token, "trombone-cactus-rivage"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptInvitation(context.Background(), token, "autre-mot-de-passe-long"); err == nil {
		t.Fatal("le lien a resservi")
	}
	if _, err := s.ResolveInvitation(context.Background(), token); err == nil {
		t.Error("un lien consommé se résout encore")
	}
}

// Réinviter quelqu'un ne doit pas créer un second compte : deux fiches se
// disputeraient la même personne, et la réservation d'adresse casserait.
func TestReinvitingReusesTheSameAccount(t *testing.T) {
	s, org, _ := invited(t)

	first, err := s.ResolveInvitationUser(context.Background(), org.ID, "lea@example.fr")
	if err != nil {
		t.Fatal(err)
	}

	again, token, err := s.InviteLearner(context.Background(), identity.InviteInput{
		OrgID: org.ID, ContactID: "CON-1", Email: "lea@example.fr",
		FirstName: "Léa", LastName: "Bertrand",
	})
	if err != nil {
		t.Fatalf("seconde invitation: %v", err)
	}
	if again.ID != first.ID {
		t.Errorf("second compte créé : %q puis %q", first.ID, again.ID)
	}
	// Et le nouveau lien fonctionne, sinon réinviter ne servirait à rien.
	if _, err := s.AcceptInvitation(context.Background(), token, "trombone-cactus-rivage"); err != nil {
		t.Fatalf("le second lien ne marche pas: %v", err)
	}
}

// Un lien expiré se refuse avec un motif qu'un apprenant peut suivre.
func TestExpiredInvitationIsRefused(t *testing.T) {
	clock := time.Now().UTC()
	s := identity.NewService(ddb.NewTestClient(t), func() time.Time { return clock })
	org := register(t, s, "Institut Vulcain", "marie@vulcain.fr")

	_, token, err := s.InviteLearner(context.Background(), identity.InviteInput{
		OrgID: org.ID, ContactID: "CON-1", Email: "lea@example.fr",
	})
	if err != nil {
		t.Fatal(err)
	}

	clock = clock.Add(identity.InvitationTTL + time.Hour)
	if _, err := s.ResolveInvitation(context.Background(), token); err == nil {
		t.Fatal("un lien expiré a été accepté")
	}
}

// Une adresse sans arobase n'est pas une adresse : le dire à la création
// évite un compte que personne ne pourra jamais activer.
func TestInvitationNeedsAnAddress(t *testing.T) {
	s := identity.NewService(ddb.NewTestClient(t), nil)
	org := register(t, s, "Institut Vulcain", "marie@vulcain.fr")

	if _, _, err := s.InviteLearner(context.Background(), identity.InviteInput{
		OrgID: org.ID, ContactID: "CON-1", Email: "",
	}); err == nil {
		t.Fatal("une invitation sans adresse a été acceptée")
	}
}
