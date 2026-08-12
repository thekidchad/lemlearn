package pdfsig

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
)

// document rassemble ce qu'il faut connaître du PDF d'origine pour lui ajouter
// une révision.
//
// L'analyse est délibérément minimale : on ne cherche pas à comprendre le
// document, seulement à retrouver son catalogue, sa première page et la
// position de sa table xref. Tout le reste est recopié tel quel.
type document struct {
	raw       []byte
	offsets   map[int]int
	size      int
	root      int
	info      int
	id        []byte
	prevXref  int
	firstPage int
}

var (
	startxrefPattern = regexp.MustCompile(`startxref\s+(\d+)\s*%%EOF\s*$`)
	trailerPattern   = regexp.MustCompile(`(?s)trailer\s*<<(.*?)>>\s*startxref`)
	refPattern       = regexp.MustCompile(`(\d+)\s+\d+\s+R`)
	idPattern        = regexp.MustCompile(`(?s)/ID\s*(\[.*?\])`)
	kidsPattern      = regexp.MustCompile(`(?s)/Kids\s*\[\s*(\d+)\s+\d+\s+R`)
)

func parse(pdf []byte) (*document, error) {
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, fmt.Errorf("pdfsig: ce n'est pas un PDF")
	}

	// La recherche part de la fin : un PDF peut déjà porter des révisions, et
	// seule la dernière décrit l'état courant.
	tail := pdf
	if len(tail) > 2048 {
		tail = tail[len(tail)-2048:]
	}
	startxref := startxrefPattern.FindSubmatch(tail)
	if startxref == nil {
		return nil, fmt.Errorf("pdfsig: startxref introuvable")
	}
	prevXref, _ := strconv.Atoi(string(startxref[1]))

	trailer := trailerPattern.FindSubmatch(pdf[prevXref:])
	if trailer == nil {
		return nil, fmt.Errorf(
			"pdfsig: trailer classique introuvable — ce PDF utilise probablement un flux xref, non géré")
	}
	fields := trailer[1]

	doc := &document{raw: pdf, prevXref: prevXref}
	doc.size = intField(fields, "/Size")
	doc.root = refField(fields, "/Root")
	doc.info = refField(fields, "/Info")
	if id := idPattern.FindSubmatch(fields); id != nil {
		doc.id = id[1]
	}
	if doc.size == 0 || doc.root == 0 {
		return nil, fmt.Errorf("pdfsig: trailer incomplet (Size=%d Root=%d)", doc.size, doc.root)
	}

	offsets, err := parseXref(pdf, prevXref)
	if err != nil {
		return nil, err
	}
	doc.offsets = offsets

	firstPage, err := doc.findFirstPage()
	if err != nil {
		return nil, err
	}
	doc.firstPage = firstPage

	return doc, nil
}

// parseXref lit la table de références croisées à partir de son offset.
//
// Les offsets exacts viennent de la table plutôt que d'une recherche textuelle
// de « N 0 obj » : cette chaîne peut apparaître à l'intérieur d'un flux
// compressé, et un objet mal localisé produirait un PDF corrompu.
func parseXref(pdf []byte, offset int) (map[int]int, error) {
	if offset <= 0 || offset >= len(pdf) {
		return nil, fmt.Errorf("pdfsig: offset de xref hors limites (%d)", offset)
	}
	section := pdf[offset:]
	if !bytes.HasPrefix(bytes.TrimLeft(section, " \r\n"), []byte("xref")) {
		return nil, fmt.Errorf("pdfsig: table xref classique attendue — flux xref non géré")
	}

	section = bytes.TrimLeft(section, " \r\n")[len("xref"):]
	offsets := map[int]int{}

	for {
		section = bytes.TrimLeft(section, " \r\n")
		if bytes.HasPrefix(section, []byte("trailer")) || len(section) == 0 {
			break
		}

		var first, count int
		read, err := fmt.Sscanf(string(section[:min(40, len(section))]), "%d %d", &first, &count)
		if err != nil || read != 2 {
			break
		}
		newline := bytes.IndexByte(section, '\n')
		if newline < 0 {
			break
		}
		section = section[newline+1:]

		for i := range count {
			if len(section) < 20 {
				return nil, fmt.Errorf("pdfsig: table xref tronquée")
			}
			entry := section[:20]
			if entry[17] == 'n' {
				offsets[first+i] = atoi(entry[:10])
			}
			section = section[20:]
		}
	}

	if len(offsets) == 0 {
		return nil, fmt.Errorf("pdfsig: table xref vide")
	}
	return offsets, nil
}

// object renvoie les octets complets d'un objet, « N 0 obj » à « endobj ».
func (d *document) object(num int) ([]byte, error) {
	offset, ok := d.offsets[num]
	if !ok || offset >= len(d.raw) {
		return nil, fmt.Errorf("pdfsig: objet %d absent de la table xref", num)
	}
	end := bytes.Index(d.raw[offset:], []byte("endobj"))
	if end < 0 {
		return nil, fmt.Errorf("pdfsig: objet %d non terminé", num)
	}
	return d.raw[offset : offset+end+len("endobj")], nil
}

// findFirstPage descend du catalogue vers la première page.
func (d *document) findFirstPage() (int, error) {
	catalog, err := d.object(d.root)
	if err != nil {
		return 0, err
	}
	pagesRef := refField(catalog, "/Pages")
	if pagesRef == 0 {
		return 0, fmt.Errorf("pdfsig: /Pages absent du catalogue")
	}

	pages, err := d.object(pagesRef)
	if err != nil {
		return 0, err
	}
	kids := kidsPattern.FindSubmatch(pages)
	if kids == nil {
		return 0, fmt.Errorf("pdfsig: /Kids absent de l'arbre des pages")
	}
	first, _ := strconv.Atoi(string(kids[1]))
	if first == 0 {
		return 0, fmt.Errorf("pdfsig: première page introuvable")
	}
	return first, nil
}

// objectWithAnnot renvoie la page réécrite avec le widget de signature ajouté
// à ses annotations.
func (d *document) objectWithAnnot(pageNum, fieldNum int) ([]byte, error) {
	page, err := d.object(pageNum)
	if err != nil {
		return nil, err
	}
	ref := fmt.Sprintf("%d 0 R", fieldNum)

	// Une page peut déjà porter des annotations — Typst en produit pour les
	// liens. Les écraser ferait disparaître ces liens du document signé.
	if idx := bytes.Index(page, []byte("/Annots")); idx >= 0 {
		open := bytes.IndexByte(page[idx:], '[')
		if open < 0 {
			return nil, fmt.Errorf("pdfsig: /Annots de la page %d non géré (référence indirecte)", pageNum)
		}
		insertAt := idx + open + 1
		updated := make([]byte, 0, len(page)+len(ref)+2)
		updated = append(updated, page[:insertAt]...)
		updated = append(updated, ' ')
		updated = append(updated, ref...)
		updated = append(updated, page[insertAt:]...)
		return append(updated, '\n'), nil
	}
	return insertBeforeDictEnd(page, fmt.Sprintf("/Annots [%s]", ref))
}

// objectWithAcroForm renvoie le catalogue réécrit avec le formulaire de
// signature.
func (d *document) objectWithAcroForm(rootNum, fieldNum int) ([]byte, error) {
	catalog, err := d.object(rootNum)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(catalog, []byte("/AcroForm")) {
		return nil, fmt.Errorf("pdfsig: ce document porte déjà un formulaire, fusion non gérée")
	}
	// SigFlags 3 = SignaturesExist | AppendOnly. AppendOnly indique au lecteur
	// que le document ne doit plus être modifié autrement que par ajout — ce
	// qui est exactement la promesse faite au signataire.
	return insertBeforeDictEnd(catalog,
		fmt.Sprintf("/AcroForm << /Fields [%d 0 R] /SigFlags 3 >>", fieldNum))
}

// insertBeforeDictEnd insère une entrée juste avant la fermeture du
// dictionnaire de l'objet.
func insertBeforeDictEnd(object []byte, entry string) ([]byte, error) {
	end := bytes.LastIndex(object, []byte(">>"))
	if end < 0 {
		return nil, fmt.Errorf("pdfsig: dictionnaire non terminé")
	}
	updated := make([]byte, 0, len(object)+len(entry)+2)
	updated = append(updated, object[:end]...)
	updated = append(updated, '\n')
	updated = append(updated, entry...)
	updated = append(updated, '\n')
	updated = append(updated, object[end:]...)
	return append(updated, '\n'), nil
}

func intField(fields []byte, name string) int {
	idx := bytes.Index(fields, []byte(name))
	if idx < 0 {
		return 0
	}
	rest := bytes.TrimLeft(fields[idx+len(name):], " \r\n")
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	return atoi(rest[:end])
}

func refField(fields []byte, name string) int {
	idx := bytes.Index(fields, []byte(name))
	if idx < 0 {
		return 0
	}
	match := refPattern.FindSubmatch(fields[idx+len(name):])
	if match == nil {
		return 0
	}
	num, _ := strconv.Atoi(string(match[1]))
	return num
}
