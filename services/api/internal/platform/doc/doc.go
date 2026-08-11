// Package doc compile les documents de lemlearn (devis, convention, feuille
// d'émargement, relevé de connexion, attestation, dossier de preuve) en PDF.
//
// Le moteur est Typst, pas un navigateur sans tête : la compilation prend
// quelques dizaines de millisecondes, le binaire est statique et sans
// dépendance système, et la mise en page est déterministe. Un Chromium
// n'entrerait de toute façon pas dans une Lambda zip.
//
// L'approche est reprise de khwiz (api/internal/service/document), qui a fait
// cette migration en production : compilateur derrière une interface, zones de
// signature déclarées dans le gabarit et extraites par `typst query`, polices
// statiques embarquées dans le binaire.
package doc

import "context"

// Document est une source Typst prête à compiler, accompagnée des fichiers
// qu'elle référence (logo de l'organisme, tracé de signature, tampon…).
// Les clés d'Assets sont des chemins relatifs au fichier source.
type Document struct {
	Source []byte
	Assets map[string][]byte

	// CreationUnix fixe l'horodatage interne du PDF (SOURCE_DATE_EPOCH).
	// Deux compilations de la même source à la même date produisent des
	// octets identiques : indispensable pour que l'empreinte SHA-256 du
	// dossier de preuve soit reproductible et vérifiable après coup.
	// Zéro laisse Typst utiliser l'heure courante.
	CreationUnix int64
}

// Compiler transforme une source Typst en PDF.
type Compiler interface {
	Compile(ctx context.Context, document Document) ([]byte, error)
}

// ZoneCompiler ajoute l'extraction des zones de signature déclarées dans le
// gabarit. BinaryCompiler l'implémente ; les doubles de test peuvent ne pas
// le faire, auquel cas CompileWithZones dégrade vers des zones nil.
type ZoneCompiler interface {
	Compiler
	CompileWithZones(ctx context.Context, document Document) ([]byte, []SignatureZone, error)
}

// CompileWithZones passe par le chemin avec zones quand le compilateur le
// supporte, sinon compile simplement en renvoyant des zones nil.
//
// nil ne signifie pas « aucune zone » mais « zones inconnues » : un appelant
// qui reçoit nil doit refuser de sceller le document plutôt que de placer la
// signature au hasard.
func CompileWithZones(ctx context.Context, c Compiler, d Document) ([]byte, []SignatureZone, error) {
	if zc, ok := c.(ZoneCompiler); ok {
		return zc.CompileWithZones(ctx, d)
	}
	pdf, err := c.Compile(ctx, d)
	return pdf, nil, err
}
