"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

/**
 * Palette de recherche de l'équipe lemlearn.
 *
 * Elle remplace un écran de recherche, et ce n'est pas qu'une question de
 * goût : quand on cherche un organisme, on est déjà en train de faire autre
 * chose. Quitter son écran, chercher, puis revenir coûte plus que la recherche
 * elle-même. La palette se pose par-dessus et disparaît.
 *
 * Les résultats sont groupés par catégorie parce qu'une même chaîne peut
 * désigner un organisme, une formation ou un gabarit — mélanger les trois
 * obligerait à lire chaque ligne pour savoir ce qu'on regarde.
 */

interface Results {
  organisations?: { id: string; name: string; plan: string }[];
  apprenants?: { orgId: string; orgName: string; name: string; email: string }[];
  formations?: { id: string; title: string }[];
  gabarits?: { key: string; label: string }[];
  hint?: string;
}

interface Item {
  key: string;
  label: string;
  detail?: string;
  href: string;
}

interface Group {
  title: string;
  items: Item[];
}

/**
 * Ce qui est atteignable sans rien chercher. Une palette qui ne sert qu'à
 * trouver reste fermée la plupart du temps ; celle qui sert aussi à naviguer
 * s'ouvre par réflexe, et c'est ce réflexe qui la rend utile.
 */
const DESTINATIONS: Item[] = [
  { key: "nav-admin", label: "Organisations", href: "/admin" },
  { key: "nav-emails", label: "Journal des envois", href: "/admin/emails" },
  { key: "nav-gabarits", label: "Gabarits de courriels", href: "/admin/gabarits" },
  { key: "nav-biblio", label: "Bibliothèque de formations", href: "/admin/bibliotheque" },
];

export function CommandPalette() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Results>({});
  const [loading, setLoading] = useState(false);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // L'ouverture réinitialise ici, et non dans un effet : remettre l'état à
  // zéro en réaction à un changement d'état déclenche un rendu en cascade,
  // alors que c'est l'ouverture — un geste — qui doit le faire.
  const openPalette = useCallback(() => {
    setQuery("");
    setResults({});
    setActive(0);
    setOpen(true);
  }, []);

  // ⌘K sur Mac, Ctrl+K ailleurs. C'est le raccourci que tout le monde essaie
  // en premier ; en choisir un autre reviendrait à ne pas en avoir.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        if (open) setOpen(false);
        else openPalette();
      }
      if (event.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, openPalette]);

  // Le champ prend le focus à l'ouverture. C'est une action sur le DOM, pas
  // une mise à jour d'état : sa place est bien dans un effet.
  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  // Le délai évite d'interroger l'API à chaque touche : on cherche pendant
  // qu'on tape, pas après chaque caractère.
  useEffect(() => {
    const terme = query.trim();
    if (terme.length < 2) return;
    const timer = setTimeout(async () => {
      setLoading(true);
      try {
        const response = await fetch(`/api/admin/recherche?q=${encodeURIComponent(terme)}`);
        setResults(response.ok ? ((await response.json()) as Results) : {});
      } catch {
        setResults({});
      } finally {
        setLoading(false);
      }
    }, 180);
    return () => clearTimeout(timer);
  }, [query]);

  // En deçà de deux caractères, on n'affiche pas les résultats précédents
  // plutôt que de les effacer : effacer en réaction à une frappe ferait un
  // rendu de plus pour rien.
  const shown = useMemo(
    () => (query.trim().length < 2 ? {} : results),
    [query, results],
  );

  const groups: Group[] = useMemo(() => {
    const terme = query.trim().toLowerCase();
    const out: Group[] = [];

    const destinations = terme
      ? DESTINATIONS.filter((item) => item.label.toLowerCase().includes(terme))
      : DESTINATIONS;
    if (destinations.length > 0) out.push({ title: "Aller à", items: destinations });

    if (shown.organisations?.length) {
      out.push({
        title: "Organisations",
        items: shown.organisations.map((org) => ({
          key: `org-${org.id}`,
          label: org.name,
          detail: org.plan,
          href: `/admin/${org.id}`,
        })),
      });
    }
    if (shown.apprenants?.length) {
      out.push({
        title: "Apprenants",
        items: shown.apprenants.map((learner) => ({
          key: `learner-${learner.orgId}-${learner.email}`,
          label: learner.name || learner.email,
          detail: `${learner.email} · ${learner.orgName}`,
          href: `/admin/${learner.orgId}`,
        })),
      });
    }
    if (shown.formations?.length) {
      out.push({
        title: "Bibliothèque",
        items: shown.formations.map((course) => ({
          key: `course-${course.id}`,
          label: course.title,
          href: `/admin/bibliotheque/${course.id}`,
        })),
      });
    }
    if (shown.gabarits?.length) {
      out.push({
        title: "Gabarits",
        items: shown.gabarits.map((template) => ({
          key: `tpl-${template.key}`,
          label: template.label,
          detail: template.key,
          href: `/admin/gabarits`,
        })),
      });
    }
    return out;
  }, [query, shown]);

  const flat = useMemo(() => groups.flatMap((group) => group.items), [groups]);

  const go = useCallback(
    (item: Item) => {
      setOpen(false);
      router.push(item.href);
    },
    [router],
  );

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActive((index) => (flat.length === 0 ? 0 : (index + 1) % flat.length));
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActive((index) => (flat.length === 0 ? 0 : (index - 1 + flat.length) % flat.length));
    }
    if (event.key === "Enter" && flat[active]) {
      event.preventDefault();
      go(flat[active]);
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={openPalette}
        className="nav-item w-full justify-between text-ink-2"
      >
        <span className="flex items-center gap-2">
          <span aria-hidden className="w-4 text-center text-ink-3">
            ⌕
          </span>
          Rechercher
        </span>
        <kbd className="rounded border border-line px-1 font-mono text-[0.625rem] text-ink-3">
          ⌘K
        </kbd>
      </button>

      {open && (
        <div
          className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 px-4 pt-[12vh] backdrop-blur-[2px]"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setOpen(false);
          }}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Rechercher"
            className="w-full max-w-xl overflow-hidden rounded-xl border border-line bg-surface-1 shadow-2xl"
          >
            <div className="flex items-center gap-3 border-b border-line px-4">
              <span aria-hidden className="text-ink-3">
                ⌕
              </span>
              <input
                ref={inputRef}
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value);
                  setActive(0);
                }}
                onKeyDown={onKeyDown}
                placeholder="Un organisme, une formation, une adresse d'apprenant…"
                className="h-12 flex-1 bg-transparent text-sm outline-none placeholder:text-ink-3"
              />
              {loading && <span className="text-2xs text-ink-3">…</span>}
            </div>

            <div className="max-h-[52vh] overflow-y-auto p-2">
              {groups.length === 0 ? (
                <p className="px-3 py-6 text-center text-xs text-ink-3">
                  {query.trim().length < 2
                    ? "Tapez au moins deux caractères."
                    : "Aucun résultat."}
                </p>
              ) : (
                groups.map((group) => (
                  <div key={group.title} className="mb-2 last:mb-0">
                    <p className="eyebrow px-3 py-1.5">{group.title}</p>
                    {group.items.map((item) => {
                      const index = flat.findIndex((candidate) => candidate.key === item.key);
                      return (
                        <button
                          key={item.key}
                          type="button"
                          onMouseEnter={() => setActive(index)}
                          onClick={() => go(item)}
                          className={`flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition-colors duration-[120ms] ${
                            index === active ? "bg-accent-dim" : "hover:bg-surface-2"
                          }`}
                        >
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm">{item.label}</span>
                            {item.detail && (
                              <span className="block truncate text-2xs text-ink-3">
                                {item.detail}
                              </span>
                            )}
                          </span>
                          {index === active && (
                            <kbd className="shrink-0 font-mono text-[0.625rem] text-ink-3">↵</kbd>
                          )}
                        </button>
                      );
                    })}
                  </div>
                ))
              )}

              {/* Dire pourquoi une catégorie reste vide vaut mieux que de
                  laisser croire à une panne : la recherche d'apprenant est
                  volontairement bornée à l'adresse exacte. */}
              {shown.hint && query.trim().length >= 2 && (
                <p className="border-t border-line px-3 pt-3 pb-1 text-2xs text-ink-3">
                  {shown.hint}
                </p>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
