import { NextResponse } from "next/server";
import { apiFetch, ApiError } from "@/lib/api";

/** Relais du questionnaire d'un module : lecture puis soumission. */
async function relay(path: string, init?: RequestInit) {
  try {
    return NextResponse.json(await apiFetch<unknown>(path, init));
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 500;
    const message = error instanceof ApiError ? error.message : "erreur interne";
    return NextResponse.json({ error: message }, { status });
  }
}

function target(
  request: Request,
  params: { sessionId: string; courseId: string; quizId: string },
  suffix = "",
) {
  const contactId = new URL(request.url).searchParams.get("contactId");
  return (
    `/v1/learn/${params.sessionId}/courses/${params.courseId}/quizzes/${params.quizId}${suffix}` +
    (contactId ? `?contactId=${encodeURIComponent(contactId)}` : "")
  );
}

export async function GET(
  request: Request,
  { params }: { params: Promise<{ sessionId: string; courseId: string; moduleId: string; quizId: string }> },
) {
  return relay(target(request, await params));
}

export async function POST(
  request: Request,
  { params }: { params: Promise<{ sessionId: string; courseId: string; moduleId: string; quizId: string }> },
) {
  const body = await request.text();
  return relay(target(request, await params, "/submit"), { method: "POST", body });
}
