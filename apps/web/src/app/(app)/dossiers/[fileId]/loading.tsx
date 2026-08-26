import { DetailBones, HeaderBones } from "@/components/app/skeleton";

/** Ossature de l'écran pendant que l'API répond. */
export default function Loading() {
  return (
    <>
      <HeaderBones actions={2} />
      <DetailBones />
    </>
  );
}
