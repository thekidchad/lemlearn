package lms_test

import (
	"testing"
	"time"

	"github.com/lemlearn/api/internal/lms"
)

const moduleDuration = 600_000 // dix minutes

var start = time.Date(2026, 2, 3, 18, 0, 0, 0, time.UTC)

// watch simule un visionnage continu en signaux de cinq secondes.
func watch(c *lms.Coverage, fromMs, toMs int64, at time.Time) time.Time {
	const step = 5_000
	for pos := fromMs; pos < toMs; pos += step {
		end := min(pos+step, toMs)
		at = at.Add(time.Duration(end-pos) * time.Millisecond)
		c.Apply(lms.Beat{FromMs: pos, ToMs: end, Rate: 1, Focused: true, At: at})
	}
	return at
}

func TestContinuousWatchCoversModule(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)
	watch(&coverage, 0, moduleDuration, start)

	if got := coverage.Percent(); got != 100 {
		t.Errorf("couverture = %d %%, attendu 100 %%", got)
	}
	if coverage.WatchedMs != moduleDuration {
		t.Errorf("temps visionné = %d ms, attendu %d", coverage.WatchedMs, int64(moduleDuration))
	}
	if len(coverage.Gaps()) != 0 {
		t.Errorf("trous détectés sur un visionnage complet: %v", coverage.Gaps())
	}
}

// Le test qui justifie tout le paquet : rejouer trois fois la même minute ne
// donne pas trois minutes d'assiduité.
func TestReplayDoesNotInflateCoverage(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)

	at := start
	for range 3 {
		at = watch(&coverage, 0, 60_000, at)
	}

	if got := coverage.CoveredMs(); got != 60_000 {
		t.Errorf("couverture unique = %d ms, attendu 60 000", got)
	}
	if coverage.WatchedMs != 180_000 {
		t.Errorf("temps de visionnage cumulé = %d ms, attendu 180 000", coverage.WatchedMs)
	}
	if got := coverage.Percent(); got != 10 {
		t.Errorf("couverture = %d %%, attendu 10 %% (une minute sur dix)", got)
	}
}

// Un client qui prétend avoir joué cinq minutes en dix secondes réelles ment.
func TestImpossibleProgressIsRejected(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)

	at := watch(&coverage, 0, 10_000, start)
	accepted, reason := coverage.Apply(lms.Beat{
		FromMs: 10_000, ToMs: 310_000, Rate: 1, Focused: true,
		At: at.Add(10 * time.Second),
	})

	if accepted {
		t.Fatal("une progression impossible a été acceptée")
	}
	if coverage.CoveredMs() != 10_000 {
		t.Errorf("le signal refusé a tout de même compté : %d ms", coverage.CoveredMs())
	}
	if coverage.Rejected != 1 {
		t.Errorf("compteur de refus = %d", coverage.Rejected)
	}
	t.Logf("motif du refus : %s", reason)
}

// La lecture ×2 reste admise : c'est une fonction normale des lecteurs.
func TestDoubleSpeedIsAccepted(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)

	at := start.Add(5 * time.Second)
	coverage.Apply(lms.Beat{FromMs: 0, ToMs: 10_000, Rate: 2, Focused: true, At: at})
	// Deuxième signal : dix secondes jouées en cinq secondes réelles.
	at = at.Add(5 * time.Second)
	accepted, reason := coverage.Apply(lms.Beat{FromMs: 10_000, ToMs: 20_000, Rate: 2, Focused: true, At: at})

	if !accepted {
		t.Fatalf("la lecture à vitesse double a été refusée : %s", reason)
	}
	if coverage.CoveredMs() != 20_000 {
		t.Errorf("couverture = %d ms, attendu 20 000", coverage.CoveredMs())
	}
}

// Une vidéo qui tourne dans un onglet caché n'est pas suivie.
func TestBackgroundPlaybackIsRejected(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)

	accepted, _ := coverage.Apply(lms.Beat{
		FromMs: 0, ToMs: 30_000, Rate: 1, Focused: false, At: start.Add(30 * time.Second),
	})
	if accepted {
		t.Fatal("une lecture en arrière-plan a été comptée")
	}
	if coverage.CoveredMs() != 0 {
		t.Errorf("couverture = %d ms sur une lecture en arrière-plan", coverage.CoveredMs())
	}
}

// Un saut ne doit pas colorier l'intervalle sauté : c'est la fraude la plus
// simple, et celle qu'un auditeur cherche en premier.
func TestSeekingDoesNotCoverSkippedRange(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)

	at := watch(&coverage, 0, 60_000, start)
	// L'apprenant saute à la huitième minute et reprend.
	at = at.Add(2 * time.Second)
	watch(&coverage, 480_000, 540_000, at)

	if got := coverage.CoveredMs(); got != 120_000 {
		t.Errorf("couverture = %d ms, attendu 120 000 (deux minutes vues)", got)
	}
	gaps := coverage.Gaps()
	if len(gaps) != 2 {
		t.Fatalf("%d trou(s) détecté(s), attendu 2 : %v", len(gaps), gaps)
	}
	if gaps[0][0] != 60_000 || gaps[0][1] != 480_000 {
		t.Errorf("premier trou = %v, attendu [60000 480000]", gaps[0])
	}
}

// Une position au-delà du module est un signal fabriqué.
func TestPositionBeyondModuleIsRejected(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)

	accepted, _ := coverage.Apply(lms.Beat{
		FromMs: 0, ToMs: moduleDuration * 3, Rate: 1, Focused: true, At: start.Add(time.Minute),
	})
	if accepted {
		t.Fatal("une position hors du module a été acceptée")
	}
}

// Un signal antidaté sert à contourner le contrôle de vitesse.
func TestBackdatedBeatIsRejected(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)

	at := watch(&coverage, 0, 20_000, start)
	accepted, _ := coverage.Apply(lms.Beat{
		FromMs: 20_000, ToMs: 30_000, Rate: 1, Focused: true, At: at.Add(-time.Minute),
	})
	if accepted {
		t.Fatal("un signal antidaté a été accepté")
	}
}

// Le bitmap doit survivre à un aller-retour par la base : c'est ainsi qu'il
// est réellement conservé entre deux séances.
func TestCoverageSurvivesSerialization(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)
	watch(&coverage, 0, 180_000, start)

	restored := lms.Coverage{
		DurationMs: coverage.DurationMs,
		Segments:   append([]byte(nil), coverage.Segments...),
		WatchedMs:  coverage.WatchedMs,
		FirstAt:    coverage.FirstAt,
		LastAt:     coverage.LastAt,
	}
	if restored.CoveredMs() != coverage.CoveredMs() {
		t.Errorf("couverture après relecture = %d ms, attendu %d", restored.CoveredMs(), coverage.CoveredMs())
	}

	// Et la séance suivante doit repartir de là, sans recompter.
	watch(&restored, 180_000, 240_000, coverage.LastAt.Add(time.Hour))
	if got := restored.CoveredMs(); got != 240_000 {
		t.Errorf("couverture cumulée = %d ms, attendu 240 000", got)
	}
}

// Le bitmap doit rester compact : c'est ce qui permet de le garder en base
// plutôt que de rejouer des millions de signaux bruts à chaque lecture.
func TestBitmapStaysCompact(t *testing.T) {
	// Deux heures de vidéo.
	coverage := lms.NewCoverage(2 * 60 * 60 * 1000)
	if len(coverage.Segments) > 128 {
		t.Errorf("bitmap de %d octets pour deux heures de vidéo", len(coverage.Segments))
	}
}

func TestPartialCoveragePercent(t *testing.T) {
	coverage := lms.NewCoverage(moduleDuration)
	watch(&coverage, 0, 540_000, start) // neuf minutes sur dix

	if got := coverage.Percent(); got != 90 {
		t.Errorf("couverture = %d %%, attendu 90 %%", got)
	}
}
