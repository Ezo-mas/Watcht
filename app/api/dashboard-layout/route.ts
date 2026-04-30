import { NextResponse } from "next/server";
import { z } from "zod";
import { auth } from "@/auth";
import { DASHBOARD_BLOCK_IDS, getOrCreateDashboardLayout, updateDashboardLayout } from "../../../lib/dashboard-layout";

export const runtime = "nodejs";

const updateLayoutSchema = z
  .object({
    blockOrder: z.array(z.enum(DASHBOARD_BLOCK_IDS)).optional(),
    hiddenBlockIds: z.array(z.enum(DASHBOARD_BLOCK_IDS)).optional(),
  })
  .refine((value) => value.blockOrder !== undefined || value.hiddenBlockIds !== undefined, {
    message: "Provide at least one field to update",
  });

async function requireAppUserId() {
  const session = await auth();
  const appUserId = session?.user?.appUserId;

  if (!appUserId) {
    return null;
  }

  return appUserId;
}

export async function GET() {
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const layout = await getOrCreateDashboardLayout(appUserId);
  return NextResponse.json({ layout });
}

export async function PUT(request: Request) {
  const appUserId = await requireAppUserId();

  if (!appUserId) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const body = await request.json().catch(() => null);
  const parsed = updateLayoutSchema.safeParse(body);

  if (!parsed.success) {
    return NextResponse.json({ error: "Invalid payload", details: parsed.error.flatten() }, { status: 400 });
  }

  const layout = await updateDashboardLayout(appUserId, parsed.data);
  return NextResponse.json({ ok: true, layout });
}
