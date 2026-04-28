import { NextResponse } from "next/server";
import { z } from "zod";
import { searchMultiTmdb } from "@/lib/tmdb";

export const runtime = "nodejs";

const searchSchema = z.object({
  q: z.string().trim().min(1).max(200),
  page: z.coerce.number().int().min(1).max(10).optional(),
});

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const parsed = searchSchema.safeParse({
    q: searchParams.get("q"),
    page: searchParams.get("page"),
  });

  if (!parsed.success) {
    return NextResponse.json({ error: "Invalid query", details: parsed.error.flatten() }, { status: 400 });
  }

  const data = await searchMultiTmdb(parsed.data.q, parsed.data.page ?? 1);
  return NextResponse.json(data);
}
