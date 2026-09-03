package bpf

// Les deux typologies du bilan pédagogique et financier.
//
// Le formulaire (Cerfa n° 10443) ne demande pas seulement combien on a
// encaissé et auprès de qui : il demande *qui* on a formé et *à quoi* servait
// la formation. Ces deux ventilations n'existaient nulle part dans le produit,
// si bien que notre bilan s'arrêtait à l'origine des fonds — la moitié de ce
// qu'il faut déposer.
//
// Elles se saisissent là où l'information est connue et vérifiable : le type de
// stagiaire à l'inscription, puisqu'il dépend de la personne et de sa
// situation ce jour-là ; l'objectif sur la formation, puisqu'il ne change pas
// d'un inscrit à l'autre. Les reconstituer en avril, de mémoire, sur un an de
// dossiers, est exactement ce que ce produit existe pour éviter.
//
// Les libellés suivent le formulaire plutôt que le langage courant : ils sont
// recopiés tels quels au dépôt, et un intitulé « maison » obligerait à traduire
// ligne à ligne. Le formulaire évolue — ces listes sont à revoir à chaque
// millésime.

// TypeStagiaire est la nature du stagiaire au sens du cadre E.
type TypeStagiaire string

const (
	StagiaireSalariePrive  TypeStagiaire = "salarie_prive"
	StagiaireApprenti      TypeStagiaire = "apprenti"
	StagiaireSalariePublic TypeStagiaire = "salarie_public"
	StagiaireRechercheEmpl TypeStagiaire = "recherche_emploi"
	StagiaireParticulier   TypeStagiaire = "particulier"
	StagiaireAutre         TypeStagiaire = "autre"
)

// LibellesStagiaire donne l'intitulé du formulaire pour chaque type.
var LibellesStagiaire = map[TypeStagiaire]string{
	StagiaireSalariePrive:  "Salariés d'employeurs privés, hors apprentis",
	StagiaireApprenti:      "Apprentis",
	StagiaireSalariePublic: "Salariés d'employeurs publics",
	StagiaireRechercheEmpl: "Personnes en recherche d'emploi",
	StagiaireParticulier:   "Particuliers à leurs propres frais",
	StagiaireAutre:         "Autres stagiaires",
}

// OrdreStagiaire fixe la présentation, celle du formulaire.
var OrdreStagiaire = []TypeStagiaire{
	StagiaireSalariePrive, StagiaireApprenti, StagiaireSalariePublic,
	StagiaireRechercheEmpl, StagiaireParticulier, StagiaireAutre,
}

// Valid dit si le type est connu du formulaire.
func (t TypeStagiaire) Valid() bool {
	_, ok := LibellesStagiaire[t]
	return ok
}

// Objectif est la finalité de la formation au sens du cadre F.
type Objectif string

const (
	ObjectifDiplome    Objectif = "diplome_rncp"
	ObjectifCQPEnreg   Objectif = "cqp_enregistre"
	ObjectifCQPNon     Objectif = "cqp_non_enregistre"
	ObjectifRS         Objectif = "repertoire_specifique"
	ObjectifBilan      Objectif = "bilan_competences"
	ObjectifVAE        Objectif = "vae"
	ObjectifApprentiss Objectif = "apprentissage"
	ObjectifAutre      Objectif = "autre"
)

// LibellesObjectif donne l'intitulé du formulaire pour chaque objectif.
var LibellesObjectif = map[Objectif]string{
	ObjectifDiplome:    "Formations visant un diplôme ou un titre à finalité professionnelle enregistré au RNCP",
	ObjectifCQPEnreg:   "Formations visant un CQP enregistré au RNCP ou au répertoire spécifique",
	ObjectifCQPNon:     "Formations visant un CQP non enregistré au RNCP ou au répertoire spécifique",
	ObjectifRS:         "Formations visant une certification ou habilitation du répertoire spécifique",
	ObjectifBilan:      "Bilans de compétences",
	ObjectifVAE:        "Actions de validation des acquis de l'expérience",
	ObjectifApprentiss: "Actions de formation par apprentissage",
	ObjectifAutre:      "Autres actions de formation professionnelle continue",
}

// OrdreObjectif fixe la présentation, celle du formulaire.
var OrdreObjectif = []Objectif{
	ObjectifDiplome, ObjectifCQPEnreg, ObjectifCQPNon, ObjectifRS,
	ObjectifBilan, ObjectifVAE, ObjectifApprentiss, ObjectifAutre,
}

// Valid dit si l'objectif est connu du formulaire.
func (o Objectif) Valid() bool {
	_, ok := LibellesObjectif[o]
	return ok
}
