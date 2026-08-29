import { Bone } from "@/components/app/skeleton";

/**
 * Ossature du module pendant que l'API répond.
 *
 * Elle suit le rythme de l'écran apprenant — large, aéré — et non celui du
 * CRM : une ossature dense sous une page généreuse produit un sursaut au
 * moment où le contenu arrive.
 */
export default function Loading() {
  return (
    <div className="mx-auto max-w-3xl px-5 py-10 sm:px-8 sm:py-12">
      <Bone className="h-4 w-32" />
      <Bone className="mt-8 h-3 w-48" />
      <Bone className="mt-3 h-9 w-3/4" />
      <Bone className="mt-8 aspect-video w-full rounded-xl" />
      <Bone className="mt-4 h-4 w-2/3" />
    </div>
  );
}
