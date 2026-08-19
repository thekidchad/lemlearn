"use client";

import { useActionState, useState } from "react";
import { anonymizeContact, updateContact, type FormState } from "@/app/actions/crm";
import type { Contact } from "@/lib/api";

/**
 * Fiche d'un contact, modifiable.
 *
 * Les champs d'état civil ne sont pas décoratifs : la date et le lieu de
 * naissance figurent sur l'attestation de fin de formation, et une attestation
 * sans eux se fait refuser. Ils sont donc saisis ici, une fois, plutôt que
 * ressaisis à chaque document.
 */
export function ContactForm({
  contact,
}: {
  contact: Contact;
}) {
  const [state, action, pending] = useActionState<FormState, FormData>(updateContact, {});
  const learner = contact.kind === "learner";

  return (
    <form action={action} className="surface-card p-5">
      <input type="hidden" name="contactId" value={contact.id} />

      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium">Identité</h2>
        {state.ok && <span className="text-2xs text-ok">Enregistré.</span>}
        {state.error && <span className="text-2xs text-danger">{state.error}</span>}
      </div>

      <fieldset disabled={contact.anonymized} className="mt-4 grid gap-3 sm:grid-cols-2">
        {learner ? (
          <>
            <Field label="Prénom" name="firstName" value={contact.firstName} />
            <Field label="Nom" name="lastName" value={contact.lastName} />
            <Field
              label="Date de naissance"
              name="birthDate"
              value={contact.birthDate}
              placeholder="1988-04-12"
              hint="Portée sur l'attestation de fin de formation."
            />
            <Field label="Lieu de naissance" name="birthPlace" value={contact.birthPlace} />
          </>
        ) : (
          <>
            <Field label="Raison sociale" name="companyName" value={contact.companyName} />
            <Field label="SIRET" name="siret" value={contact.siret} />
            <Field label="Interlocuteur — prénom" name="firstName" value={contact.firstName} />
            <Field label="Interlocuteur — nom" name="lastName" value={contact.lastName} />
            <Field label="Fonction" name="position" value={contact.position} />
          </>
        )}

        <Field label="Courriel" name="email" type="email" value={contact.email} />
        <Field label="Téléphone" name="phone" value={contact.phone} />

        <div className="sm:col-span-2">
          <Field label="Adresse" name="line1" value={contact.address?.line1} />
        </div>
        <Field label="Code postal" name="postalCode" value={contact.address?.postalCode} />
        <Field label="Ville" name="city" value={contact.address?.city} />

        <div className="sm:col-span-2">
          <label className="block">
            <span className="mb-1 block text-2xs text-ink-3">Notes internes</span>
            <textarea
              name="notes"
              rows={2}
              defaultValue={contact.notes ?? ""}
              className="w-full rounded-lg border border-line bg-surface-0 px-3 py-2 text-sm outline-none focus:border-accent"
            />
            <span className="mt-1 block text-2xs text-ink-3">
              Visibles de l&apos;organisme seul — mais exportables si la personne
              demande ses données : rien de ce qu&apos;on n&apos;assumerait pas de lui
              montrer.
            </span>
          </label>
        </div>
      </fieldset>

      {!contact.anonymized && (
        <div className="mt-5 flex flex-wrap items-center gap-3">
          <button
            type="submit"
            disabled={pending}
            className="h-9 rounded-lg bg-accent px-4 text-xs font-medium text-white hover:bg-accent-hover disabled:opacity-50"
          >
            {pending ? "Enregistrement…" : "Enregistrer"}
          </button>

          <a
            href={`/api/contacts/${contact.id}/donnees`}
            className="text-2xs text-ink-3 underline hover:text-ink"
          >
            Exporter ses données (portabilité)
          </a>

          {learner && <AnonymizeButton contactId={contact.id} />}
        </div>
      )}
    </form>
  );
}

/** Effacement RGPD, derrière un motif obligatoire. */
function AnonymizeButton({ contactId }: { contactId: string }) {
  const [open, setOpen] = useState(false);
  const [state, action, pending] = useActionState<FormState, FormData>(anonymizeContact, {});

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="ml-auto text-2xs text-ink-3 underline hover:text-danger"
      >
        Anonymiser (droit à l&apos;effacement)
      </button>
    );
  }

  return (
    <div className="w-full rounded-lg border border-danger/40 bg-danger/5 p-3">
      <p className="text-2xs text-ink-2">
        L&apos;anonymisation efface l&apos;état civil, le courriel, le téléphone,
        l&apos;adresse et la pièce d&apos;identité. Les documents scellés
        subsistent sous pseudonyme : ils prouvent une formation réellement
        dispensée, que l&apos;organisme doit pouvoir justifier dix ans.
      </p>
      <input type="hidden" name="contactId" value={contactId} form="anonymize" />
      <div className="mt-2.5 flex flex-wrap items-center gap-2">
        <input
          name="reason"
          form="anonymize"
          required
          placeholder="Motif — il figure au journal d'audit"
          className="h-8 min-w-56 flex-1 rounded-md border border-line bg-surface-0 px-2.5 text-xs outline-none focus:border-danger"
        />
        <button
          type="submit"
          form="anonymize"
          disabled={pending}
          className="h-8 rounded-md bg-danger px-3 text-xs font-medium text-white disabled:opacity-50"
        >
          {pending ? "En cours…" : "Confirmer"}
        </button>
        <button
          type="button"
          onClick={() => setOpen(false)}
          className="h-8 px-2 text-2xs text-ink-3 hover:text-ink"
        >
          Annuler
        </button>
      </div>
      {state.error && <p className="mt-2 text-2xs text-danger">{state.error}</p>}
      {/* Formulaire distinct : imbriquer deux <form> est invalide, et
          l'anonymisation ne doit pas partir avec l'enregistrement. */}
      <form id="anonymize" action={action} />
    </div>
  );
}

function Field({
  label,
  name,
  value,
  type = "text",
  placeholder,
  hint,
}: {
  label: string;
  name: string;
  value?: string;
  type?: string;
  placeholder?: string;
  hint?: string;
}) {
  return (
    <label className="block">
      <span className="mb-1 block text-2xs text-ink-3">{label}</span>
      <input
        name={name}
        type={type}
        defaultValue={value ?? ""}
        placeholder={placeholder}
        className="h-9 w-full rounded-lg border border-line bg-surface-0 px-3 text-sm outline-none focus:border-accent disabled:opacity-60"
      />
      {hint && <span className="mt-1 block text-2xs text-ink-3">{hint}</span>}
    </label>
  );
}
