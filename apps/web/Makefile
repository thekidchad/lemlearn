# Assemblage du paquet Lambda du front.
#
# `next build` produit un serveur autonome dans .next/standalone, mais il lui
# manque deux choses que Next laisse volontairement de côté : les fichiers
# statiques versionnés et le dossier public. Sans elles, la page arrive nue.
#
# Il manque surtout une propriété que Next ne promet pas : la sortie autonome
# reprend l'arborescence de pnpm, où chaque paquet est un lien symbolique vers
# un dossier de la réserve locale. CDK, lui, résout ces liens au moment de
# fabriquer l'archive — chaque paquet devient un vrai dossier, mais à un
# endroit où ses dépendances ne sont plus ses voisines. Next démarre alors
# jusqu'à `require("@swc/helpers")` et s'arrête là.
#
# On reconstruit donc un `node_modules` à plat, comme npm l'aurait posé : tous
# les paquets côte à côte à la racine du paquet. Cette forme survit à la
# résolution des liens, puisqu'elle n'en contient plus aucun.

DIST := dist/lambda
PNPM := .next/standalone/node_modules/.pnpm

# Écartés du paquet : `sharp` sert l'optimiseur d'images, désactivé ici, et son
# binaire est celui de la machine d'assemblage — il ne démarrerait pas sur
# Lambda. `semver@6` ne vient que par cette chaîne-là.
EXCLUDE := sharp semver@6 @img

.PHONY: lambda clean

lambda: clean
	LEMLEARN_BUILD=lambda npx next build
	mkdir -p $(DIST)/apps/web/.next
	cp -R .next/standalone/apps/web/.next/. $(DIST)/apps/web/.next/
	cp .next/standalone/apps/web/package.json $(DIST)/apps/web/package.json 2>/dev/null || true
	cp -R .next/static $(DIST)/apps/web/.next/static
	cp -R public $(DIST)/apps/web/public
	cp lambda-handler.js $(DIST)/apps/web/lambda-handler.js
	@$(MAKE) --no-print-directory flatten
	@echo "paquet : $$(du -sh $(DIST) | cut -f1)"

# Un paquet n'est copié qu'une fois : la première occurrence gagne, et les
# occurrences suivantes sont le même contenu — pnpm ne garde qu'un dossier par
# version, les autres chemins n'y mènent que par des liens.
.PHONY: flatten
flatten:
	@mkdir -p $(DIST)/node_modules
	@for dir in $(PNPM)/*/node_modules; do \
	  case "$$dir" in *"/.pnpm/node_modules"*) continue;; esac; \
	  skip=""; \
	  for bad in $(EXCLUDE); do case "$$dir" in *"$$bad"*) skip=1;; esac; done; \
	  [ -n "$$skip" ] && continue; \
	  for entry in "$$dir"/*; do \
	    name=$$(basename "$$entry"); \
	    skip=""; \
	    for bad in $(EXCLUDE); do case "$$name" in "$$bad"*) skip=1;; esac; done; \
	    [ -n "$$skip" ] && continue; \
	    case "$$name" in \
	      @*) for sub in "$$entry"/*; do \
	            dest=$(DIST)/node_modules/$$name/$$(basename "$$sub"); \
	            [ -e "$$dest" ] && continue; \
	            mkdir -p $(DIST)/node_modules/$$name; \
	            cp -RL "$$sub" "$$dest"; \
	          done;; \
	      *) dest=$(DIST)/node_modules/$$name; \
	         [ -e "$$dest" ] && continue; \
	         cp -RL "$$entry" "$$dest";; \
	    esac; \
	  done; \
	done

clean:
	rm -rf $(DIST)
