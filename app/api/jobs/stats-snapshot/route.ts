import { NextResponse } from "next/server";
import { z } from "zod";
import { runStatsSnapshotJob } from "@/lib/stats-snapshots";

export const runtime = "nodejs";

const querySchema = z.object({
  userId: z.string().uuid().optional(),
});

function isAuthorized(request: Request) {
  const jobSecret = process.env.JOB_SECRET;

  if (!jobSecret) {
    return process.env.NODE_ENV !== "production";
  }

  const token = request.headers.get("x-job-token");
  return token === jobSecret;
}

export async function POST(request: Request) {
  if (!isAuthorized(request)) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { searchParams } = new URL(request.url);
  const parsed = querySchema.safeParse({
    userId: searchParams.get("userId") ?? undefined,
  });

  if (!parsed.success) {
    return NextResponse.json({ error: "Invalid request", details: parsed.error.flatten() }, { status: 400 });
  }

  const results = await runStatsSnapshotJob({ userId: parsed.data.userId });
  return NextResponse.json({ ok: true, count: results.length, results });
}
