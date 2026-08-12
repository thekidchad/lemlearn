// Package lms mesure l'assiduité réelle sur les modules vidéo.
//
// C'est la pièce la plus scrutée d'un audit Qualiopi pour une formation à
// distance : il faut pouvoir produire, apprenant par apprenant, la durée
// réellement suivie de chaque module. « Réellement » exclut trois choses que
// le client peut affirmer sans qu'elles soient vraies — avoir regardé en
// accéléré, avoir laissé tourner la vidéo en arrière-plan, et avoir rejoué
// dix fois la même minute.
package lms

import (
	"fmt"
	"time"
)

// SegmentMs est la granularité de la couverture : dix secondes.
//
// Assez fin pour qu'un module de vingt minutes se décrive en cent vingt
// segments — quinze octets de bitmap — et assez grossier pour qu'un apprenant
// qui saute deux secondes de générique ne soit pas compté absent.
const SegmentMs = 10_000

// MaxRate est la vitesse de lecture maximale admise dans le décompte.
//
// Les lecteurs proposent jusqu'à ×2. Au-delà, ce n'est plus du visionnage, et
// un client qui prétend avoir joué trente secondes en dix secondes réelles
// est soit accéléré au-delà du permis, soit en train de mentir.
const MaxRate = 2.0

// clockTolerance absorbe la dérive d'horloge et la latence réseau entre deux
// signaux. Sans elle, un apprenant sur une connexion lente serait sanctionné
// pour du temps qu'il a bel et bien passé devant la vidéo.
const clockTolerance = 3 * time.Second

// Coverage est le relevé de ce qu'un apprenant a réellement vu d'un module.
type Coverage struct {
	// DurationMs est la durée du module, connue à l'encodage.
	DurationMs int64 `dynamodbav:"durationMs" json:"durationMs"`
	// Segments est un bitmap : un bit par tranche de dix secondes, à 1 si la
	// tranche a été vue au moins une fois. Rejouer trois fois la même minute
	// n'allume pas trois fois les mêmes bits — c'est tout l'intérêt.
	Segments []byte `dynamodbav:"segments" json:"-"`
	// WatchedMs est le temps de visionnage validé, cumulé. Il peut dépasser
	// la durée du module (l'apprenant a rejoué des passages) ; c'est
	// `CoveredMs` qui mesure la progression.
	WatchedMs int64 `dynamodbav:"watchedMs" json:"watchedMs"`
	// Rejected compte les signaux écartés : progression impossible, lecture
	// en arrière-plan, position hors du module. Un compteur élevé mérite un
	// regard, et il figure au dossier de preuve.
	Rejected int `dynamodbav:"rejected" json:"rejected"`

	FirstAt  time.Time `dynamodbav:"firstAt,omitempty" json:"firstAt,omitempty"`
	LastAt   time.Time `dynamodbav:"lastAt,omitempty" json:"lastAt,omitempty"`
	LastPos  int64     `dynamodbav:"lastPositionMs" json:"lastPositionMs"`
	Sessions int       `dynamodbav:"sessions" json:"sessions"`
}

// NewCoverage prépare le relevé d'un module de durée connue.
func NewCoverage(durationMs int64) Coverage {
	return Coverage{DurationMs: durationMs, Segments: make([]byte, segmentBytes(durationMs))}
}

// segmentCount est le nombre de tranches d'un module.
func segmentCount(durationMs int64) int {
	if durationMs <= 0 {
		return 0
	}
	return int((durationMs + SegmentMs - 1) / SegmentMs)
}

func segmentBytes(durationMs int64) int {
	return (segmentCount(durationMs) + 7) / 8
}

// Beat est un signal envoyé par le lecteur, toutes les cinq secondes.
//
// Le client déclare l'intervalle qu'il vient de jouer, pas seulement sa
// position : c'est ce qui permet de distinguer une lecture continue d'un saut,
// sans avoir à deviner.
type Beat struct {
	// FromMs et ToMs délimitent l'intervalle joué depuis le signal précédent.
	FromMs int64 `json:"fromMs"`
	ToMs   int64 `json:"toMs"`
	// Rate est la vitesse de lecture déclarée.
	Rate float64 `json:"rate"`
	// Focused indique que l'onglet était au premier plan. Une vidéo qui tourne
	// dans un onglet caché n'est pas suivie.
	Focused bool `json:"focused"`
	// At est l'heure serveur de réception. Elle n'est jamais prise du client :
	// c'est elle qui borne ce qu'il peut prétendre avoir joué.
	At time.Time `json:"-"`
}

// Apply intègre un signal et dit s'il a été retenu.
//
// La règle tient en une phrase : on ne peut pas avoir joué plus de temps qu'il
// ne s'en est écoulé, à la vitesse maximale près.
func (c *Coverage) Apply(beat Beat) (accepted bool, reason string) {
	if c.DurationMs <= 0 {
		return false, "durée du module inconnue"
	}
	if len(c.Segments) < segmentBytes(c.DurationMs) {
		grown := make([]byte, segmentBytes(c.DurationMs))
		copy(grown, c.Segments)
		c.Segments = grown
	}

	if !beat.Focused {
		c.Rejected++
		return false, "lecture en arrière-plan"
	}

	from, to := beat.FromMs, beat.ToMs
	if to <= from {
		c.Rejected++
		return false, "intervalle vide ou inversé"
	}
	if from < 0 || to > c.DurationMs+SegmentMs {
		// La tolérance d'un segment couvre l'arrondi de fin de piste : un
		// lecteur annonce volontiers une position légèrement au-delà de la
		// durée déclarée.
		c.Rejected++
		return false, "position hors du module"
	}
	if to > c.DurationMs {
		to = c.DurationMs
	}

	played := to - from
	if !c.FirstAt.IsZero() {
		elapsed := beat.At.Sub(c.LastAt)
		if elapsed < 0 {
			c.Rejected++
			return false, "signal antidaté"
		}
		budget := int64(float64(elapsed.Milliseconds())*MaxRate) + clockTolerance.Milliseconds()
		if played > budget {
			// Le cas d'école : un client qui annonce cinq minutes jouées en
			// dix secondes réelles. On refuse tout le signal plutôt que de le
			// tronquer — un client qui ment sur un intervalle ment sur tous.
			c.Rejected++
			return false, fmt.Sprintf("progression impossible : %d ms joués en %d ms", played, elapsed.Milliseconds())
		}
	}

	// Marquage des tranches. Une tranche déjà vue ne recompte pas.
	first := int(from / SegmentMs)
	last := int((to - 1) / SegmentMs)
	total := segmentCount(c.DurationMs)
	for i := first; i <= last && i < total; i++ {
		c.Segments[i/8] |= 1 << (i % 8)
	}

	c.WatchedMs += played
	c.LastPos = to
	c.LastAt = beat.At
	if c.FirstAt.IsZero() {
		c.FirstAt = beat.At
		c.Sessions = 1
	}
	return true, ""
}

// StartSession marque le début d'une nouvelle séance de visionnage.
//
// Le nombre de séances figure au relevé de connexion : un module suivi en une
// fois et un module suivi en huit fois ne racontent pas la même histoire.
func (c *Coverage) StartSession(at time.Time) {
	if c.FirstAt.IsZero() {
		c.FirstAt = at
	} else {
		c.Sessions++
	}
	c.LastAt = at
}

// CoveredSegments compte les tranches vues au moins une fois.
func (c Coverage) CoveredSegments() int {
	count := 0
	total := segmentCount(c.DurationMs)
	for i := range total {
		if c.Segments[i/8]&(1<<(i%8)) != 0 {
			count++
		}
	}
	return count
}

// CoveredMs est la durée unique réellement vue.
func (c Coverage) CoveredMs() int64 {
	covered := int64(c.CoveredSegments()) * SegmentMs
	if covered > c.DurationMs {
		return c.DurationMs
	}
	return covered
}

// Ratio est la part du module réellement vue, entre 0 et 1.
func (c Coverage) Ratio() float64 {
	total := segmentCount(c.DurationMs)
	if total == 0 {
		return 0
	}
	return float64(c.CoveredSegments()) / float64(total)
}

// Percent est le taux de couverture arrondi.
func (c Coverage) Percent() int { return int(c.Ratio()*100 + 0.5) }

// Gaps énumère les intervalles non vus, en millisecondes.
//
// C'est ce que regarde un formateur avant de valider une assiduité limite : un
// apprenant à 88 % qui a sauté la conclusion n'est pas dans le même cas que
// celui qui a sauté douze fragments de dix secondes.
func (c Coverage) Gaps() [][2]int64 {
	var gaps [][2]int64
	total := segmentCount(c.DurationMs)

	start := -1
	for i := range total {
		seen := c.Segments[i/8]&(1<<(i%8)) != 0
		switch {
		case !seen && start < 0:
			start = i
		case seen && start >= 0:
			gaps = append(gaps, [2]int64{int64(start) * SegmentMs, int64(i) * SegmentMs})
			start = -1
		}
	}
	if start >= 0 {
		gaps = append(gaps, [2]int64{int64(start) * SegmentMs, c.DurationMs})
	}
	return gaps
}
