import { NextResponse } from "next/server";
import { apiRaw } from "@/lib/api";

/** Duplication d'une formation et de ses modules. */
export async function POST(
  request: Request,
  { params }: { params: Promise<{ courseId: string }> },
) {
  const { courseId } = await params;
  const upstream = await apiRaw(`/v1/courses/${encodeURIComponent(courseId)}/copie`, {
    method: "POST",
    body: await request.text(),
    headers: { "Content-Type": "application/json" },
  });
  const payload = (await upstream.json().catch(() => ({}))) as Record<string, unknown>;
  return NextResponse.json(payload, { status: upstream.status });
}
