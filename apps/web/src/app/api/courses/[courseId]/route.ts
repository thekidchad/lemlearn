import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Correction du programme d'une formation. */
export async function PATCH(
  request: Request,
  { params }: { params: Promise<{ courseId: string }> },
) {
  const { courseId } = await params;
  const upstream = await apiRaw(`/v1/courses/${encodeURIComponent(courseId)}`, {
    method: "PATCH",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
