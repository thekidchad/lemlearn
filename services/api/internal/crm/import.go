package crm

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// L'import d'un portefeuille.
//
// C'est ce qui décide d'un changement de logiciel : un organisme arrive avec
// un tableur de trois cents lignes, et sans import il n'arrive pas du tout.
//
// Le fichier n'est jamais accepté à moitié. Une ligne fautive n'interrompt pas
// l'import — refuser trois cents lignes pour une virgule serait absurde — mais
// elle est rapportée avec son numéro et sa raison, et rien de ce qui a échoué
// n'est deviné.

// LigneImport rapporte le sort d'une ligne du fichier.
type LigneImport struct {
	// Numero est celui du fichier, en-tête comprise : c'est celui qu'affiche
	// le tableur de la personne, et lui en donner un autre l'obligerait à
	// compter.
	Numero int    `json:"numero"`
	Nom    string `json:"nom,omitempty"`
	ID     string `json:"id,omitempty"`
	Erreur string `json:"erreur,omitempty"`
}

// ResultatImport résume ce qui est entré et ce qui a été refusé.
type ResultatImport struct {
	Importes int           `json:"importes"`
	Refusees []LigneImport `json:"refusees,omitempty"`
	// Colonnes rend les en-têtes reconnues, pour que l'écran puisse dire ce
	// qu'il a compris du fichier plutôt que de laisser deviner.
	Colonnes []string `json:"colonnes"`
}

// colonnes reconnues, dans les graphies qu'on rencontre vraiment.
//
// Un import qui exige un gabarit exact fait recommencer le tableur ; celui-ci
// accepte ce qu'un logiciel français exporte, accents et casse compris.
var entetes = map[string]string{
	"prenom": "firstName", "prénom": "firstName", "firstname": "firstName",
	"nom": "lastName", "lastname": "lastName", "nom de famille": "lastName",
	"email": "email", "e-mail": "email", "courriel": "email", "mail": "email",
	"telephone": "phone", "téléphone": "phone", "phone": "phone", "tel": "phone",
	"raison sociale": "companyName", "societe": "companyName", "société": "companyName",
	"entreprise": "companyName", "companyname": "companyName",
	"siret":             "siret",
	"date de naissance": "birthDate", "naissance": "birthDate", "birthdate": "birthDate",
	"lieu de naissance": "birthPlace",
	"adresse":           "line1", "rue": "line1",
	"code postal": "postalCode", "cp": "postalCode", "postalcode": "postalCode",
	"ville": "city", "city": "city",
	"source": "marketingSource", "origine": "marketingSource",
	"notes": "notes", "commentaire": "notes",
}

// ImportContacts lit un CSV et crée les fiches.
//
// Le séparateur est deviné sur l'en-tête : un export français est le plus
// souvent en points-virgules, et exiger la virgule ferait échouer un fichier
// parfaitement valable sans dire pourquoi.
func (s *Service) ImportContacts(
	ctx context.Context, orgID string, kind Kind, source io.Reader,
) (ResultatImport, error) {
	if !kind.Valid() {
		return ResultatImport{}, fmt.Errorf("nature de contact %q inconnue", kind)
	}

	brut, err := io.ReadAll(io.LimitReader(source, 4<<20))
	if err != nil {
		return ResultatImport{}, fmt.Errorf("lecture du fichier: %w", err)
	}
	texte := strings.TrimPrefix(string(brut), "\ufeff") // le BOM qu'Excel ajoute
	if strings.TrimSpace(texte) == "" {
		return ResultatImport{}, fmt.Errorf("le fichier est vide")
	}

	lecteur := csv.NewReader(strings.NewReader(texte))
	lecteur.FieldsPerRecord = -1
	premiere := texte
	if saut := strings.IndexAny(texte, "\r\n"); saut > 0 {
		premiere = texte[:saut]
	}
	if strings.Count(premiere, ";") > strings.Count(premiere, ",") {
		lecteur.Comma = ';'
	}

	entete, err := lecteur.Read()
	if err != nil {
		return ResultatImport{}, fmt.Errorf("en-tête illisible: %w", err)
	}

	champs := make([]string, len(entete))
	reconnues := make([]string, 0, len(entete))
	for i, cellule := range entete {
		clef := strings.ToLower(strings.TrimSpace(cellule))
		champs[i] = entetes[clef]
		if champs[i] != "" {
			reconnues = append(reconnues, cellule)
		}
	}

	resultat := ResultatImport{Colonnes: reconnues}
	numero := 1

	for {
		ligne, err := lecteur.Read()
		numero++
		if err == io.EOF {
			break
		}
		if err != nil {
			resultat.Refusees = append(resultat.Refusees, LigneImport{
				Numero: numero, Erreur: "ligne illisible",
			})
			continue
		}

		contact := NewContact(orgID, kind, s.now())
		for i, cellule := range ligne {
			if i >= len(champs) || champs[i] == "" {
				continue
			}
			valeur := strings.TrimSpace(cellule)
			if valeur == "" {
				continue
			}
			switch champs[i] {
			case "firstName":
				contact.FirstName = valeur
			case "lastName":
				contact.LastName = valeur
			case "email":
				contact.Email = valeur
			case "phone":
				contact.Phone = valeur
			case "companyName":
				contact.CompanyName = valeur
			case "siret":
				contact.SIRET = valeur
			case "birthDate":
				contact.BirthDate = valeur
			case "birthPlace":
				contact.BirthPlace = valeur
			case "line1":
				contact.Address.Line1 = valeur
			case "postalCode":
				contact.Address.PostalCode = valeur
			case "city":
				contact.Address.City = valeur
			case "marketingSource":
				contact.MarketingSource = valeur
			case "notes":
				contact.Notes = valeur
			}
		}

		nom := contact.DisplayName()
		if err := contact.Validate(); err != nil {
			resultat.Refusees = append(resultat.Refusees, LigneImport{
				Numero: numero, Nom: nom, Erreur: err.Error(),
			})
			continue
		}

		cree, err := s.CreateContact(ctx, contact)
		if err != nil {
			resultat.Refusees = append(resultat.Refusees, LigneImport{
				Numero: numero, Nom: nom, Erreur: err.Error(),
			})
			continue
		}
		resultat.Importes++
		_ = cree.ID
	}

	return resultat, nil
}
