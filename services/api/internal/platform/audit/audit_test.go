package audit

import (
	"strings"
	"testing"
	"time"
)

func chain(t *testing.T) []Event {
	t.Helper()
	at := time.Date(2026, 1, 14, 9, 12, 0, 0, time.UTC)
	actor := Actor{Type: ActorUser, ID: "USR-1", Label: "Marie Dubreuil", IP: "82.65.14.3"}

	steps := []struct {
		action  Action
		payload map[string]any
	}{
		{ActionFileCreated, map[string]any{"reference": "DOS-2026-0143"}},
		{ActionDocumentGenerated, map[string]any{"document": "CONV-2026-0143", "sha256": "9f2b…"}},
		{ActionSignatureOTPSent, map[string]any{"channel": "email"}},
		{ActionDocumentSigned, map[string]any{"signer": "lea.bertrand@example.fr", "tsa": "freetsa"}},
	}

	var events []Event
	var prev Event
	for i, step := range steps {
		event, err := Append(prev, "file/DOS-2026-0143", at.Add(time.Duration(i)*time.Minute), step.action, actor, step.payload)
		if err != nil {
			t.Fatalf("append %s: %v", step.action, err)
		}
		events = append(events, event)
		prev = event
	}
	return events
}

func TestChainVerifies(t *testing.T) {
	if err := Verify(chain(t)); err != nil {
		t.Fatalf("chaîne saine rejetée: %v", err)
	}
}

func TestFirstEventHasNoPrevHash(t *testing.T) {
	events := chain(t)
	if events[0].PrevHash != GenesisHash {
		t.Errorf("le premier événement porte une empreinte précédente: %q", events[0].PrevHash)
	}
	if events[0].Seq != 1 {
		t.Errorf("rang initial = %d, attendu 1", events[0].Seq)
	}
}

// Le cas qui justifie tout le paquet : modifier une valeur déjà journalisée
// doit être détectable, même si l'attaquant a accès à la base.
func TestTamperedPayloadIsDetected(t *testing.T) {
	events := chain(t)
	events[1].Payload["sha256"] = "empreinte substituée"

	err := Verify(events)
	if err == nil {
		t.Fatal("une charge utile modifiée a passé la vérification")
	}
	if !strings.Contains(err.Error(), "altéré") {
		t.Errorf("message peu explicite: %v", err)
	}
}

// Supprimer un événement gênant — un échec d'OTP, par exemple — doit casser
// la chaîne aussi sûrement qu'une modification.
func TestDeletedEventIsDetected(t *testing.T) {
	events := chain(t)
	withHole := append([]Event{}, events[0], events[2], events[3])

	if err := Verify(withHole); err == nil {
		t.Fatal("un événement supprimé a passé la vérification")
	}
}

// Réordonner les événements pour antidater une signature doit échouer.
func TestReorderedEventsAreDetected(t *testing.T) {
	events := chain(t)
	events[1], events[2] = events[2], events[1]

	if err := Verify(events); err == nil {
		t.Fatal("des événements réordonnés ont passé la vérification")
	}
}

// Remplacer un événement par un autre, correctement haché mais dont
// l'empreinte précédente ne correspond pas, doit échouer : c'est la tentative
// la plus crédible, celle d'un attaquant qui connaît le format.
func TestForgedEventWithValidOwnHashIsDetected(t *testing.T) {
	events := chain(t)

	forged, err := Append(Event{}, "file/DOS-2026-0143", events[2].At, ActionDocumentSigned, events[2].Actor, map[string]any{"signer": "faux"})
	if err != nil {
		t.Fatalf("forge: %v", err)
	}
	forged.Seq = 3
	rehashed, err := forged.computeHash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	forged.Hash = rehashed
	events[2] = forged

	if err := Verify(events); err == nil {
		t.Fatal("un événement forgé avec sa propre empreinte valide a passé la vérification")
	}
}

// L'ordre d'insertion des clés d'une charge utile ne doit pas changer
// l'empreinte, sinon deux serveurs calculeraient des chaînes divergentes.
func TestHashIsIndependentOfMapOrder(t *testing.T) {
	at := time.Date(2026, 2, 3, 18, 47, 0, 0, time.UTC)
	actor := Actor{Type: ActorLearner, ID: "LRN-9"}

	first, err := Append(Event{}, "enrollment/ENR-91", at, ActionWatchProgress,
		actor, map[string]any{"module": "MOD-2", "watched_ms": 2460000, "coverage": 0.96})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Append(Event{}, "enrollment/ENR-91", at, ActionWatchProgress,
		actor, map[string]any{"coverage": 0.96, "watched_ms": 2460000, "module": "MOD-2"})
	if err != nil {
		t.Fatal(err)
	}

	if first.Hash != second.Hash {
		t.Errorf("empreintes divergentes pour la même charge utile:\n%s\n%s", first.Hash, second.Hash)
	}
}

func TestAppendRejectsCrossSubjectChaining(t *testing.T) {
	events := chain(t)
	if _, err := Append(events[3], "file/DOS-AUTRE", events[3].At, ActionFileCreated, events[3].Actor, nil); err == nil {
		t.Fatal("le chaînage entre deux sujets différents a été accepté")
	}
}

func TestVerifyRejectsBackwardsTimestamp(t *testing.T) {
	at := time.Date(2026, 1, 14, 9, 0, 0, 0, time.UTC)
	actor := Actor{Type: ActorSystem, ID: "cron"}

	first, _ := Append(Event{}, "org/ORG-7", at, ActionDocumentSent, actor, nil)
	// Antidatage : l'événement suivant prétend s'être produit avant le premier.
	second, err := Append(first, "org/ORG-7", at.Add(-time.Hour), ActionDocumentSigned, actor, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Verify([]Event{first, second}); err == nil {
		t.Fatal("un horodatage antidaté a passé la vérification")
	}
}
