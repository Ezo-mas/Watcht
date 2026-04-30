import Link from "next/link";
import { auth, signOut } from "@/auth";

const navItems = [{ href: "/dashboard", label: "Dashboard" }];

export default async function SiteHeader() {
  const session = await auth();
  const isAuthenticated = Boolean(session?.user);
  const userLabel = session?.user?.handle ? `@${session.user.handle}` : session?.user?.name ?? session?.user?.email ?? "Account";
  const privacyLabel = session?.user?.isPrivate === false ? "public" : "private";

  async function handleSignOut() {
    "use server";

    await signOut({ redirectTo: "/" });
  }

  return (
    <header className="sticky top-0 z-50 border-b border-white/10 bg-slate-950/80 backdrop-blur">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-5 py-4">
        <Link href="/" className="text-lg font-semibold tracking-wide text-amber-400">
          watcht
        </Link>
        <nav className="flex items-center gap-4 text-sm text-slate-300">
          {navItems.map((item) => (
            <Link key={item.href} href={item.href} className="transition hover:text-white">
              {item.label}
            </Link>
          ))}
          {isAuthenticated ? (
            <>
              <span className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1 text-slate-200">
                <span className="max-w-40 truncate" title={userLabel}>
                  {userLabel}
                </span>
                {session?.user?.isPrivate !== undefined ? (
                  <span className="rounded-full border border-amber-400/30 px-2 py-0.5 text-[10px] uppercase tracking-[0.18em] text-amber-200">
                    {privacyLabel}
                  </span>
                ) : null}
              </span>
              <form action={handleSignOut}>
                <button type="submit" className="transition hover:text-white">
                  Sign out
                </button>
              </form>
            </>
          ) : (
            <Link href="/auth/sign-in" className="transition hover:text-white">
              Sign in
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}
