import { auth, signIn as authSignIn, signOut } from "@/auth";

const localBypassEnabled = process.env.NODE_ENV === "development" && process.env.AUTH_ENABLE_LOCAL_BYPASS !== "false";

const providers = [
  {
    id: "google",
    label: "Google",
    configured: Boolean(process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET),
  },
  {
    id: "github",
    label: "GitHub",
    configured: Boolean(process.env.GITHUB_ID && process.env.GITHUB_SECRET),
  },
  ...(localBypassEnabled
    ? [
        {
          id: "credentials",
          label: "Local Dev Bypass",
          configured: true,
        },
      ]
    : []),
] as const;

export default async function SignInPage() {
  const session = await auth();

  async function handleSignOut() {
    "use server";

    await signOut({ redirectTo: "/auth/sign-in" });
  }

  if (session?.user) {
    const accountLabel = session.user.handle ? `@${session.user.handle}` : session.user.name ?? session.user.email ?? "Account";

    return (
      <section className="mx-auto max-w-xl rounded-2xl border border-white/10 bg-slate-900/70 p-8">
        <p className="text-xs uppercase tracking-[0.18em] text-amber-300">signed in</p>
        <h1 className="mt-2 text-3xl font-bold text-white">{accountLabel}</h1>
        <p className="mt-3 text-sm text-slate-300">
          {session.user.isPrivate === false ? "Profile is public." : "Profile is private by default."}
          {session.user.joinedAt ? ` Joined ${new Date(session.user.joinedAt).toLocaleDateString()}.` : ""}
        </p>

        <div className="mt-6 flex flex-wrap gap-3 text-sm text-slate-200">
          {session.user.handle ? <span className="rounded-full border border-white/15 px-3 py-1">@{session.user.handle}</span> : null}
          <span className="rounded-full border border-white/15 px-3 py-1">{session.user.isPrivate === false ? "public" : "private"}</span>
        </div>

        <form action={handleSignOut} className="mt-6">
          <button type="submit" className="rounded-md border border-white/20 px-4 py-2 text-slate-100 transition hover:border-white/40">
            Sign out
          </button>
        </form>
      </section>
    );
  }

  return (
    <section className="mx-auto max-w-xl rounded-2xl border border-white/10 bg-slate-900/70 p-8">
      <p className="text-xs uppercase tracking-[0.18em] text-amber-300">auth shell</p>
      <h1 className="mt-2 text-3xl font-bold text-white">Sign in to watcht</h1>
      <p className="mt-3 text-sm text-slate-300">
        OAuth is enabled with Google and GitHub. Configure provider secrets in .env.local to activate sign-in.
        {localBypassEnabled ? " The local dev bypass stays available in development." : ""}
      </p>

      <div className="mt-6 space-y-3">
        {providers.map((provider) => (
          <form
            key={provider.id}
            action={async () => {
              "use server";

              if (!provider.configured) {
                return;
              }

              try {
                if (provider.id === "credentials") {
                  // Credentials provider expects credentials; local dev bypass uses an empty credentials object
                  await authSignIn("credentials", { credentials: {}, callbackUrl: "/dashboard" });
                } else {
                  // OAuth flows use callbackUrl
                  await authSignIn(provider.id, { callbackUrl: "/dashboard" });
                }
              } catch (err) {
                // If Next.js is issuing an internal redirect (e.g. NEXT_REDIRECT), rethrow so Next handles it.
                const digest = (err instanceof Error ? err.message : String(err)) ?? "";
                if (typeof digest === "string" && digest.startsWith("NEXT_REDIRECT")) {
                  throw err;
                }

                // Provide clearer server-side logging and a friendlier error message for real errors
                console.error("Sign-in failed for provider", provider.id, err);
                throw new Error(
                  `Sign-in failed for ${provider.label}. Check provider client ID/secret, NEXTAUTH_URL, and redirect URIs.`
                );
              }
            }}
          >
            <button
              type="submit"
              disabled={!provider.configured}
              className="w-full rounded-md border border-white/20 px-4 py-2 text-left text-slate-100 transition hover:border-white/40 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Continue with {provider.label}
              {!provider.configured ? " (configure env first)" : ""}
            </button>
          </form>
        ))}
      </div>
    </section>
  );
}
