import Image from "next/image";
import Link from "next/link";
import { auth, signOut } from "@/auth";

const navItems = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/dashboard#shows", label: "Shows" },
  { href: "/dashboard#movies", label: "Movies" },
  { href: "/dashboard#calendar", label: "Calendar" },
  { href: "/dashboard#discover", label: "Discover" },
  { href: "/dashboard#apps", label: "Apps" },
  { href: "/dashboard#forums", label: "Forums" },
  { href: "/profile", label: "Profile" },
];

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
      <div className="mx-auto flex max-w-[96rem] items-center justify-between gap-4 px-5 py-4">
        <Link href="/" className="text-lg font-semibold tracking-wide text-amber-400">
          watcht
        </Link>
        <nav className="hidden items-center gap-5 text-sm text-slate-300 lg:flex">
          {navItems.map((item) => (
            <Link key={item.href} href={item.href} className="transition hover:text-white">
              {item.label}
            </Link>
          ))}
        </nav>
        <nav className="flex items-center gap-4 text-sm text-slate-300">
          {isAuthenticated ? (
            <>
              <Link href="/profile" className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1 text-slate-200 transition hover:border-white/25">
                {session?.user?.image ? (
                  <Image src={session.user.image} alt={userLabel} width={24} height={24} unoptimized className="h-6 w-6 rounded-full object-cover" />
                ) : (
                  <span className="flex h-6 w-6 items-center justify-center rounded-full bg-white/10 text-[11px] font-semibold uppercase text-white">
                    {userLabel.charAt(0)}
                  </span>
                )}
                <span className="max-w-40 truncate" title={userLabel}>
                  {userLabel}
                </span>
                {session?.user?.isPrivate !== undefined ? (
                  <span className="rounded-full border border-amber-400/30 px-2 py-0.5 text-[10px] uppercase tracking-[0.18em] text-amber-200">
                    {privacyLabel}
                  </span>
                ) : null}
              </Link>
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
